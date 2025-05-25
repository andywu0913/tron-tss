package secret_manager_party

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
	"tron-tss/config"
	"tron-tss/internal/utils"
	"tron-tss/types"

	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/gorilla/websocket"
)

func NewKeyGenHandler(conn *websocket.Conn) *KeyGenHandler {
	return &KeyGenHandler{conn: conn}
}

type KeyGenHandler struct {
	conn  *websocket.Conn
	party tss.Party
	errCh chan *tss.Error
	outCh chan tss.Message
	endCh chan *keygen.LocalPartySaveData
}

func (h *KeyGenHandler) handleIncomingStartMsg(partyID string, requestUUID string, data types.KeyGenStartMsg) {
	h.errCh = make(chan *tss.Error, len(data.PartyIDs))
	h.outCh = make(chan tss.Message, len(data.PartyIDs))
	h.endCh = make(chan *keygen.LocalPartySaveData, len(data.PartyIDs))
	h.party = h.startLocalParty(partyID, data.Threshold, data.PartyIDs)

	done := make(chan struct{})
	go func() {
		for {
			select {
			case msg := <-h.outCh:
				log.Printf("Sending key gen message from party %v to %v, broadcast=%v", msg.GetFrom().Id, msg.GetTo(), msg.IsBroadcast())

				h.sendCommunicateMsg(requestUUID, msg)

			case saveData := <-h.endCh:
				log.Printf("Key generation completed.")

				tronAddress, err := utils.GenerateTronAddress(saveData.ECDSAPub.ToECDSAPubKey())
				if err != nil {
					h.errCh <- h.party.WrapError(fmt.Errorf("Failed to generate tron address: %w", err))
					continue
				}
				log.Printf("Generated tron address: %v", tronAddress)

				err = utils.StoreLocalPartyData(partyID, config.SecretManagerPartyConfigMap[partyID].StoragePath, fmt.Sprintf("%v.json", tronAddress), saveData)
				if err != nil {
					h.errCh <- h.party.WrapError(fmt.Errorf("Failed to store local party data: %w", err))
					continue
				}
				log.Printf("Store private key success: %v", tronAddress)

				h.sendDoneMsg(requestUUID, h.party.PartyID(), tronAddress)

				close(done)
				return

			case err := <-h.errCh:
				log.Printf("Key gen error received from channel: %v", err)

				h.sendErrorMsg(requestUUID, h.party.PartyID(), err)

				close(done)
				return

			case <-time.After(config.Timeout):
				log.Printf("Timeout waiting for key generation. Exiting...")

				close(done)
				return
			}
		}
	}()

	log.Printf("Started key generation process.")

	// clean up
	go func() {
		<-done
		close(h.errCh)
		close(h.outCh)
		close(h.endCh)

		log.Printf("Cleaned up resources for request: %v", requestUUID)
	}()
}

func (h *KeyGenHandler) handleIncomingCommunicateMsg(data types.KeyGenCommunicateMsg) {
	log.Printf("Received message from party %v", data.From)

	msg, err := base64.StdEncoding.DecodeString(*data.Msg)
	if err != nil {
		log.Panicf("Failed to decode message data: %v", err)
		return
	}

	pMsg, err := tss.ParseWireMessage(msg, data.From, *data.IsBroadcast)
	if err != nil {
		log.Panicf("Failed to parse wire message: %v", err)
		h.errCh <- h.party.WrapError(err)
		return
	}

	if _, err := h.party.Update(pMsg); err != nil {
		log.Panicf("Failed to update party state: %v", err)
		h.errCh <- err
		return
	}
}

func (h *KeyGenHandler) sendCommunicateMsg(requestUUID string, msg tss.Message) {
	msgBytes, _, err := msg.WireBytes()
	if err != nil {
		log.Panicf("Failed to get wire message bytes: %v", err)
	}

	msgEncoded := base64.StdEncoding.EncodeToString(msgBytes)
	from := msg.GetFrom()
	to := msg.GetTo()
	isBroadcast := msg.IsBroadcast()

	data, _ := json.Marshal(types.KeyGenCommunicateMsg{
		From:        from,
		To:          to,
		Msg:         &msgEncoded,
		IsBroadcast: &isBroadcast,
	})

	err = h.conn.WriteJSON(types.Msg{
		RequestUUID: requestUUID,
		Type:        types.MsgTypeKeyGenCommunicate,
		Data:        data,
	})

	if err != nil {
		log.Printf("Failed to send communicate message: %v", err)
	}
}

func (h *KeyGenHandler) sendDoneMsg(requestUUID string, from *tss.PartyID, tronAddress string) {
	data, _ := json.Marshal(types.KeyGenDoneMsg{
		From:    from,
		Address: tronAddress,
	})

	err := h.conn.WriteJSON(types.Msg{
		RequestUUID: requestUUID,
		Type:        types.MsgTypeKeyGenDone,
		Data:        data,
	})

	if err != nil {
		log.Panicf("Failed to send done message: %v", err)
	}
}

func (h *KeyGenHandler) sendErrorMsg(requestUUID string, from *tss.PartyID, err error) {
	e := err.Error()

	data, _ := json.Marshal(types.KeyGenErrorMsg{
		From: from,
		Err:  &e,
	})

	err = h.conn.WriteJSON(types.Msg{
		RequestUUID: requestUUID,
		Type:        types.MsgTypeKeyGenError,
		Data:        data,
	})
	if err != nil {
		log.Printf("Failed to send error message: %v", err)
	}
}

func (h *KeyGenHandler) startLocalParty(id string, threshold int, partyIDs tss.SortedPartyIDs) tss.Party {
	peerCtx := tss.NewPeerContext(partyIDs)

	for _, partyID := range partyIDs {
		if partyID.Id != id {
			continue
		}

		params := tss.NewParameters(tss.S256(), peerCtx, partyID, len(partyIDs), threshold)

		var preParams []keygen.LocalPreParams

		fixtureFilePath := config.SecretManagerPartyConfigMap[id].StoragePath + string(os.PathSeparator) + "ecdsa_fixtures.json"
		if bz, err := os.ReadFile(fixtureFilePath); err == nil {
			var key keygen.LocalPartySaveData
			if err = json.Unmarshal(bz, &key); err != nil {
				goto END_LOADING_FIXTURE
			}
			for _, kbxj := range key.BigXj {
				kbxj.SetCurve(tss.S256())
			}
			key.ECDSAPub.SetCurve(tss.S256())

			preParams = append(preParams, key.LocalPreParams)

			log.Printf("Load fixture success.")
		} else {
			log.Printf("No exist fixture was found, so the safe primes will be generated from scratch. This may take a while.")

		}
	END_LOADING_FIXTURE:

		var p tss.Party
		p = keygen.NewLocalParty(params, h.outCh, h.endCh, preParams...)

		log.Printf("Starting keygen party %s", partyID.Id)
		go func(p tss.Party) {
			if err := p.Start(); err != nil {
				h.errCh <- err
				log.Printf("Error starting keygen party %s: %v", p.PartyID().Id, err)

			}
		}(p)
		return p
	}

	return nil
}

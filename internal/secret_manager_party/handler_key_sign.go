package secret_manager_party

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"time"
	"tron-tss/config"
	"tron-tss/internal/utils"
	"tron-tss/types"

	"github.com/bnb-chain/tss-lib/v2/common"
	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/ecdsa/signing"
	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/gorilla/websocket"
)

func NewSignHandler(conn *websocket.Conn) *SignHandler {
	return &SignHandler{conn: conn}
}

type SignHandler struct {
	conn  *websocket.Conn
	party tss.Party
	errCh chan *tss.Error
	outCh chan tss.Message
	endCh chan *common.SignatureData
}

func (h *SignHandler) handleIncomingStartMsg(partyID string, requestUUID string, data types.SignStartMsg) {
	msgHashInt := new(big.Int).SetBytes(data.MessageHash)

	saveData, err := utils.LoadLocalPartyData(partyID, config.SecretManagerPartyConfigMap[partyID].StoragePath, fmt.Sprintf("%v.json", data.FromAddress))
	if err != nil {
		log.Printf("Failed to load local party data for partyID %v, address %v: %v", partyID, data.FromAddress, err)
		h.sendErrorMsg(requestUUID, nil, fmt.Errorf("Failed to load local party data for partyID %v, address %v: %w", partyID, data.FromAddress, err))
		return
	}

	h.errCh = make(chan *tss.Error, len(data.PartyIDs))
	h.outCh = make(chan tss.Message, len(data.PartyIDs))
	h.endCh = make(chan *common.SignatureData, len(data.PartyIDs))
	h.party = h.startLocalParty(partyID, data.Threshold, data.PartyIDs, saveData, msgHashInt)

	done := make(chan struct{})
	go func() {
		for {
			select {
			case msg := <-h.outCh:
				log.Printf("Sending signing message from party %v to %v, broadcast=%v", msg.GetFrom().Id, msg.GetTo(), msg.IsBroadcast())

				h.sendCommunicateMsg(requestUUID, msg)

			case sigData := <-h.endCh:
				log.Printf("Signing completed.")

				h.sendDoneMsg(requestUUID, h.party.PartyID(), saveData, data.MessageHash, sigData)

				close(done)
				return

			case err := <-h.errCh:
				log.Printf("Signing error received from channel: %v", err)

				h.sendErrorMsg(requestUUID, h.party.PartyID(), err)

				close(done)
				return

			case <-time.After(config.Timeout):
				log.Printf("Timeout waiting for signing. Exiting...")

				close(done)
				return
			}
		}
	}()

	log.Printf("Started signing process.")

	// clean up
	go func() {
		<-done
		close(h.errCh)
		close(h.outCh)
		close(h.endCh)

		log.Printf("Cleaned up resources for request: %v", requestUUID)
	}()
}

func (h *SignHandler) handleIncomingCommunicateMsg(data types.SignCommunicateMsg) {
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

func (h *SignHandler) sendCommunicateMsg(requestUUID string, msg tss.Message) {
	msgBytes, _, err := msg.WireBytes()
	if err != nil {
		log.Panicf("Failed to get wire message bytes: %v", err)
	}

	msgEncoded := base64.StdEncoding.EncodeToString(msgBytes)
	from := msg.GetFrom()
	to := msg.GetTo()
	isBroadcast := msg.IsBroadcast()

	data, _ := json.Marshal(types.SignCommunicateMsg{
		From:        from,
		To:          to,
		Msg:         &msgEncoded,
		IsBroadcast: &isBroadcast,
	})

	err = h.conn.WriteJSON(types.Msg{
		RequestUUID: requestUUID,
		Type:        types.MsgTypeSignCommunicate,
		Data:        data,
	})

	if err != nil {
		log.Printf("Failed to send communicate message: %v", err)
	}
}

func (h *SignHandler) sendDoneMsg(requestUUID string, from *tss.PartyID, saveData *keygen.LocalPartySaveData, msgHash []byte, sigData *common.SignatureData) {
	canonicalSig := utils.FormatCanonicalSignature(
		new(big.Int).SetBytes(sigData.R),
		new(big.Int).SetBytes(sigData.S),
	)
	log.Printf("Canonical signature (r||s): %x", canonicalSig)

	var x btcec.FieldVal
	var y btcec.FieldVal

	x.SetByteSlice(saveData.ECDSAPub.X().Bytes())
	y.SetByteSlice(saveData.ECDSAPub.Y().Bytes())

	signature, err := utils.FindValidRecoveryID(msgHash, canonicalSig, btcec.NewPublicKey(&x, &y))
	if err != nil {
		log.Panicf("Failed to find valid recovery ID: %v", err)
	}

	sig := hex.EncodeToString(signature)

	log.Printf("Final signature: %v", sig)

	data, _ := json.Marshal(types.SignDoneMsg{
		From:      from,
		Signature: sig,
	})

	err = h.conn.WriteJSON(types.Msg{
		RequestUUID: requestUUID,
		Type:        types.MsgTypeSignDone,
		Data:        data,
	})

	if err != nil {
		log.Panicf("Failed to send done message: %v", err)
	}
}

func (h *SignHandler) sendErrorMsg(requestUUID string, from *tss.PartyID, err error) {
	e := err.Error()

	data, _ := json.Marshal(types.SignErrorMsg{
		From: from,
		Err:  &e,
	})

	err = h.conn.WriteJSON(types.Msg{
		RequestUUID: requestUUID,
		Type:        types.MsgTypeSignError,
		Data:        data,
	})
	if err != nil {
		log.Printf("Failed to send error message: %v", err)
	}
}

func (h *SignHandler) startLocalParty(id string, threshold int, partyIDs tss.SortedPartyIDs, key *keygen.LocalPartySaveData, msgHashInt *big.Int) tss.Party {
	peerCtx := tss.NewPeerContext(partyIDs)

	for _, partyID := range partyIDs {
		if partyID.Id != id {
			continue
		}

		params := tss.NewParameters(tss.S256(), peerCtx, partyID, len(partyIDs), threshold)
		p := signing.NewLocalParty(msgHashInt, params, *key, h.outCh, h.endCh)

		log.Printf("Starting signing party %s", partyID.Id)
		go func(p tss.Party) {
			if err := p.Start(); err != nil {
				h.errCh <- err
				log.Printf("Error starting signing party %s: %v", p.PartyID().Id, err)

			}
		}(p)

		return p
	}

	return nil
}

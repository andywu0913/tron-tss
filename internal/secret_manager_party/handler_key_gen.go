package secret_manager_party

import (
	"encoding/base64"
	"fmt"
	"log"
	"time"
	"tron-tss/config"
	internalTSS "tron-tss/internal/tss"
	"tron-tss/internal/utils"
	"tron-tss/types"

	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/gorilla/websocket"
)

type KeyGenHandler struct {
	party tss.Party
	errCh chan *tss.Error
	outCh chan tss.Message
	endCh chan *keygen.LocalPartySaveData
}

func (h *KeyGenHandler) handleIncomingStartMsg(partyID int, requestUUID string, conn *websocket.Conn, threshold int, partyIDs tss.SortedPartyIDs) {
	h.errCh = make(chan *tss.Error, len(partyIDs))
	h.outCh = make(chan tss.Message, len(partyIDs))
	h.endCh = make(chan *keygen.LocalPartySaveData, len(partyIDs))
	h.party = internalTSS.GenStart(partyID, threshold, partyIDs, h.errCh, h.outCh, h.endCh)

	done := make(chan struct{})
	go func() {
		for {
			select {
			case msg := <-h.outCh:
				log.Printf("Sending key gen message from party %v to %v, broadcast=%v", msg.GetFrom().Id, msg.GetTo(), msg.IsBroadcast())

				sendCommunicateMsg(requestUUID, conn, msg)

			case saveData := <-h.endCh:
				log.Printf("Key generation completed.")

				tronAddress, err := utils.GenerateTronAddress(saveData.ECDSAPub.ToECDSAPubKey())
				if err != nil {
					h.errCh <- h.party.WrapError(fmt.Errorf("Failed to generate tron address: %w", err))
					continue
				}

				err = utils.StoreLocalPartyData(partyID, config.SecretManagerPartyConfigMap[partyID].StoragePath, fmt.Sprintf("%v.txt", tronAddress), saveData)
				if err != nil {
					h.errCh <- h.party.WrapError(fmt.Errorf("Failed to store local party data: %w", err))
					continue
				}

				sendDoneMsg(requestUUID, conn, h.party.PartyID(), saveData.ECDSAPub)

				close(done)
				return

			case err := <-h.errCh:
				log.Printf("Key gen error received from channel: %v", err)

				sendErrorMsg(requestUUID, conn, h.party.PartyID(), err)

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

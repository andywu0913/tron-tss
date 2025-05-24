package secret_manager

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
	"tron-tss/config"
	internalTSS "tron-tss/internal/tss"
	"tron-tss/types"

	"github.com/bnb-chain/tss-lib/v2/crypto"
	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/gorilla/websocket"
)

var (
	requestChMap = sync.Map{}
)

func HandleConnection(partyID int) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var upgrader = websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Error upgrading connection:", err)
			return
		}
		defer conn.Close()

		log.Println("Client connected.")

		for {
			var msgStruct types.Msg

			err := conn.ReadJSON(&msgStruct)
			if err != nil {
				log.Println("Error reading message:", err)
				log.Printf("Error type: %T", err)
				continue
			}

			log.Printf("Received message: request_uuid: %v, type: %v", msgStruct.RequestUUID, msgStruct.Type)

			switch msgStruct.Type {
			case types.MsgTypeKeyGenStart:
				var data types.MsgKeyGenStart

				err := json.Unmarshal(msgStruct.Data, &data)
				if err != nil {
					log.Println("Error unmarshal message:", err)
					continue
				}

				handleKeyGenStart(partyID, msgStruct.RequestUUID, conn, data.Threshold, data.PartyIDs)

			case types.MsgTypeKeyGenCommunicate:
				var data types.MsgKeyGenCommunicate

				err := json.Unmarshal(msgStruct.Data, &data)
				if err != nil {
					log.Println("Error unmarshal message:", err)
					continue
				}

				handleKeyGenCommunicate(msgStruct.RequestUUID, data)

			case types.MsgTypeKeyGenError:
				// TODO: abort key gen process
			}
		}
	}
}

func handleKeyGenStart(partyID int, requestUUID string, conn *websocket.Conn, threshold int, partyIDs tss.SortedPartyIDs) {
	errCh := make(chan *tss.Error, len(partyIDs))
	outCh := make(chan tss.Message, len(partyIDs))
	endCh := make(chan *keygen.LocalPartySaveData, len(partyIDs))
	done := make(chan struct{}, 1)

	party := internalTSS.GenStart(partyID, threshold, partyIDs, errCh, outCh, endCh)
	storeRequestState(requestUUID, party, errCh, outCh, endCh)

	go func() {
		for {
			select {
			case msg := <-outCh:
				log.Printf("Send message from %v to %v, isBroadcast: %v", msg.GetFrom().Id, msg.GetTo(), msg.IsBroadcast())

				sendCommunicateMsg(requestUUID, conn, msg)

			case saveData := <-endCh:
				log.Printf("Kengen done.")

				sendDoneMsg(requestUUID, conn, party.PartyID(), saveData.ECDSAPub)

				done <- struct{}{}
				return

			case err := <-errCh:
				log.Printf("Get error from channel: %s", err)

				sendErrorMsg(requestUUID, conn, party.PartyID(), err)

				done <- struct{}{}
				return

			case <-time.After(config.Timeout):
				log.Println("Timeout waiting for keygen! Exiting...")

				done <- struct{}{}
				return
			}
		}
	}()

	// clean up
	go func() {
		<-done
		close(done)
		close(errCh)
		close(outCh)
		close(endCh)
		destroyRequestState(requestUUID)

		log.Printf("Cleaned up resource for request: %v", requestUUID)
	}()

	log.Println("Start keygen process...")
}
func handleKeyGenCommunicate(requestUUID string, data types.MsgKeyGenCommunicate) {
	log.Printf("Get message from party %v", data.From)

	party, errCh, _, _, ok := loadRequestState(requestUUID)
	if !ok {
		log.Panicf("Cannot load request state with request_uuid %v", requestUUID)
		return
	}

	msg, err := base64.StdEncoding.DecodeString(*data.Msg)
	if err != nil {
		log.Panicf("decode msg data failed: %v", err)
		return
	}

	pMsg, err := tss.ParseWireMessage(msg, data.From, *data.IsBroadcast)
	if err != nil {
		log.Panicf("parse wire message error: %v", err)
		errCh <- party.WrapError(err)
		return
	}

	if _, err := party.Update(pMsg); err != nil {
		log.Panicf("update party state error: %v", err)
		errCh <- err
		return
	}
}

func sendCommunicateMsg(requestUUID string, conn *websocket.Conn, msg tss.Message) {
	msgBytes, _, err := msg.WireBytes()
	if err != nil {
		log.Panicf("wire msg bytes error: %v", err)
	}

	msgEncoded := base64.StdEncoding.EncodeToString(msgBytes)
	from := msg.GetFrom()
	to := msg.GetTo()
	isBroadcast := msg.IsBroadcast()

	data, _ := json.Marshal(types.MsgKeyGenCommunicate{
		From:        from,
		To:          to,
		Msg:         &msgEncoded,
		IsBroadcast: &isBroadcast,
	})

	conn.WriteJSON(types.Msg{
		RequestUUID: requestUUID,
		Type:        types.MsgTypeKeyGenCommunicate,
		Data:        data,
	})
}

func sendDoneMsg(requestUUID string, conn *websocket.Conn, from *tss.PartyID, ecdsaPub *crypto.ECPoint) {
	data, _ := json.Marshal(types.MsgKeyGenDone{
		From:     from,
		ECDSAPub: ecdsaPub,
	})

	conn.WriteJSON(types.Msg{
		RequestUUID: requestUUID,
		Type:        types.MsgTypeKeyGenDone,
		Data:        data,
	})
}

func sendErrorMsg(requestUUID string, conn *websocket.Conn, from *tss.PartyID, err error) {
	e := err.Error()

	data, _ := json.Marshal(types.MsgKeyGenError{
		From: from,
		Err:  &e,
	})

	conn.WriteJSON(types.Msg{
		RequestUUID: requestUUID,
		Type:        types.MsgTypeKeyGenError,
		Data:        data,
	})
}

func loadRequestState(requestUUID string) (party tss.Party, errCh chan *tss.Error, outCh chan tss.Message, endCh chan *keygen.LocalPartySaveData, ok bool) {
	requestCh, ok := requestChMap.Load(requestUUID)
	if !ok {
		return nil, nil, nil, nil, false
	}

	ch := requestCh.(*types.RequestState)

	return ch.Party, ch.ErrCh, ch.OutCh, ch.EndCh, true
}

func storeRequestState(requestUUID string, party tss.Party, errCh chan *tss.Error, outCh chan tss.Message, endCh chan *keygen.LocalPartySaveData) {
	requestChMap.Store(requestUUID, &types.RequestState{
		ErrCh: errCh,
		OutCh: outCh,
		EndCh: endCh,
		Party: party,
	})
}

func destroyRequestState(requestUUID string) {
	requestChMap.Delete(requestUUID)
}

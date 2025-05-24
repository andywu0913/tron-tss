package secret_manager

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	internalTSS "tron-tss/internal/tss"
	"tron-tss/types"

	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/gorilla/websocket"
)

var (
	requestChMap = sync.Map{}
)

func HandleConnection(managerID int) func(w http.ResponseWriter, r *http.Request) {
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

		log.Println("Client connected")

		for {
			var msgStruct types.Msg

			err := conn.ReadJSON(&msgStruct)
			if err != nil {
				log.Println("Error reading message:", err)
				log.Printf("Error type: %T", err)
				continue
			}
			log.Printf("Received message: type: %v", msgStruct.Type)

			requestUUID := msgStruct.RequestUUID

			switch msgStruct.Type {
			case types.MsgTypeGenKeyStart:
				var data types.MsgGenKeyStart
				err := json.Unmarshal(msgStruct.Data, &data)
				if err != nil {
					log.Println("Error reading message:", err)
					continue
				}

				handleGenKeyStart(managerID, requestUUID, conn, data.Threshold, data.PartyIDs)

			case types.MsgTypeGenKeyCommunicate:
				var data types.MsgGenKeyCommunicate
				err := json.Unmarshal(msgStruct.Data, &data)
				if err != nil {
					log.Println("Error reading message:", err)
					continue
				}

				handleGenKeyCommunicate(requestUUID, data)
			}
		}
	}
}

func handleGenKeyStart(managerID int, requestUUID string, conn *websocket.Conn, threshold int, partyIDs tss.SortedPartyIDs) {
	errCh := make(chan *tss.Error, len(partyIDs))
	outCh := make(chan tss.Message, len(partyIDs))
	endCh := make(chan *keygen.LocalPartySaveData, len(partyIDs))

	party := internalTSS.GenStart(managerID, threshold, partyIDs, errCh, outCh, endCh)
	storeRequestChannel(requestUUID, errCh, outCh, endCh, party)

	done := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case err := <-errCh:
				log.Printf("Error: %s", err)

				sendError(requestUUID, conn, err)
				return

			case msg := <-outCh:
				// dest := msg.GetTo()

				log.Printf("Send message from %v to %v, isBroadcast: %v", msg.GetFrom().Id, msg.GetTo(), msg.IsBroadcast())
				sendMsg(requestUUID, conn, msg)

				// if msg.IsBroadcast() { // broadcast
				// 	log.Printf("Broadcast message from %v to all", msg.GetFrom().Id)

				// 	for _, p := range partyIDs {
				// 		if p.Index == msg.GetFrom().Index {
				// 			continue
				// 		}

				// 		// go sharedPartyUpdater(p, msg, errCh)
				// 		sendMsg(conn, msg)
				// 	}
				// } else { // point-to-point
				// 	log.Printf("Message from %v to %v", msg.GetFrom().Id, dest)

				// 	if dest[0].Index == msg.GetFrom().Index {
				// 		log.Fatalf("party %d tried to send a message to itself (%d)", dest[0].Index, msg.GetFrom().Index)
				// 	}

				// 	// go sharedPartyUpdater(p, msg, errCh)
				// 	sendMsg(conn, msg)
				// }

			case saveData := <-endCh:
				log.Printf("Done. Received endCh channel: %+v", saveData)
				done <- struct{}{}
				return
			}
		}
	}()

	// log.Println("Waiting for keygen to be generated...")

	// select {
	// case <-done:
	// 	log.Println("Keygen generated successfully!")
	// case <-time.After(config.Timeout):
	// 	log.Println("Timeout waiting for keygen! Exiting...")
	// 	close(done)
	// }
}
func handleGenKeyCommunicate(requestUUID string, data types.MsgGenKeyCommunicate) {
	errCh, _, _, party := loadRequestChannel(requestUUID)

	if data.Err != nil {
		log.Panicf("get error data: %v", data.Err)
		return
	}

	log.Printf("Get message from slave %v", data.From)

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
	}
}

func sendMsg(requestUUID string, conn *websocket.Conn, msg tss.Message) {
	msgBytes, _, err := msg.WireBytes()
	if err != nil {
		log.Panicf("wire msg bytes error: %v", err)
	}

	msgEncoded := base64.StdEncoding.EncodeToString(msgBytes)
	from := msg.GetFrom()
	to := msg.GetTo()
	isBroadcast := msg.IsBroadcast()

	data, _ := json.Marshal(types.MsgGenKeyCommunicate{
		Msg:         &msgEncoded,
		From:        from,
		To:          to,
		IsBroadcast: &isBroadcast,
	})

	conn.WriteJSON(types.Msg{
		RequestUUID: requestUUID,
		Type:        types.MsgTypeGenKeyCommunicate,
		Data:        data,
	})
}

func sendError(requestUUID string, conn *websocket.Conn, err error) {
	e := err.Error()

	data, _ := json.Marshal(types.MsgGenKeyCommunicate{
		Err: &e,
	})

	conn.WriteJSON(types.Msg{
		RequestUUID: requestUUID,
		Type:        types.MsgTypeGenKeyCommunicate,
		Data:        data,
	})
}

func sharedPartyUpdater(party tss.Party, msg tss.Message, errCh chan<- *tss.Error) {
	// do not send a message from this party back to itself
	// if party.PartyID() == msg.GetFrom() {
	// 	return
	// }

	bz, _, err := msg.WireBytes()
	if err != nil {
		errCh <- party.WrapError(err)
		return
	}

	pMsg, err := tss.ParseWireMessage(bz, msg.GetFrom(), msg.IsBroadcast())
	if err != nil {
		errCh <- party.WrapError(err)
		return
	}

	if _, err := party.Update(pMsg); err != nil {
		errCh <- err
	}
}

func loadRequestChannel(requestUUID string) (errCh chan *tss.Error, outCh chan tss.Message, endCh chan *keygen.LocalPartySaveData, party tss.Party) {
	requestCh, ok := requestChMap.Load(requestUUID)
	if !ok {
		return nil, nil, nil, nil
	}

	ch := requestCh.(*types.RequestChannel)

	return ch.ErrCh, ch.OutCh, ch.EndCh, ch.Party
}

func storeRequestChannel(requestUUID string, errCh chan *tss.Error, outCh chan tss.Message, endCh chan *keygen.LocalPartySaveData, party tss.Party) {
	requestChMap.Store(requestUUID, &types.RequestChannel{
		ErrCh: errCh,
		OutCh: outCh,
		EndCh: endCh,
		Party: party,
	})
}

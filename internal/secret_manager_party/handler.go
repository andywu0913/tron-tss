package secret_manager_party

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"tron-tss/types"

	"github.com/bnb-chain/tss-lib/v2/crypto"
	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/gorilla/websocket"
)

func HandleConnection(partyID int) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var upgrader = websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("Failed to upgrade connection: %v", err)
			return
		}
		defer conn.Close()

		log.Printf("Coordinator connected.")

		keyGenHandler := new(KeyGenHandler)
		signHandler := new(SignHandler)

		for {
			var msgStruct types.Msg

			err := conn.ReadJSON(&msgStruct)
			if err != nil {
				if _, ok := err.(*websocket.CloseError); ok {
					log.Printf("Coordinator disconnected.")
					break
				}

				log.Printf("Failed to read message: %v", err)
				continue
			}

			log.Printf("Received message: request_uuid=%v, type=%v", msgStruct.RequestUUID, msgStruct.Type)

			switch msgStruct.Type {
			case types.MsgTypeKeyGenStart:
				var data types.KeyGenStartMsg

				err := json.Unmarshal(msgStruct.Data, &data)
				if err != nil {
					log.Println("Error unmarshal message:", err)
					continue
				}

				keyGenHandler.handleIncomingStartMsg(partyID, msgStruct.RequestUUID, conn, data.Threshold, data.PartyIDs)

			case types.MsgTypeKeyGenCommunicate:
				var data types.KeyGenCommunicateMsg

				err := json.Unmarshal(msgStruct.Data, &data)
				if err != nil {
					log.Println("Error unmarshal message:", err)
					continue
				}

				keyGenHandler.handleIncomingCommunicateMsg(data)

			case types.MsgTypeKeyGenError:
				// TODO: abort key gen process

			case types.MsgTypeSignStart:
				var data types.SignStartMsg

				err := json.Unmarshal(msgStruct.Data, &data)
				if err != nil {
					log.Println("Error unmarshal message:", err)
					continue
				}

			case types.MsgTypeSignCommunicate:
				var data types.SignCommunicateMsg

				err := json.Unmarshal(msgStruct.Data, &data)
				if err != nil {
					log.Println("Error unmarshal message:", err)
					continue
				}

			case types.MsgTypeSignError:
				// TODO: abort key gen process
			}
		}
	}
}

func sendCommunicateMsg(requestUUID string, conn *websocket.Conn, msg tss.Message) {
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

	err = conn.WriteJSON(types.Msg{
		RequestUUID: requestUUID,
		Type:        types.MsgTypeKeyGenCommunicate,
		Data:        data,
	})

	if err != nil {
		log.Printf("Failed to send communicate message: %v", err)
	}
}

func sendDoneMsg(requestUUID string, conn *websocket.Conn, from *tss.PartyID, ecdsaPub *crypto.ECPoint) {
	data, _ := json.Marshal(types.KeyGenDoneMsg{
		From:     from,
		ECDSAPub: ecdsaPub,
	})

	err := conn.WriteJSON(types.Msg{
		RequestUUID: requestUUID,
		Type:        types.MsgTypeKeyGenDone,
		Data:        data,
	})

	if err != nil {
		log.Printf("Failed to send done message: %v", err)
	}
}

func sendErrorMsg(requestUUID string, conn *websocket.Conn, from *tss.PartyID, err error) {
	e := err.Error()

	data, _ := json.Marshal(types.KeyGenErrorMsg{
		From: from,
		Err:  &e,
	})

	err = conn.WriteJSON(types.Msg{
		RequestUUID: requestUUID,
		Type:        types.MsgTypeKeyGenError,
		Data:        data,
	})
	if err != nil {
		log.Printf("Failed to send error message: %v", err)
	}
}

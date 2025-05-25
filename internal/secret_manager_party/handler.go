package secret_manager_party

import (
	"encoding/json"
	"log"
	"net/http"
	"tron-tss/types"

	"github.com/gorilla/websocket"
)

func HandleConnection(partyID string) func(w http.ResponseWriter, r *http.Request) {
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

		keyGenHandler := NewKeyGenHandler(conn)
		signHandler := NewSignHandler(conn)

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

				keyGenHandler.handleIncomingStartMsg(partyID, msgStruct.RequestUUID, data)

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

				signHandler.handleIncomingStartMsg(partyID, msgStruct.RequestUUID, data)

			case types.MsgTypeSignCommunicate:
				var data types.SignCommunicateMsg

				err := json.Unmarshal(msgStruct.Data, &data)
				if err != nil {
					log.Println("Error unmarshal message:", err)
					continue
				}

				signHandler.handleIncomingCommunicateMsg(data)

			case types.MsgTypeSignError:
				// TODO: abort key gen process
			}
		}
	}
}

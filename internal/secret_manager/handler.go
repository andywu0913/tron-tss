package secret_manager

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

func HandleConnection(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading connection:", err)
		return
	}
	defer conn.Close()

	log.Println("Client connected")

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			break
		}
		log.Printf("Received message: %s\n", msg)

		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			log.Println("Error writing message:", err)
			break
		}
	}
}

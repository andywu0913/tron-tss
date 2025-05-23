package main

import (
	"log"

	"github.com/gorilla/websocket"
)

type SecretManagerSlaveConfig struct {
	host string
}

var (
	secretManagerSlaveConfigMap = map[int]SecretManagerSlaveConfig{
		1: {
			host: "ws://localhost:8081/",
		},
		2: {
			host: "ws://localhost:8082/",
		},
		3: {
			host: "ws://localhost:8083/",
		},
		4: {
			host: "ws://localhost:8084/",
		},
		5: {
			host: "ws://localhost:8085/",
		},
	}
)

func main() {
	for i, config := range secretManagerSlaveConfigMap {
		conn, _, err := websocket.DefaultDialer.Dial(config.host, nil)
		if err != nil {
			log.Printf("Error connecting to slave %v: %v", i, err)
			continue
		}

		log.Printf("Connected to secret manager slave %v", i)

		go func(conn *websocket.Conn) {
			for {
				_, msg, err := conn.ReadMessage()
				if err != nil {
					log.Println("Error reading message from slave 1:", err)
					return
				}
				log.Printf("Message from slave 1: %s", msg)
			}
		}(conn)
	}

	select {}
}

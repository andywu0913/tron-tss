package main

import (
	"fmt"
	"log"
	"net/http"
	"tron-tss/config"
	secretManagerParty "tron-tss/internal/secret_manager_party"
)

const (
	partyID = config.PARTY_2
)

func main() {
	addr := fmt.Sprintf("0.0.0.0:%v", config.SecretManagerPartyConfigMap[partyID].Port)

	log.Printf("Starting WebSocket server on %s", addr)

	http.HandleFunc("/", secretManagerParty.HandleConnection(partyID))

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Panicf("Error starting WebSocket server: %v", err)
	}

	log.Printf("WebSocket server started at %s", addr)
}

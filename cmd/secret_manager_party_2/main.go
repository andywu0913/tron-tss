package main

import (
	"log"
	"net/http"
	secretManager "tron-tss/internal/secret_manager"
)

const (
	partyID = 2
	addr    = "0.0.0.0:8082"
)

func main() {
	log.Printf("Starting WebSocket server on %s", addr)

	http.HandleFunc("/", secretManager.HandleConnection(partyID))

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Panicf("Error starting WebSocket server: %v", err)
	}

	log.Printf("WebSocket server started at %s", addr)
}

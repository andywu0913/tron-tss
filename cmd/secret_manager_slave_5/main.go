package main

import (
	"log"
	"net/http"
	secretManager "tron-tss/internal/secret_manager"
)

const (
	id   = 5
	addr = "0.0.0.0:8085"
)

func main() {
	log.Printf("Starting WebSocket server on %s", addr)

	http.HandleFunc("/", secretManager.HandleConnection(id))

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Error starting WebSocket server: %v", err)
	}

	log.Printf("WebSocket server started at %s\n", addr)
}

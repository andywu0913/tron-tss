package main

import (
	"log"
	"net/http"
	secretManager "tron-tss/internal/secret_manager"
)

const (
	addr = "0.0.0.0:8083"
)

func main() {
	log.Printf("Starting WebSocket server on %s", addr)

	http.HandleFunc("/", secretManager.HandleConnection)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Error starting WebSocket server: %v", err)
	}

	log.Printf("WebSocket server started at %s\n", addr)
}

package main

import (
	"encoding/hex"
	"log"
	secretManagerCoordinator "tron-tss/internal/secret_manager_coordinator"
	"tron-tss/internal/tron"
)

const (
	testFromAddress = "TNUDwnN5nPzp8ps3TdPfboSRpkzd6Tz5Eu"
	toAddress       = "TMhvZyXm3NgQfhR59gRZwihwjxVSEb9ahH"
	amountTRX       = 1
)

func main() {
	// create address
	address := testFromAddress
	// address, err := new(secretManagerCoordinator.KeyGenRequest).GenerateTronAddress()
	// if err != nil {
	// 	log.Panicf("Failed to generate Tron address: %v", err)
	// }

	log.Printf("Generated Tron address: %s", address)

	// create transaction
	transaction, err := tron.CreateTransferTRXTransaction(address, toAddress, amountTRX*1_000_000)
	if err != nil {
		log.Panicf("Failed to create transaction: %v", err)
	}

	rawDataHexBytes, err := hex.DecodeString(transaction.RawDataHex)
	if err != nil {
		log.Panicf("Failed to decode raw data hex: %v", err)
	}

	signature, err := new(secretManagerCoordinator.SignRequest).SignTx(address, rawDataHexBytes)
	if err != nil {
		log.Panicf("Failed to sign transaction: %v", err)
	}
	log.Printf("Signature: %s", signature)

	// broadcast transaction
	transaction.Signature = []string{signature}

	response, err := tron.BroadcastTransaction(transaction)
	if err != nil {
		log.Panicf("Failed to broadcast transaction: %v", err)
	}

	log.Printf("Transaction broadcasted! Success: %v, TxID: %s", response.Result, response.TxID)
	log.Printf("Full response: %+v", response)
}

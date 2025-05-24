package main

import (
	secretManagerCoordinator "tron-tss/internal/secret_manager_coordinator"
)

func main() {
	var address string

	address, err := new(secretManagerCoordinator.KeyGenRequest).GenerateTronAddress()
	if err != nil {

	}

	// address = ""
	unsignedPayload := []byte("123")
	new(secretManagerCoordinator.Sign).Sign(address, unsignedPayload)
}

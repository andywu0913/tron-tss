package tron

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"tron-tss/config"
	"tron-tss/types"
)

func CreateTransferTRXTransaction(fromAddress, toAddress string, amountSun int64) (*types.Transaction, error) {
	payload := map[string]interface{}{
		"owner_address": fromAddress,
		"to_address":    toAddress,
		"amount":        amountSun,
		"visible":       true,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	resp, err := http.Post(config.TronAPIBaseUrl+config.CreateTxEndpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("api request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var response types.Transaction
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return &response, nil
}

func BroadcastTransaction(tx *types.Transaction) (*types.BroadcastResponse, error) {
	jsonData, err := json.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transaction: %v", err)
	}

	resp, err := http.Post(config.TronAPIBaseUrl+config.BroadcastEndpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("api request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var response types.BroadcastResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return &response, nil
}

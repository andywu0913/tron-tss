package main

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mr-tron/base58/base58"
)

type Transaction struct {
	Visible    bool     `json:"visible"`
	TxID       string   `json:"txID,omitempty"`
	Signature  []string `json:"signature,omitempty"`
	RawDataHex string   `json:"raw_data_hex"`
	RawData    struct {
		RefBlockBytes string `json:"ref_block_bytes"`
		RefBlockHash  string `json:"ref_block_hash"`
		Expiration    int64  `json:"expiration"`
		Timestamp     int64  `json:"timestamp"`
		Contract      []struct {
			Type      string `json:"type"`
			Parameter struct {
				TypeUrl string `json:"type_url"`
				Value   struct {
					OwnerAddress string `json:"owner_address"`
					ToAddress    string `json:"to_address"`
					Amount       int64  `json:"amount"`
				} `json:"value"`
			} `json:"parameter"`
		} `json:"contract"`
	} `json:"raw_data"`
}

type BroadcastResponse struct {
	Result  bool   `json:"result"`
	TxID    string `json:"txid,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func generateTronAddress(publicKeyECDSA *ecdsa.PublicKey) (string, error) {
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
	address = "41" + address[2:]
	addb, err := hex.DecodeString(address)
	if err != nil {
		return "", fmt.Errorf("generateTronAddress DecodeString fail, err(%v)", err.Error())
	}

	hash1 := s256(s256(addb))
	secret := hash1[:4]
	addb = append(addb, secret...)
	addr := base58.Encode(addb)
	return addr, nil
}

func createTransferTRXTransaction(fromAddress, toAddress string, amountSun int64) (*Transaction, error) {
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

	resp, err := http.Post(ApiBaseUrl+CreateTxEndpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var response Transaction
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return &response, nil
}

func formatCanonicalSignature(r, s *big.Int) []byte {
	rBytes := r.FillBytes(make([]byte, 32))
	sBytes := s.FillBytes(make([]byte, 32))
	return append(rBytes, sBytes...)
}

func findValidRecoveryID(hash []byte, canonicalSig []byte, pubKey *btcec.PublicKey) ([]byte, error) {
	origPubKeyBytes := pubKey.SerializeUncompressed()

	for v := byte(0); v < 4; v++ {
		sig := append(canonicalSig, v)

		recoveredPubKey, err := crypto.SigToPub(hash, sig)
		if err != nil {
			continue
		}

		recoveredPubKeyBytes := crypto.FromECDSAPub(recoveredPubKey)

		if bytes.Equal(recoveredPubKeyBytes, origPubKeyBytes) {
			return sig, nil
		}
	}

	return nil, errors.New("could not find valid recovery ID")
}

func broadcastTransaction(tx *Transaction) (*BroadcastResponse, error) {
	jsonData, err := json.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transaction: %v", err)
	}

	resp, err := http.Post(ApiBaseUrl+BroadcastEndpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var response BroadcastResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return &response, nil
}

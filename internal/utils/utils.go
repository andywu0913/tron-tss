package utils

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mr-tron/base58"
)

func SHA256(s []byte) []byte {
	h := sha256.New()
	h.Write(s)
	bs := h.Sum(nil)
	return bs
}

func GenerateTronAddress(publicKeyECDSA *ecdsa.PublicKey) (string, error) {
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
	address = "41" + address[2:]
	addb, err := hex.DecodeString(address)
	if err != nil {
		return "", fmt.Errorf("generateTronAddress DecodeString fail, err(%v)", err.Error())
	}

	hash1 := SHA256(SHA256(addb))
	secret := hash1[:4]
	addb = append(addb, secret...)
	addr := base58.Encode(addb)
	return addr, nil
}

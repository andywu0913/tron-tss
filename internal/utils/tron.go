package utils

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mr-tron/base58"
)

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

func FormatCanonicalSignature(r, s *big.Int) []byte {
	rBytes := r.FillBytes(make([]byte, 32))
	sBytes := s.FillBytes(make([]byte, 32))
	return append(rBytes, sBytes...)
}

func FindValidRecoveryID(hash []byte, canonicalSig []byte, pubKey *btcec.PublicKey) ([]byte, error) {
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

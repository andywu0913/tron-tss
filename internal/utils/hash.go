package utils

import (
	"crypto/sha256"
)

func SHA256(s []byte) []byte {
	h := sha256.New()
	h.Write(s)
	bs := h.Sum(nil)
	return bs
}

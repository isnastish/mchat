package session

import (
	"crypto/sha256"
	"fmt"
)

func Sha256(bytes []byte) string {
	h := sha256.New()
	h.Write(bytes)
	hashString := fmt.Sprintf("%X", h.Sum(nil))
	return hashString
}

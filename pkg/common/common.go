package common

import (
	"crypto/sha256"
	"fmt"
)

func StripCR(buf []byte, bytesRead int) []byte {
	for back := bytesRead - 1; back >= 0; back-- {
		if buf[back] == '\n' || buf[back] == '\r' {
			bytesRead--
			continue
		}
		break
	}
	return buf[:bytesRead]
}

func Sha256(bytes []byte) string {
	h := sha256.New()
	h.Write(bytes)
	hashString := fmt.Sprintf("%X", h.Sum(nil))
	return hashString
}

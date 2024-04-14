package main

import (
	// "bytes"
	// "crypto/rand"
	"flag"
	"fmt"
	// "math/big"

	"github.com/isnastish/chat/pkg/client"
)

// type PublicKey struct {
// 	N   *big.Int
// 	Exp *big.Int
// }

// type PrivateKey struct {
// 	N *big.Int
// 	D *big.Int
// }

// func encrypt(pub *PublicKey, m *big.Int) *big.Int {
// 	cipher := new(big.Int)
// 	// Enc(M) = M^e (mod n)
// 	cipher.Exp(m, pub.Exp, pub.N)
// 	return cipher
// }

// func decrypt(priv *PrivateKey, c *big.Int) *big.Int {
// 	m := new(big.Int)
// 	// Dec(C) = C^d (mod n)
// 	m.Exp(c, priv.D, priv.N)
// 	return m
// }

// func EncryptRSA(pub *PublicKey, message []byte) ([]byte, error) {
// 	// Compute length of key in bytes, rounding up
// 	keyLen := (pub.N.BitLen() + 7) / 8
// 	if len(message) > keyLen-11 {
// 		return nil, fmt.Errorf("len(m)=%v, too long", len(message))
// 	}

// 	// Following RFC 2313, using block type 02 as recommended for encryption:
// 	// EB = [00 || 02 || PS || 00 || binary data]
// 	psLen := keyLen - len(message) - 3
// 	eb := make([]byte, keyLen)
// 	eb[0] = 0x00
// 	eb[1] = 0x02

// 	fmt.Printf("psLen: %s\n", psLen)

// 	// Fill PS with random non-zero bytes.
// 	for i := 2; i < 2+psLen; {
// 		_, err := rand.Read(eb[i : i+1])
// 		if err != nil {
// 			return nil, err
// 		}
// 		if eb[i] != 0x00 {
// 			i++
// 		}
// 	}
// 	eb[2+psLen] = 0x00 // put one byte after PS

// 	// copy the contents of our message into the rest of the encryption block.
// 	copy(eb[3+psLen:], message)

// 	// Now the encryption block is complete; we take it as m-byte big.Int and RSA-encrypt it with the public key.
// 	mnum := new(big.Int).SetBytes(eb)
// 	cipher := encrypt(pub, mnum)

// 	// convert big.Int to a slice of bytes of length keyLen.
// 	padLen := keyLen - len(cipher.Bytes())
// 	for i := 0; i < padLen; i++ {
// 		eb[i] = 0x00
// 	}

// 	copy(eb[padLen:], cipher.Bytes())
// 	return eb, nil
// }

// func DecryptRSA(priv *PrivateKey, cipher []byte) ([]byte, error) {
// 	keyLen := (priv.N.BitLen() + 7) / 8
// 	if len(cipher) != keyLen {
// 		return nil, fmt.Errorf("len(c)=%v, want keyLen=%v", len(cipher), keyLen)
// 	}

// 	// Convert cipher into a bit.Int and decrypt it using the private key.
// 	cnum := new(big.Int).SetBytes(cipher)
// 	mnum := decrypt(priv, cnum)

// 	// Write the bytes of mnum into m, left-padding if needed
// 	m := make([]byte, keyLen)
// 	copy(m[keyLen-len(mnum.Bytes()):], mnum.Bytes())

// 	// Expect proper block 02 beginning.
// 	if m[0] != 0x00 {
// 		return nil, fmt.Errorf("m[0]=%v, want 0x00", m[0])
// 	}

// 	if m[1] != 0x02 {
// 		return nil, fmt.Errorf("m[1]=%v, want 0x02", m[1])
// 	}

// 	// Skip over random padding until a 0x00 byte is reached. +2 adjusts the index
// 	// back to the full slice.
// 	endPad := bytes.IndexByte(m[2:], 0x00) + 2
// 	if endPad < 2 {
// 		return nil, fmt.Errorf("end of padding not found")
// 	}

// 	return m[endPad+1:], nil
// }

// GenerateKeys generates a public/private key pair for RSA
// encryption/decryption with the given bitlen. RFC 2313 section 6 is the reference.
// func GenerateKeys(bitLen int) (*PublicKey, *PrivateKey, error) {

// }

func main() {
	var network string
	var address string

	flag.StringVar(&network, "network", "tcp", "Network protocol [TCP|UDP]")
	flag.StringVar(&address, "address", "127.0.0.1:5000", "Address, for example: localhost:8080")

	flag.Parse()

	c, err := client.NewClient(network, address)
	if err != nil {
		fmt.Printf("failed to create a client: %s\n", err.Error())
		return
	}
	c.Run()
}

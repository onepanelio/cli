package util

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// GenerateHMACKey generates a 32 byte random key that is encoded to base64
func GenerateHMACKey() string {
	b := make([]byte, 32)

	if _, err := rand.Read(b); err != nil {
		fmt.Println(err)
	}

	return base64.StdEncoding.EncodeToString(b)
}

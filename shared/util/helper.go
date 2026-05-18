package util

import (
	"crypto/rand"
	"encoding/hex"
)

func GenerateRandomString(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

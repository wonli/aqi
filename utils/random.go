package utils

import (
	"crypto/rand"
	"fmt"
)

func GetRandomString(n int) string {
	randBytes := make([]byte, n/2)
	_, err := rand.Read(randBytes)
	if err != nil {
		return ""
	}

	return fmt.Sprintf("%x", randBytes)
}

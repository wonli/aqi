package encrypt

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"

	"github.com/wonli/aqi/utils"
)

type Azdg struct {
	cipherHash string
}

func NewAzdg(key string) *Azdg {
	cipherHash := fmt.Sprintf("%x", md5.Sum([]byte(key)))
	return &Azdg{cipherHash: cipherHash}
}

func (a *Azdg) Encrypt(sourceText string) string {
	noise := utils.GetRandomString(32)
	inputData := []byte(sourceText)
	loopCount := len(inputData)
	outData := make([]byte, loopCount*2)

	for i, j := 0, 0; i < loopCount; i, j = i+1, j+1 {
		outData[j] = noise[i%32]
		j++
		outData[j] = inputData[i] ^ noise[i%32]
	}

	return base64.RawURLEncoding.EncodeToString([]byte(a.cipherEncode(fmt.Sprintf("%s", outData))))
}

func (a *Azdg) Decrypt(sourceText string) string {
	buf, err := base64.RawURLEncoding.DecodeString(sourceText)
	if err != nil {
		fmt.Printf("Decode (%q) failed: %v\n", sourceText, err)
		return ""
	}

	inputData := []byte(a.cipherEncode(fmt.Sprintf("%s", buf)))
	loopCount := len(inputData)

	if loopCount%2 != 0 {
		fmt.Printf("Decrypted data has unexpected length: %d\n", loopCount)
		return ""
	}

	outData := make([]byte, loopCount/2)
	var p int
	for i, j := 0, 0; i < loopCount; i, j = i+2, j+1 {
		outData[j] = inputData[i] ^ inputData[i+1]
		p++
	}

	return string(outData[:p])
}

func (a *Azdg) cipherEncode(sourceText string) string {
	inputData := []byte(sourceText)
	loopCount := len(inputData)
	outData := make([]byte, loopCount)
	for i := 0; i < loopCount; i++ {
		outData[i] = inputData[i] ^ a.cipherHash[i%32]
	}
	return fmt.Sprintf("%s", outData)
}

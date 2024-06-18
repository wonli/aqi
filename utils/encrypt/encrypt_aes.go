package encrypt

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/hex"
	"errors"

	"github.com/wonli/aqi/logger"
)

// AesEncrypt AES Encrypt,CBC
func AesEncrypt(origData []byte, key string) ([]byte, error) {
	encryptKey := getKey(key)
	block, err := aes.NewCipher(encryptKey)
	if err != nil {
		return nil, err
	}

	blockSize := block.BlockSize()
	origData = pkcs7Padding(origData, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, encryptKey[:blockSize])
	encrypted := make([]byte, len(origData))
	blockMode.CryptBlocks(encrypted, origData)
	return encrypted, nil
}

// AesDecrypt AES Decrypt
func AesDecrypt(encrypted []byte, key string) ([]byte, error) {
	encryptKey := getKey(key)
	block, err := aes.NewCipher(encryptKey)
	if err != nil {
		return nil, err
	}

	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, encryptKey[:blockSize])
	origData := make([]byte, len(encrypted))
	blockMode.CryptBlocks(origData, encrypted)
	err, data := pkcs7UnPadding(origData)
	return data, err
}

func getKey(key string) []byte {
	sha := sha1.New()
	_, err := sha.Write([]byte(key))
	if err != nil {
		logger.SugarLog.Errorf("Gen key fail %s", err.Error())
		return nil
	}

	byteKey := []byte(hex.EncodeToString(sha.Sum(nil)))
	return byteKey[:32]
}

func pkcs7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padText...)
}

func pkcs7UnPadding(origData []byte) (error, []byte) {
	length := len(origData)
	unPadding := int(origData[length-1])

	if length-unPadding < 0 {
		return errors.New("PKCS7 fail"), nil
	}

	return nil, origData[:(length - unPadding)]
}

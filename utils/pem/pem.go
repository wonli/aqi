package pem

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"time"
)

// LoadCertificate loads a certificate from its textual content.
func LoadCertificate(certificateStr string) (certificate *x509.Certificate, err error) {
	block, _ := pem.Decode([]byte(certificateStr))
	if block == nil {
		return nil, fmt.Errorf("decode certificate err")
	}

	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("the kind of PEM should be CERTIFICATE")
	}

	certificate, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate err:%s", err.Error())
	}

	return certificate, nil
}

// LoadPrivateKey loads a private key from its textual content.
func LoadPrivateKey(privateKeyStr string) (privateKey *rsa.PrivateKey, err error) {
	block, _ := pem.Decode([]byte(privateKeyStr))
	if block == nil {
		return nil, fmt.Errorf("decode private key err")
	}
	if block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("the kind of PEM should be PRVATE KEY")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key err:%s", err.Error())
	}
	privateKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%s is not rsa private key", privateKeyStr)
	}
	return privateKey, nil
}

// LoadPublicKey loads a public key from its textual text content.
func LoadPublicKey(publicKeyStr string) (publicKey *rsa.PublicKey, err error) {
	block, _ := pem.Decode([]byte(publicKeyStr))
	if block == nil {
		return nil, errors.New("decode public key error")
	}
	if block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("the kind of PEM should be PUBLIC KEY")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key err:%s", err.Error())
	}
	publicKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%s is not rsa public key", publicKeyStr)
	}
	return publicKey, nil
}

// LoadCertificateWithPath loads a certificate from a file path.
func LoadCertificateWithPath(path string) (certificate *x509.Certificate, err error) {
	certificateBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read certificate pem file err:%s", err.Error())
	}
	return LoadCertificate(string(certificateBytes))
}

// LoadPrivateKeyWithPath loads a private key from a file path.
func LoadPrivateKeyWithPath(path string) (privateKey *rsa.PrivateKey, err error) {
	privateKeyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read private pem file err:%s", err.Error())
	}
	return LoadPrivateKey(string(privateKeyBytes))
}

// LoadPublicKeyWithPath load public key
func LoadPublicKeyWithPath(path string) (publicKey *rsa.PublicKey, err error) {
	publicKeyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read certificate pem file err:%s", err.Error())
	}
	return LoadPublicKey(string(publicKeyBytes))
}

// GetCertificateSerialNumber retrieves the serial number from a certificate.
func GetCertificateSerialNumber(certificate x509.Certificate) string {
	return fmt.Sprintf("%X", certificate.SerialNumber.Bytes())
}

// IsCertificateExpired checks if the certificate is expired at a specific time.
func IsCertificateExpired(certificate x509.Certificate, now time.Time) bool {
	return now.After(certificate.NotAfter)
}

// IsCertificateValid checks if the certificate is valid at a specific time.
func IsCertificateValid(certificate x509.Certificate, now time.Time) bool {
	return now.After(certificate.NotBefore) && now.Before(certificate.NotAfter)
}

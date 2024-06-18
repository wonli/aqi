package config

import (
	"bytes"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"html/template"
)

//go:embed config.yaml
var defaultConfig []byte

// DefaultConfigTpl store template data.
type DefaultConfigTpl struct {
	JwtSecurity string
}

func generateRandomHex(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func GetDefaultConfig() (string, error) {
	jwtSecurity, err := generateRandomHex(16)
	if err != nil {
		return "", err
	}

	config := DefaultConfigTpl{
		JwtSecurity: jwtSecurity,
	}

	tmpl, err := template.New("config").Parse(string(defaultConfig))
	if err != nil {
		return "", err
	}

	var renderedConfig bytes.Buffer
	err = tmpl.Execute(&renderedConfig, config)
	if err != nil {
		return "", err
	}

	return renderedConfig.String(), nil
}

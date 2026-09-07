package config

import (
	"strings"
	"testing"
)

func TestGetDefaultConfig(t *testing.T) {
	content, err := GetDefaultConfig()
	if err != nil {
		t.Fatalf("GetDefaultConfig returned error: %v", err)
	}

	if !strings.Contains(content, "jwtSecurity:") {
		t.Fatalf("config content missing jwtSecurity:\n%s", content)
	}
	if !strings.Contains(content, "\nlog:\n") {
		t.Fatalf("config content missing log block:\n%s", content)
	}
}

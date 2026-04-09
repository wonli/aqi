package config

import (
	"strings"
	"testing"
)

func TestGetDefaultConfig_InsertsAppConfigBlockBeforeLog(t *testing.T) {
	content, err := GetDefaultConfig("theme: theme/default\nrouteSuffix: \"\"")
	if err != nil {
		t.Fatalf("GetDefaultConfig returned error: %v", err)
	}

	themeIndex := strings.Index(content, "theme: theme/default")
	logIndex := strings.Index(content, "\nlog:\n")
	if themeIndex < 0 || logIndex < 0 {
		t.Fatalf("config content missing expected sections:\n%s", content)
	}
	if themeIndex > logIndex {
		t.Fatalf("app config block should appear before log block:\n%s", content)
	}
}

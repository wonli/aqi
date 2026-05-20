package aqi

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestWriteDefaultConfigFileDoesNotOverwriteExistingFile(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "config.yaml")
	original := []byte("user: config\n")
	if err := os.WriteFile(filename, original, 0644); err != nil {
		t.Fatalf("failed to create existing config: %v", err)
	}

	app := &AppConfig{
		ConfigPath: dir,
		ConfigName: "config",
		ConfigType: "yaml",
	}

	_, err := app.writeDefaultConfigFile()
	if err == nil {
		t.Fatal("expected an error when config already exists")
	}
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("expected os.ErrExist, got %v", err)
	}

	got, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	if string(got) != string(original) {
		t.Fatalf("existing config was overwritten:\n%s", string(got))
	}
}

func TestWriteDefaultConfigFileCreatesMissingFile(t *testing.T) {
	dir := t.TempDir()
	app := &AppConfig{
		ConfigPath:     dir,
		ConfigName:     "config",
		ConfigType:     "yaml",
		AppConfigBlock: "custom: true",
	}

	filename, err := app.writeDefaultConfigFile()
	if err != nil {
		t.Fatalf("writeDefaultConfigFile returned error: %v", err)
	}
	if filename != filepath.Join(dir, "config.yaml") {
		t.Fatalf("unexpected filename: %s", filename)
	}

	got, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	if !strings.Contains(string(got), "custom: true") {
		t.Fatalf("generated config missing app config block:\n%s", string(got))
	}
}

func TestIsConfigFileNotFound(t *testing.T) {
	reader := viper.New()
	reader.SetConfigName("missing-config")
	reader.SetConfigType("yaml")
	reader.AddConfigPath(t.TempDir())

	err := reader.ReadInConfig()
	if err == nil {
		t.Fatal("expected missing config error")
	}
	if !isConfigFileNotFound(err) {
		t.Fatalf("expected missing config error to be recognized, got %T: %v", err, err)
	}

	if isConfigFileNotFound(errors.New("parse failed")) {
		t.Fatal("non-missing config errors should not trigger default config generation")
	}
}

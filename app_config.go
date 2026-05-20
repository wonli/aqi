package aqi

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"

	"github.com/wonli/aqi/internal/config"
)

func (a *AppConfig) GetDataPath(dir string) string {
	return filepath.Join(a.ConfigPath, a.DataPath, dir)
}

func (a *AppConfig) IsDevMode() bool {
	return a.devMode
}

func (a *AppConfig) WriteDefaultConfig() error {
	filename, err := a.writeDefaultConfigFile()
	if err != nil {
		return err
	}

	color.Green("Configuration file has been created: " + filename)
	os.Exit(0)
	return nil
}

func (a *AppConfig) writeDefaultConfigFile() (string, error) {
	ctx, err := config.GetDefaultConfig(a.AppConfigBlock)
	if err != nil {
		return "", err
	}

	filename := filepath.Join(a.ConfigPath, a.ConfigName+"."+a.ConfigType)
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return filename, fmt.Errorf("configuration file already exists, refusing to overwrite: %s: %w", filename, err)
		}
		return filename, err
	}

	if _, err = file.WriteString(ctx); err != nil {
		_ = file.Close()
		return filename, err
	}

	if err = file.Close(); err != nil {
		return filename, err
	}

	return filename, nil
}

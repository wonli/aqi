package aqi

import (
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
	ctx, err := config.GetDefaultConfig(a.AppConfigBlock)
	if err != nil {
		return err
	}

	filename := filepath.Join(a.ConfigPath, a.ConfigName+"."+a.ConfigType)
	err = os.WriteFile(filename, []byte(ctx), 0644)
	if err != nil {
		return err
	}

	color.Green("Configuration file has been created: " + filename)
	os.Exit(0)
	return nil
}

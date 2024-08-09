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
	workerDir, err := os.Getwd()
	if err != nil {
		return err
	}

	ctx, err := config.GetDefaultConfig()
	if err != nil {
		return err
	}

	filename := filepath.Join(workerDir, a.ConfigName+"."+a.ConfigType)
	err = os.WriteFile(filename, []byte(ctx), 0755)
	if err != nil {
		return err
	}

	color.Green("Configuration file has been created: " + filename)
	os.Exit(0)
	return nil
}

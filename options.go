package aqi

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/wonli/aqi/ws"
)

type Option func(config *AppConfig) error

func LogConfig(configKeyPath string) Option {
	return func(config *AppConfig) error {
		config.LogPathKey = configKeyPath
		return nil
	}
}

func DataPath(path string) Option {
	return func(config *AppConfig) error {
		config.DataPath = path
		return nil
	}
}

func ConfigFile(file string) Option {
	if !filepath.IsAbs(file) {
		workerDir, err := os.Getwd()
		if err != nil {
			log.Fatalf("获取工作目录失败: %s", err.Error())
		}

		file = filepath.Join(workerDir, file)
	}

	return func(config *AppConfig) error {
		configPath := filepath.Dir(file)
		config.ConfigPath = configPath

		fileType := filepath.Ext(file)
		config.ConfigType = fileType[1:]

		filename := filepath.Base(file)
		config.ConfigName = strings.TrimSuffix(filename, fileType)

		return nil
	}
}

func Server(name ...string) Option {
	return func(config *AppConfig) error {
		config.Servername = name
		return nil
	}
}

func Language(lng string) Option {
	return func(config *AppConfig) error {
		config.Language = lng
		return nil
	}
}

func HttpServer(name, portFindPath string) Option {
	return func(config *AppConfig) error {
		config.Servername = append(config.Servername, name)
		config.HttpServerPortFindPath = portFindPath
		return nil
	}
}

func WatchHandler(handler func()) Option {
	return func(config *AppConfig) error {
		config.WatchHandler = handler
		return nil
	}
}

func Guard(fn ws.GuardFunc) Option {
	return func(config *AppConfig) error {
		config.Guard = fn
		return nil
	}
}

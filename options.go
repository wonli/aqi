package aqi

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"

	internalconfig "github.com/wonli/aqi/internal/config"
	"github.com/wonli/aqi/telemetry"
	"github.com/wonli/aqi/ws"
)

type Option func(config *AppConfig) error

// ClusterTransport is the minimal transport contract AQI needs for inter-node routing.
type ClusterTransport = ws.ClusterTransport

// ClusterMessageHandler is the inbound callback supplied to custom cluster transports.
type ClusterMessageHandler = ws.ClusterMessageHandler

// ClusterTransportFactory builds a custom cluster transport after AQI config has loaded.
type ClusterTransportFactory = ws.ClusterTransportFactory

// ConfigBuilder is used to modify the generated default config before it is written.
type ConfigBuilder = internalconfig.Builder

// DefaultConfig registers a hook that can modify AQI's default config before it is written.
func DefaultConfig(hook func(*ConfigBuilder)) Option {
	return func(config *AppConfig) error {
		config.DefaultConfigHook = hook
		return nil
	}
}

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

// WebSocketMaxFrameSize configures gobwas/ws Reader.MaxFrameSize.
// Zero keeps gobwas's default unlimited behavior.
func WebSocketMaxFrameSize(size int64) Option {
	return func(config *AppConfig) error {
		config.WebSocketMaxFrameSize = size
		return nil
	}
}

func Telemetry(provider telemetry.Provider) Option {
	return func(config *AppConfig) error {
		config.Telemetry = provider
		return nil
	}
}

// WithCluster enables realtime multi-node routing using the reserved redis.aqi store.
func WithCluster() Option {
	return func(config *AppConfig) error {
		config.Cluster = true
		return nil
	}
}

// WithClusterTransport enables cluster routing with a caller-provided transport.
// The factory runs after AQI configuration has loaded and receives the inbound
// callback it must invoke for messages received from other AQI nodes.
func WithClusterTransport(factory ClusterTransportFactory) Option {
	return func(config *AppConfig) error {
		if factory == nil {
			return errors.New("aqi cluster: transport factory is nil")
		}
		config.Cluster = true
		config.clusterTransportFactory = factory
		return nil
	}
}

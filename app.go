package aqi

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"

	"github.com/wonli/aqi/config"
	"github.com/wonli/aqi/logger"
	"github.com/wonli/aqi/validate"
)

type AppConfig struct {

	//运行时数据存储基础路径
	DataPath string

	//应用日志文件配置路径
	LogPathKey string

	//默认语言
	Language string

	//开发模式
	devMode bool

	//服务名称，support.Version
	//当指定 HttpServerPortFindPath 时，在配置读取之后从配置路径获取http端口
	Servername             []string
	ServerPort             string
	HttpServerPortFindPath string

	ConfigType string //配置文件类型
	ConfigPath string //配置文件路径
	ConfigName string //配置文件名称

	Configs map[string]any

	HttpServer http.Handler //http server

	RemoteProvider *RemoteProvider //远程配置支持etcd, consul

	WatchHandler func()
}

var acf *AppConfig

func Init(options ...Option) *AppConfig {
	acf = &AppConfig{
		Language:   "zh",
		ConfigType: "yaml",
		ConfigName: "config",
		ServerPort: "1091",
		LogPathKey: "log",
		DataPath:   "data",
	}

	for _, opt := range options {
		if opt != nil {
			err := opt(acf)
			if err != nil {
				color.Red("error %s", err.Error())
				os.Exit(1)
			}
		}
	}

	if acf.ConfigPath == "" {
		workerDir, err := os.Getwd()
		if err != nil {
			color.Red("Failed to get the configuration file directory: %s", err.Error())
			os.Exit(1)
		}

		acf.ConfigPath = workerDir
	}

	if CommitVersion == "" {
		acf.devMode = true
		acf.ConfigName = fmt.Sprintf("%s-dev", acf.ConfigName)
	}

	// 设置环境变量的前缀
	// 自动将环境变量绑定到 Viper 配置中
	viper.SetEnvPrefix("")
	viper.AutomaticEnv()

	//设置配置文件
	viper.SetConfigName(acf.ConfigName)
	viper.SetConfigType(acf.ConfigType)

	viper.AddConfigPath(acf.ConfigPath)
	err := viper.ReadInConfig()
	if err != nil {
		if acf.RemoteProvider == nil {
			err = acf.WriteDefaultConfig()
			if err != nil {
				color.Red("Error gen default config file: %s", err.Error())
				os.Exit(1)
			}

			color.Red("failed to read config file: %s", err.Error())
			os.Exit(1)
		}

		color.Red("Remote configuration will be used: %s", err.Error())
	} else {
		acf.Configs = viper.AllSettings()
	}

	if acf.LogPathKey == "" {
		color.Red("Please specify LogPathKey")
		os.Exit(1)
	}

	isSetDevMode := viper.IsSet("devMode")
	if isSetDevMode {
		setDevModel := viper.GetBool("devMode")
		acf.devMode = setDevModel
	}

	viper.Set("devMode", acf.devMode)
	if acf.RemoteProvider != nil {
		_ = viper.AddRemoteProvider(string(acf.RemoteProvider.Name), acf.RemoteProvider.Endpoint, acf.RemoteProvider.Path)
		viper.SetConfigType(acf.RemoteProvider.Type)

		err := viper.ReadRemoteConfig()
		if err != nil {
			color.Red("Failed to read remote config")
			os.Exit(1)
		}

		go func() {
			t := time.NewTicker(time.Minute * 30)
			for range t.C {
				err2 := viper.WatchRemoteConfig()
				if err2 != nil {
					logger.SugarLog.Errorf("unable to read remote config: %v", err2)
					continue
				}

				if acf.WatchHandler != nil {
					acf.WatchHandler()
				}
			}
		}()
	}

	//处理http服务端口信息
	if acf.HttpServerPortFindPath != "" {
		port := viper.GetString(acf.HttpServerPortFindPath)
		if port == "" {
			port = acf.ServerPort
		}

		if strings.Contains(port, ":") {
			s := strings.Split(port, ":")
			port = s[len(s)-1]
		}

		acf.ServerPort = port
		acf.Servername = append(acf.Servername, "is now running at http://0.0.0.0:"+port)
	}

	//打印系统信息
	if acf.Servername != nil {
		AsciiLogo(acf.Servername...)
	}

	if CommitVersion == "" {
		color.Green("dev mode -- use config %s", acf.ConfigName+"."+acf.ConfigType)
	}

	var c config.Logger
	err = viper.UnmarshalKey(acf.LogPathKey, &c)
	if err != nil {
		color.Red("failed to init app log")
		os.Exit(1)
	}

	if !filepath.IsAbs(c.LogPath) {
		c.LogPath = acf.GetDataPath(c.LogPath)
	}

	//初始化日志库
	logger.Init(c)

	//validate语言配置
	validate.InitTranslator(acf.Language)

	//配置文件更新回调
	viper.OnConfigChange(func(e fsnotify.Event) {
		logger.SugarLog.Infof("config file changed: %s", e.Name)
		if acf.WatchHandler != nil {
			acf.WatchHandler()
		}
	})

	//监听配置
	viper.WatchConfig()
	return acf
}

package config

import (
	"time"

	"go.uber.org/zap/zapcore"
)

type Logger struct {
	LogFile        string
	RuntimeLogFile string

	LogPath    string `yaml:"logPath"`    // Path of the log file
	MaxSize    int    `yaml:"maxSize"`    // Maximum log file size in MB
	MaxBackups int    `yaml:"maxBackups"` // Maximum number of log file backups
	MaxAge     int    `yaml:"maxAge"`     // Maximum number of days to retain log files
	Compress   bool   `yaml:"compress"`   // Whether to enable gzip compression
	UseCaller  bool   `yaml:"useCaller"`  // Whether to enable Zap Caller
}

// GetEncoder 根据模式获取编码器
func (config *Logger) GetEncoder(mode string) zapcore.Encoder {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.FullCallerEncoder,
	}

	if config.UseCaller {
		encoderConfig.CallerKey = "caller"
		encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	}

	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
	}

	if mode == "file" {
		return zapcore.NewConsoleEncoder(encoderConfig)
	}

	//控制台模式下显示颜色
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	return zapcore.NewConsoleEncoder(encoderConfig)
}

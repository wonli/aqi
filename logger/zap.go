package logger

import (
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/wonli/aqi/internal/config"
)

var ZapLog *zap.Logger
var SugarLog *zap.SugaredLogger

func Init(c config.Logger) {
	if c.LogPath == "" {
		c.LogPath = "."
	}

	if c.LogFile == "" {
		c.LogFile = "error.log"
	}

	isAbsPath := filepath.IsAbs(c.LogPath)
	if !isAbsPath {
		path, err := os.Getwd()
		if err != nil {
			color.Red("Failed to get the runtime directory %s", err.Error())
			os.Exit(0)
		}

		c.LogPath = filepath.Join(path, c.LogPath)
		err = os.MkdirAll(c.LogPath, 0755)
		if err != nil {
			color.Red("Failed to create log directory %s", err.Error())
			os.Exit(0)
		}
	}

	hook := lumberjack.Logger{
		Filename:   filepath.Join(c.LogPath, c.LogFile),
		MaxSize:    c.MaxSize,
		MaxBackups: c.MaxBackups,
		MaxAge:     c.MaxAge,
		Compress:   c.Compress,
	}

	stdLog := zapcore.NewCore(c.GetEncoder(""), zapcore.AddSync(os.Stdout), zap.DebugLevel)
	fileLog := zapcore.NewCore(c.GetEncoder("file"), zapcore.AddSync(&hook), zap.DebugLevel)

	//ZapLog
	ZapLog = zap.New(zapcore.NewTee(stdLog, fileLog), zap.AddCaller(), zap.Development())

	//sugar
	SugarLog = ZapLog.Sugar()
}

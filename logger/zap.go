package logger

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/wonli/aqi/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var ZapLog *zap.Logger
var RuntimeLog *zap.Logger
var SugarLog *zap.SugaredLogger

func Init(c config.Logger) {
	if c.LogPath == "" {
		c.LogPath = "."
	}

	if c.LogFile == "" {
		c.LogFile = "app.log"
	}

	if c.RuntimeLogFile == "" {
		ext := filepath.Ext(c.LogFile)
		fileName := strings.TrimSuffix(c.LogFile, ext)
		c.RuntimeLogFile = fileName + "_runtime" + ext
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

	runtimeHook := lumberjack.Logger{
		Filename:   filepath.Join(c.LogPath, c.RuntimeLogFile),
		MaxSize:    c.MaxSize,
		MaxBackups: c.MaxBackups,
		MaxAge:     c.MaxAge,
		Compress:   c.Compress,
	}

	stdEncoder := newLimitLengthEncoder(c.GetEncoder(""), 300)
	stdLog := zapcore.NewCore(stdEncoder, zapcore.AddSync(os.Stdout), zap.InfoLevel)

	fileEncoder := getFileStyleEncoder()
	fileLog := zapcore.NewCore(fileEncoder, zapcore.AddSync(&hook), zap.InfoLevel)
	rFileLog := zapcore.NewCore(fileEncoder, zapcore.AddSync(&runtimeHook), zap.InfoLevel)

	ZapLog = zap.New(zapcore.NewTee(stdLog, fileLog), zap.AddCaller(), zap.Development())
	RuntimeLog = zap.New(zapcore.NewTee(rFileLog), zap.AddCaller(), zap.Development())

	//sugar
	SugarLog = ZapLog.Sugar()

	defer func() {
		_ = ZapLog.Sync()
		_ = SugarLog.Sync()
		_ = RuntimeLog.Sync()
	}()
}

// 创建一个自定义的 encoder 来限制消息长度
type limitLengthEncoder struct {
	zapcore.Encoder
	limit int
}

func (l *limitLengthEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	if len(entry.Message) > l.limit {
		entry.Message = entry.Message[:l.limit] + "..."
	}
	return l.Encoder.EncodeEntry(entry, fields)
}

func newLimitLengthEncoder(encoder zapcore.Encoder, limit int) zapcore.Encoder {
	return &limitLengthEncoder{
		Encoder: encoder,
		limit:   limit,
	}
}

// getFileStyleEncoder 获取文件风格的日志编码器
func getFileStyleEncoder() zapcore.Encoder {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "", // 这些字段会在自定义格式中处理
		LevelKey:       "",
		NameKey:        "logger",
		CallerKey:      "",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeName:     zapcore.FullNameEncoder,
	}

	return &fileStyleEncoder{
		Encoder: zapcore.NewConsoleEncoder(encoderConfig),
	}
}

// fileStyleEncoder 自定义编码风格输出
type fileStyleEncoder struct {
	zapcore.Encoder
}

func (e *fileStyleEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	// 正则表达式过滤base64的主体
	filterRegex := regexp.MustCompile(`("data:[^"]*;base64,)([^"]*)`)
	if filterRegex.MatchString(entry.Message) {
		// 替换中间部分，保留前后部分
		entry.Message = filterRegex.ReplaceAllString(entry.Message, `$1..(replace)..`)
	}

	// 创建输出缓冲区
	buf := buffer.NewPool().Get()

	// 标题分隔线
	buf.AppendString("\n---------------------------------- START ----------------------------------\n")

	// 时间和日志级别放到每一行前
	logPrefix := entry.Time.Format("2006-01-02 15:04:05.000") + " "
	switch entry.Level {
	case zapcore.DebugLevel:
		logPrefix += "[DEBUG] "
	case zapcore.InfoLevel:
		logPrefix += "[INFO ] "
	case zapcore.WarnLevel:
		logPrefix += "[WARN ] "
	case zapcore.ErrorLevel:
		logPrefix += "[ERROR] "
	case zapcore.DPanicLevel:
		logPrefix += "[PANIC] "
	case zapcore.PanicLevel:
		logPrefix += "[PANIC] "
	case zapcore.FatalLevel:
		logPrefix += "[FATAL] "
	default:
		logPrefix += "[UNK  ] "
	}

	// 调用者信息
	if entry.Caller.Defined {
		buf.AppendString(logPrefix)
		buf.AppendString(entry.Caller.TrimmedPath())
		buf.AppendString("\n")
	}

	// 消息内容：前面加上时间和级别
	buf.AppendString(logPrefix)
	buf.AppendString(entry.Message)

	// 如果有额外的字段，附加到日志信息后
	if len(fields) > 0 {
		for _, field := range fields {
			logStr := ""
			if field.String != "" {
				logStr = field.String
			} else if field.Integer > 0 {
				logStr = fmt.Sprintf("%d", field.Integer)
			} else {
				logStr = fmt.Sprintf("%v", field.Interface)
			}

			if field.Key != "" {
				logStr = fmt.Sprintf("%s(%s)", field.Key, logStr)
			}

			buf.AppendString("\n")
			buf.AppendString(logPrefix)
			buf.AppendString(logStr)
		}
	}

	// 结束分隔线
	buf.AppendString("\n----------------------------------  END  ----------------------------------\n")

	return buf, nil
}

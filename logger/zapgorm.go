package logger

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

// ZapGormLogger implements gorm.io/gorm/logger.Interface using zap.SugaredLogger.
type ZapGormLogger struct {
	sugar *zap.SugaredLogger
	cfg   gormLogger.Config
}

// NewZapGormLogger constructs a ZapGormLogger.
// If sugar is nil, it falls back to logger.SugarLog; ensure Init() was called.
func NewZapGormLogger(sugar *zap.SugaredLogger, cfg gormLogger.Config) gormLogger.Interface {
	if sugar == nil {
		sugar = SugarLog
	}
	if cfg.SlowThreshold == 0 {
		cfg.SlowThreshold = 200 * time.Millisecond
	}
	return &ZapGormLogger{sugar: sugar, cfg: cfg}
}

// LogMode sets the logging level.
func (l *ZapGormLogger) LogMode(level gormLogger.LogLevel) gormLogger.Interface {
	newCfg := l.cfg
	newCfg.LogLevel = level
	return &ZapGormLogger{sugar: l.sugar, cfg: newCfg}
}

func (l *ZapGormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.cfg.LogLevel < gormLogger.Info {
		return
	}
	l.sugar.Infof(msg, data...)
}

func (l *ZapGormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.cfg.LogLevel < gormLogger.Warn {
		return
	}
	l.sugar.Warnf(msg, data...)
}

func (l *ZapGormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.cfg.LogLevel < gormLogger.Error {
		return
	}
	l.sugar.Errorf(msg, data...)
}

// Trace prints SQL logs. It honors SlowThreshold and IgnoreRecordNotFoundError.
func (l *ZapGormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rows int64), err error) {
	if l.cfg.LogLevel == gormLogger.Silent {
		return
	}
	elapsed := time.Since(begin)
	sql, rows := fc()

	switch {
	case err != nil && l.cfg.LogLevel >= gormLogger.Error:
		if l.cfg.IgnoreRecordNotFoundError && errors.Is(err, gorm.ErrRecordNotFound) {
			break
		}
		l.sugar.Errorf("gorm sql error: elapsed=%s rows=%d err=%v sql=%s", elapsed, rows, err, sql)
	case l.cfg.SlowThreshold != 0 && elapsed > l.cfg.SlowThreshold && l.cfg.LogLevel >= gormLogger.Warn:
		l.sugar.Warnf("gorm slow sql: elapsed=%s rows=%d sql=%s", elapsed, rows, sql)
	case l.cfg.LogLevel >= gormLogger.Info:
		l.sugar.Infof("gorm sql: elapsed=%s rows=%d sql=%s", elapsed, rows, sql)
	}
}
package store

import (
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"github.com/wonli/aqi/internal/config"
	"github.com/wonli/aqi/logger"
)

type SQLiteStore struct {
	configKey string

	gormDB      *gorm.DB
	options     *gorm.Config
	callback    callback
	hasCallback bool
}

func (m *SQLiteStore) Config() *config.Sqlite {
	var r *config.Sqlite
	err := viper.UnmarshalKey(m.configKey, &r)
	if err != nil {
		return nil
	}

	return r
}

func (m *SQLiteStore) ConfigKey() string {
	return m.configKey
}

func (m *SQLiteStore) Options(options *gorm.Config) {
	m.options = options
}

func (m *SQLiteStore) Callback(fn callback) {
	m.callback = fn
	m.hasCallback = true
}

func (m *SQLiteStore) Use() *gorm.DB {
	if m.gormDB != nil {
		return m.gormDB
	}

	r := m.Config()
	if r == nil {
		return nil
	}

	conf := &gorm.Config{
		Logger: gormLogger.Default.LogMode(gormLogger.LogLevel(r.LogLevel)),
		NamingStrategy: schema.NamingStrategy{
			TablePrefix: r.Prefix,
		},
	}

	if m.options != nil {
		if m.options.Logger == nil {
			m.options.Logger = conf.Logger
		}

		if m.options.NamingStrategy == nil {
			m.options.NamingStrategy = conf.NamingStrategy
		}
	} else {
		m.options = conf
	}

	db, err := gorm.Open(sqlite.Open(r.Database), conf)
	if err != nil {
		logger.SugarLog.Error("Connect to SQLite error", zap.String("error", err.Error()))
		return nil
	}

	if m.hasCallback {
		m.callback(db)
	}

	sqlDB, err := db.DB()
	if err != nil {
		logger.SugarLog.Error("Ping SQLite error",
			zap.String("error", err.Error()),
		)
		return nil
	}

	// 设置连接池参数
	sqlDB.SetMaxIdleConns(r.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(r.ConnMaxLifetime)
	if r.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(r.MaxOpenConns)
	}

	m.gormDB = db
	return db
}

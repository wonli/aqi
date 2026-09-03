package store

import (
	"sync"

	"github.com/libtnb/sqlite"
	"github.com/spf13/viper"
	"go.uber.org/zap"
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
	mu          sync.Mutex
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
	m.mu.Lock()
	m.options = options
	m.mu.Unlock()
}

func (m *SQLiteStore) Callback(fn callback) {
	m.mu.Lock()
	m.callback = fn
	m.hasCallback = true
	m.mu.Unlock()
}

func (m *SQLiteStore) Use() *gorm.DB {
	m.mu.Lock()
	defer m.mu.Unlock()

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

	options := m.options
	if options != nil {
		if options.Logger == nil {
			options.Logger = conf.Logger
		}

		if options.NamingStrategy == nil {
			options.NamingStrategy = conf.NamingStrategy
		}
	} else {
		options = conf
	}

	db, err := gorm.Open(sqlite.Open(r.Database), options)
	if err != nil {
		if logger.SugarLog != nil {
			logger.SugarLog.Error("Connect to SQLite error", zap.String("error", err.Error()))
		}
		return nil
	}

	if m.hasCallback {
		m.callback(db)
	}

	sqlDB, err := db.DB()
	if err != nil {
		if logger.SugarLog != nil {
			logger.SugarLog.Error("Ping SQLite error",
				zap.String("error", err.Error()),
			)
		}
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

package store

import (
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"github.com/wonli/aqi/internal/config"
	"github.com/wonli/aqi/logger"
)

type MySQLStore struct {
	configKey string

	gormDB      *gorm.DB
	options     *gorm.Config
	callback    callback
	hasCallback bool
}

func (m *MySQLStore) Config() *config.MySQL {
	var r *config.MySQL
	err := viper.UnmarshalKey(m.configKey, &r)
	if err != nil {
		return nil
	}

	return r
}

func (m *MySQLStore) ConfigKey() string {
	return m.configKey
}

func (m *MySQLStore) Options(options *gorm.Config) {
	m.options = options
}

func (m *MySQLStore) Callback(fn callback) {
	m.callback = fn
	m.hasCallback = true
}

func (m *MySQLStore) Use() *gorm.DB {
	if m.gormDB != nil {
		return m.gormDB
	}

	r := m.Config()
	if r == nil {
		return nil
	}

	if r.Enable == 0 {
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

	db, err := gorm.Open(mysql.Open(r.GetDsn()), conf)
	if err != nil {
		logger.SugarLog.Error("Failed to connect to MySQL database", zap.String("error", err.Error()))
		return nil
	}

	if m.hasCallback {
		m.callback(db)
	}

	sqlDB, err := db.DB()
	if err != nil {
		logger.SugarLog.Error("Error pinging database", zap.String("error", err.Error()))
		return nil
	}

	sqlDB.SetMaxIdleConns(r.Idle)
	sqlDB.SetConnMaxLifetime(r.MaxLifetime)
	if r.MaxOpen > 0 {
		sqlDB.SetMaxOpenConns(r.MaxOpen)
	}

	m.gormDB = db
	return db
}

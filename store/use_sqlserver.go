package store

import (
	"github.com/spf13/viper"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"github.com/wonli/aqi/internal/config"
	"github.com/wonli/aqi/logger"
)

type SqlServerStore struct {
	configKey string

	gormDB      *gorm.DB
	options     *gorm.Config
	callback    callback
	hasCallback bool
}

func (m *SqlServerStore) Config() *config.SqlServer {
	var r *config.SqlServer
	err := viper.UnmarshalKey(m.configKey, &r)
	if err != nil {
		return nil
	}

	return r
}

func (m *SqlServerStore) ConfigKey() string {
	return m.configKey
}

func (m *SqlServerStore) Options(options *gorm.Config) {
	m.options = options
}

func (m *SqlServerStore) Callback(fn callback) {
	m.callback = fn
	m.hasCallback = true
}

func (m *SqlServerStore) Use() *gorm.DB {
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

	db, err := gorm.Open(sqlserver.Open(r.GetDsn()), m.options)
	if err != nil {
		logger.SugarLog.Errorf("%s (gorm.open)", err.Error())
		return nil
	}

	if m.hasCallback {
		m.callback(db)
	}

	sqlDB, err := db.DB()
	if err != nil {
		logger.SugarLog.Errorf("%s (ping)", err.Error())
		return nil
	}

	if r.Idle > 0 {
		sqlDB.SetMaxIdleConns(r.Idle)
	}

	if r.MaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(r.MaxLifetime)
	}

	if r.MaxOpen > 0 {
		sqlDB.SetMaxOpenConns(r.MaxOpen)
	}

	m.gormDB = db
	return db
}

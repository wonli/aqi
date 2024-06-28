package store

import (
	"github.com/spf13/viper"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"github.com/wonli/aqi/config"
	"github.com/wonli/aqi/logger"
)

type SqlServerStore struct {
	configKey string
}

func (m *SqlServerStore) Config() *config.SqlServer {
	var r *config.SqlServer
	err := viper.UnmarshalKey(m.configKey, &r)
	if err != nil {
		return nil
	}

	return r
}

func (m *SqlServerStore) Use() *gorm.DB {
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

	db, err := gorm.Open(sqlserver.Open(r.GetDsn()), conf)
	if err != nil {
		logger.SugarLog.Errorf("%s (gorm.open)", err.Error())
		return nil
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

	return db
}

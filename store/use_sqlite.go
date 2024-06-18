package store

import (
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"github.com/wonli/aqi/config"
	"github.com/wonli/aqi/logger"
)

type SQLiteStore struct {
	configKey string
}

func (m *SQLiteStore) Config() *config.Sqlite {
	var r *config.Sqlite
	err := viper.UnmarshalKey(m.configKey, &r)
	if err != nil {
		return nil
	}

	return r
}

func (m *SQLiteStore) Use() *gorm.DB {
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

	db, err := gorm.Open(sqlite.Open(r.Database), conf)
	if err != nil {
		logger.SugarLog.Error("Connect to SQLite error", zap.String("error", err.Error()))
		return nil
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

	return db
}

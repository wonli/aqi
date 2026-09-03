package store

import (
	"sync"

	"gorm.io/gorm"
)

type callback func(db *gorm.DB)

var (
	mysqlStores     sync.Map
	sqliteStores    sync.Map
	redisStores     sync.Map
	sqlServerStores sync.Map
)

func DB(configKey string, options ...*gorm.Config) *MySQLStore {
	newStore := &MySQLStore{configKey: configKey}
	if len(options) > 0 && options[0] != nil {
		newStore.Options(options[0])
	}

	store, _ := mysqlStores.LoadOrStore(configKey, newStore)
	return store.(*MySQLStore)
}

func SQLite(configKey string, options ...*gorm.Config) *SQLiteStore {
	newStore := &SQLiteStore{configKey: configKey}
	if len(options) > 0 && options[0] != nil {
		newStore.Options(options[0])
	}

	store, _ := sqliteStores.LoadOrStore(configKey, newStore)
	return store.(*SQLiteStore)
}

func Redis(configKey string) *RedisStore {
	newStore := &RedisStore{configKey: configKey}
	store, _ := redisStores.LoadOrStore(configKey, newStore)
	return store.(*RedisStore)
}

func SqlServer(configKey string, options ...*gorm.Config) *SqlServerStore {
	newStore := &SqlServerStore{configKey: configKey}
	if len(options) > 0 && options[0] != nil {
		newStore.Options(options[0])
	}

	store, _ := sqlServerStores.LoadOrStore(configKey, newStore)
	return store.(*SqlServerStore)
}

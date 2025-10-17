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
	if store, ok := mysqlStores.Load(configKey); ok {
		return store.(*MySQLStore)
	}

	newStore := &MySQLStore{configKey: configKey}
	if len(options) > 0 && options[0] != nil {
		newStore.Options(options[0])
	}

	mysqlStores.Store(configKey, newStore)
	return newStore
}

func SQLite(configKey string, options ...*gorm.Config) *SQLiteStore {
	if store, ok := sqliteStores.Load(configKey); ok {
		return store.(*SQLiteStore)
	}

	newStore := &SQLiteStore{configKey: configKey}
	if len(options) > 0 && options[0] != nil {
		newStore.Options(options[0])
	}

	sqliteStores.Store(configKey, newStore)
	return newStore
}

func Redis(configKey string) *RedisStore {
	if store, ok := redisStores.Load(configKey); ok {
		return store.(*RedisStore)
	}

	newStore := &RedisStore{configKey: configKey}
	redisStores.Store(configKey, newStore)
	return newStore
}

func SqlServer(configKey string, options ...*gorm.Config) *SqlServerStore {
	if store, ok := sqlServerStores.Load(configKey); ok {
		return store.(*SqlServerStore)
	}

	newStore := &SqlServerStore{configKey: configKey}
	if len(options) > 0 && options[0] != nil {
		newStore.Options(options[0])
	}

	sqlServerStores.Store(configKey, newStore)
	return newStore
}

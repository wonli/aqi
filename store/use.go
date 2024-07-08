package store

import "gorm.io/gorm"

type callback func(db *gorm.DB)

func DB(configKey string) *MySQLStore {
	return &MySQLStore{configKey: configKey}
}

func SQLite(configKey string) *SQLiteStore {
	return &SQLiteStore{configKey: configKey}
}

func Redis(configKey string) *RedisStore {
	return &RedisStore{configKey: configKey}
}

func SqlServer(configKey string) *SqlServerStore {
	return &SqlServerStore{configKey: configKey}
}

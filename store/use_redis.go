package store

import (
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"

	"github.com/wonli/aqi/config"
)

type RedisStore struct {
	configKey string
}

func (s *RedisStore) Config() *config.Redis {
	var r *config.Redis
	err := viper.UnmarshalKey(s.configKey, &r)
	if err != nil {
		return nil
	}

	return r
}

func (s *RedisStore) Use() *redis.Client {
	r := s.Config()
	if r == nil {
		return nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:            r.Addr,
		Username:        r.Username,
		Password:        r.Pwd,
		DB:              r.Db,
		MinIdleConns:    r.MinIdleConns,
		ConnMaxIdleTime: r.IdleTimeout,
	})

	return client
}

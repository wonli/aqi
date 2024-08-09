package store

import (
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"

	"github.com/wonli/aqi/internal/config"
)

type RedisStore struct {
	redisClient *redis.Client
	configKey   string
}

func (s *RedisStore) Config() *config.Redis {
	var r *config.Redis
	err := viper.UnmarshalKey(s.configKey, &r)
	if err != nil {
		return nil
	}

	return r
}

func (s *RedisStore) ConfigKey() string {
	return s.configKey
}

func (s *RedisStore) Use() *redis.Client {
	if s.redisClient != nil {
		return s.redisClient
	}

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

	s.redisClient = client
	return client
}

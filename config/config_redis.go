package config

import (
	"time"
)

type Redis struct {
	Addr         string        `yaml:"addr" json:"addr"`
	Username     string        `yaml:"username" json:"username"`
	Pwd          string        `yaml:"pwd" json:"pwd"`
	Db           int           `yaml:"db" json:"db"`
	LogLevel     int           `yaml:"logLevel"`
	MinIdleConns int           `yaml:"minIdleConns" json:"minIdleConns"`         // Minimum number of idle connections, useful when establishing new connections is slow.
	IdleTimeout  time.Duration `yaml:"idleTimeout" json:"idleTimeout,omitempty"` // Time after which idle connections are closed by the client, default is 5 minutes, -1 disables the setting.
}

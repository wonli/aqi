package config

import (
	"fmt"
	"time"
)

type MySQL struct {
	Host          string        `yaml:"host" json:"host"`
	Port          int           `yaml:"port" json:"port"`
	User          string        `yaml:"user" json:"user"`
	Password      string        `yaml:"password" json:"password"`
	Database      string        `yaml:"database" json:"database"`
	Prefix        string        `yaml:"prefix" json:"prefix"`
	LogLevel      int           `yaml:"logLevel"`
	Idle          int           `yaml:"idle" json:"idle"`
	IdleTime      time.Duration `yaml:"idleTime" json:"idleTime,omitempty"`
	MaxLifetime   time.Duration `yaml:"maxLifetime" json:"maxLifetime,omitempty"`     // Maximum time a connection can be reused
	HeartBeatTime time.Duration `yaml:"heartBeatTime" json:"heartBeatTime,omitempty"` // Heartbeat check time for MySQL server connections
	Active        int           `yaml:"active" json:"active"`                         // Active connections
	MaxOpen       int           `yaml:"maxOpen" json:"maxOpen"`                       // Maximum open connections
	Enable        int           `yaml:"enable" json:"enable"`                         // 0, disabled; 1, enabled

	AutoMigrateTables bool // Whether to synchronize table structures
}

func (dbc *MySQL) GetDsn() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbc.User,
		dbc.Password,
		dbc.Host,
		dbc.Port,
		dbc.Database,
	)
}

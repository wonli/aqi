package config

import (
	"time"
)

type Sqlite struct {
	Database        string        `yaml:"database"`        // Path to the database file
	Prefix          string        `yaml:"prefix"`          // Table prefix
	MaxIdleConns    int           `yaml:"maxIdleConns"`    // Maximum number of idle connections in the pool
	MaxOpenConns    int           `yaml:"maxOpenConns"`    // Maximum number of open connections to the database
	LogLevel        int           `yaml:"logLevel"`        // Log level
	ConnMaxLifetime time.Duration `yaml:"connMaxLifetime"` // Maximum lifetime of connections
}

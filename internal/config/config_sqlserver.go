package config

import (
	"fmt"
	"net/url"
	"time"
)

type SqlServer struct {
	Host                   string        `yaml:"host"`
	Port                   int           `yaml:"port"`
	User                   string        `yaml:"user"`
	Pwd                    string        `yaml:"pwd"`
	Database               string        `yaml:"database"`
	Prefix                 string        `yaml:"prefix"`
	Encrypt                string        `yaml:"encrypt"`
	LogLevel               int           `yaml:"logLevel"`
	TrustServerCertificate string        `yaml:"trustServerCertificate"`
	Idle                   int           `yaml:"idle"`
	IdleTime               time.Duration `yaml:"idleTime"`
	MaxLifetime            time.Duration `yaml:"maxLifetime"` // Maximum time a connection can be reused
	MaxOpen                int           `yaml:"maxOpen"`
}

func (m *SqlServer) GetDsn() string {
	return fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s&encrypt=%s&trustServerCertificate=%s",
		m.User,
		url.QueryEscape(m.Pwd),
		m.Host,
		m.Port,
		m.Database,
		m.Encrypt,
		m.TrustServerCertificate,
	)
}

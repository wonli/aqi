package config

import "time"

type Dialog struct {
	OpInterval     time.Duration `yaml:"opInterval" json:"opInterval,omitempty"`         // Interval for sending op messages
	IdleInterval   time.Duration `yaml:"idleInterval" json:"idleInterval,omitempty"`     // Interval for inserting system time in the session list
	SessionExpire  time.Duration `yaml:"sessionExpire" json:"sessionExpire,omitempty"`   // Session expiration duration
	GuardInterval  time.Duration `yaml:"guardInterval" json:"guardInterval,omitempty"`   // Scan interval duration
	AssignInterval time.Duration `yaml:"assignInterval" json:"assignInterval,omitempty"` // Assignment interval time
}

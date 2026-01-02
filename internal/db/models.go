package db

import (
	"time"
)

type LocalVM struct {
	ID          uint   `gorm:"primaryKey"`
	ProviderID  int64  `gorm:"uniqueIndex"`
	Alias       string `gorm:"uniqueIndex"`
	ServerName  string
	IP          string
	Region      string
	Tier        string
	SSHKeyPath  string
	OwnerWallet string    `gorm:"index"`
	ExpiresAt   time.Time `gorm:"index"`
	CreatedAt   time.Time
}

package db

import (
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Init() error {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".config", "entropy")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	dbPath := filepath.Join(configDir, "entropy.db")
	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return err
	}

	return DB.AutoMigrate(&LocalVM{})
}

package utils

import (
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type StorageConfig struct {
	DBType   string `json:"db_type"`
	HostName string `json:"host_name"`
	DBName   string `json:"db_name"`
	UserName string `json:"user_name"`
	Password string `json:"password"`
}

// NewStorage :Opening a database and save the reference to `Database` struct.
func NewStorage(cfg StorageConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=True",
		cfg.UserName, cfg.Password, cfg.HostName, cfg.DBName)
	return gorm.Open(mysql.New(mysql.Config{
		DSN:                       dsn,
		DefaultStringSize:         256,   // default size for string fields
		DisableDatetimePrecision:  true,  // disable datetime precision, which not supported before MySQL 5.6
		DontSupportRenameIndex:    true,  // drop & create when rename index, rename index not supported before MySQL 5.7, MariaDB
		DontSupportRenameColumn:   true,  // `change` when rename column, rename column not supported before MySQL 8, MariaDB
		SkipInitializeWithVersion: false, // autoconfigure based on currently MySQL version
	}), &gorm.Config{})
}

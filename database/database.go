/*
 * Copyright 2023 ICON Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package database

import (
	"fmt"
	"strings"

	"github.com/glebarez/sqlite"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	DriverMysql    = "mysql"
	DriverPostgres = "postgres"
	DriverSQLite   = "sqlite"
)

type DB interface{}

type Config struct {
	Driver   string `json:"driver"`
	User     string `json:"user"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     uint   `json:"port"`
	DBName   string `json:"dbname"`
}

var zeroDefaultDatetimePrecision = 0

func OpenDatabase(cfg Config, l log.Logger) (*gorm.DB, error) {
	gcfg := &gorm.Config{
		Logger: &databaseLogger{
			l: l.WithFields(log.Fields{log.FieldKeyModule: "database"}),
		},
	}
	switch cfg.Driver {
	case DriverMysql:
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)
		return gorm.Open(mysql.New(mysql.Config{
			DSN:                       dsn,
			DefaultStringSize:         256,  // default size for string fields
			DisableDatetimePrecision:  true, // disable datetime precision, which not supported before MySQL 5.6
			DefaultDatetimePrecision:  &zeroDefaultDatetimePrecision,
			DontSupportRenameIndex:    true,  // drop & create when rename index, rename index not supported before MySQL 5.7, MariaDB
			DontSupportRenameColumn:   true,  // `change` when rename column, rename column not supported before MySQL 8, MariaDB
			SkipInitializeWithVersion: false, // autoconfigure based on currently MySQL version
		}), gcfg)
	case DriverPostgres:
		dsn := fmt.Sprintf("user=%s password=%s host=%s port=%d dbname=%s sslmode=disable",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)
		return gorm.Open(postgres.Open(dsn), gcfg)
	case DriverSQLite:
		dsn := fmt.Sprintf("file:%s", cfg.DBName)
		if len(cfg.User) > 0 {
			auth := fmt.Sprintf("_auth&_auth_user=%s&_auth_pass=%s",
				cfg.User, cfg.Password)
			if !strings.Contains(dsn, "?") {
				auth = "?" + auth
			}
			dsn = dsn + auth
		}
		return gorm.Open(sqlite.Open(dsn), gcfg)
	default:
		return nil, errors.Errorf("not support db type:%s", cfg.Driver)
	}
}

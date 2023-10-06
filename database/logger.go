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
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/icon-project/btp2/common/log"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	DefaultLogSlowThreshold = time.Millisecond * 200
)

type databaseLogger struct {
	l log.Logger
}

func (l *databaseLogger) LogMode(level logger.LogLevel) logger.Interface {
	var lv log.Level
	switch level {
	case logger.Silent:
		lv = log.PanicLevel
	case logger.Error:
		lv = log.ErrorLevel
	case logger.Warn:
		lv = log.WarnLevel
	case logger.Info:
		lv = log.InfoLevel
	}
	l.l.SetLevel(lv)
	return l
}

func (l *databaseLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	l.l.Logf(log.InfoLevel, msg, data...)
}

func (l *databaseLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	l.l.Logf(log.InfoLevel, msg, data...)
}

func (l *databaseLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	l.l.Logf(log.ErrorLevel, msg, data...)
}

func (l *databaseLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.l.GetLevel() <= log.PanicLevel {
		return
	}

	lv := l.l.GetLevel()
	elapsed := time.Since(begin)
	switch {
	case err != nil && lv <= log.ErrorLevel && !errors.Is(err, gorm.ErrRecordNotFound):
		sql, rows := fc()
		l.l.Logf(log.ErrorLevel, "err:%s\n[%.3fms] [rows:%v] %s", err, float64(elapsed.Nanoseconds())/1e6, rows, sql)
	case elapsed > DefaultLogSlowThreshold && lv <= log.WarnLevel:
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", DefaultLogSlowThreshold)
		l.l.Logf(log.WarnLevel, "%s\n[%.3fms] [rows:%v] %s", slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql)
	case lv <= log.TraceLevel:
		sql, rows := fc()
		l.l.Logf(log.TraceLevel, "[%.3fms] [rows:%v] %s", float64(elapsed.Nanoseconds())/1e6, rows, sql)
	}
}

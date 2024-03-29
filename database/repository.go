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
	"database/sql"
	"math"
	"sync"
	"time"

	"github.com/icon-project/btp2/common/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Model struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Pageable struct {
	// Page 0-indexed
	Page uint `json:"page"`
	// Size zero for unlimited
	Size uint `json:"size"`
	// Sort for example "FIELD desc,FIELD"
	Sort string `json:"sort,omitempty"`
}

type Page[T any] struct {
	Content       []T      `json:"content"`
	TotalElements int      `json:"total_elements"`
	TotalPages    int      `json:"total_pages"`
	Pageable      Pageable `json:"pageable"`
}

func (p *Page[T]) ToAny() *Page[any] {
	r := &Page[any]{
		TotalElements: p.TotalElements,
		TotalPages:    p.TotalPages,
		Pageable:      p.Pageable,
	}
	r.Content = make([]any, len(p.Content))
	for i, e := range p.Content {
		r.Content[i] = e
	}
	return r
}

type Repository[T any] interface {
	Save(v *T) error
	SaveIf(v *T, predicate func(found *T) bool, lock interface{}) (bool, error)
	Update(column string, value interface{}, query interface{}, conds ...interface{}) error
	Updates(value interface{}, query interface{}, conds ...interface{}) error
	Delete(query interface{}, conds ...interface{}) error
	Exists(query interface{}, conds ...interface{}) (bool, error)
	Count(query interface{}, conds ...interface{}) (int64, error)
	FindByID(id interface{}) (*T, error)
	FindOne(query interface{}, conds ...interface{}) (*T, error)
	FindOneWithOrder(order string, query interface{}, conds ...interface{}) (*T, error)
	Find(query interface{}, conds ...interface{}) ([]T, error)
	FindWithOrder(order string, query interface{}, conds ...interface{}) ([]T, error)
	Page(p Pageable, query interface{}, conds ...interface{}) (*Page[T], error)
	Transaction(fc func(tx Repository[T]) error, opts ...*sql.TxOptions) error
	TransactionWithLock(fc func(tx Repository[T]) error, lock interface{}) error
	Begin(opts ...*sql.TxOptions) (tx Repository[T])
	Rollback()
	Commit()
	DB() DB
	WithDB(db DB) (tx Repository[T], err error)
	TransactionNested(db DB, fc func(tx Repository[T]) error, opts ...*sql.TxOptions) error
	BeginNested(db DB, opts ...*sql.TxOptions) (tx Repository[T], err error)
}

type DefaultRepository[T any] struct {
	db   *gorm.DB
	name string
	mtx  *sync.RWMutex
	mtxs map[interface{}]*sync.Mutex
}

func NewDefaultRepository[T any](db *gorm.DB, name string) (*DefaultRepository[T], error) {
	if err := db.Table(name).AutoMigrate(new(T)); err != nil {
		return nil, err
	}
	return &DefaultRepository[T]{
		db:   db,
		name: name,
		mtx:  &sync.RWMutex{},
		mtxs: make(map[interface{}]*sync.Mutex),
	}, nil
}

func (r *DefaultRepository[T]) table() *gorm.DB {
	if len(r.name) > 0 {
		return r.db.Table(r.name)
	} else {
		return r.db
	}
}

func (r *DefaultRepository[T]) Save(v *T) error {
	return r.table().Save(v).Error
}

func (r *DefaultRepository[T]) getOrNewLock(k interface{}) *sync.Mutex {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	l, ok := r.mtxs[k]
	if !ok {
		l = &sync.Mutex{}
		r.mtxs[k] = l
	}
	return l
}

// SaveIf if predicate func returns true, save the given v.
// the argument of predicate func is result of query by primary key of given v.
// note : if given v has not primary key value, the found could be any record
func (r *DefaultRepository[T]) SaveIf(v *T, predicate func(found *T) bool, lock interface{}) (save bool, err error) {
	if v == nil {
		return false, errors.Errorf("invalid v, must be not nil")
	}
	if predicate == nil {
		return false, errors.Errorf("invalid predicate, must be not nil")
	}
	mtx := r.getOrNewLock(lock)
	mtx.Lock()
	defer mtx.Unlock()
	err = r.Transaction(func(tx Repository[T]) error {
		ret := tx.(*DefaultRepository[T]).table()
		found := new(T)
		*found = *v
		if fe := ret.First(found).Error; fe != nil {
			if fe != gorm.ErrRecordNotFound {
				return fe
			}
			found = nil
		}
		if save = predicate(found); !save {
			return nil
		}
		return tx.Save(v)
	})
	return
}

func (r *DefaultRepository[T]) Update(column string, value interface{}, query interface{}, conds ...interface{}) error {
	return r.where(query, conds...).Update(column, value).Error
}

func (r *DefaultRepository[T]) Updates(value interface{}, query interface{}, conds ...interface{}) error {
	return r.where(query, conds...).Updates(value).Error
}

func (r *DefaultRepository[T]) Delete(query interface{}, conds ...interface{}) error {
	v := new(T)
	return r.where(query, conds...).Delete(v).Error
}

func (r *DefaultRepository[T]) Exists(query interface{}, conds ...interface{}) (bool, error) {
	ret := r.table().Select("count(*) > 0")
	if query != nil {
		ret = ret.Where(query, conds...)
	}
	var exists bool
	err := ret.Find(&exists).Error
	return exists, err
}

func (r *DefaultRepository[T]) where(query interface{}, conds ...interface{}) *gorm.DB {
	ret := r.table()
	if query != nil {
		ret = ret.Where(query, conds...)
	}
	return ret
}

func (r *DefaultRepository[T]) Count(query interface{}, conds ...interface{}) (int64, error) {
	var count int64
	if err := r.where(query, conds...).Count(&count).Error; err != nil {
		return -1, err
	}
	return count, nil
}

func filterError(err error) error {
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	return nil
}

func (r *DefaultRepository[T]) FindByID(id interface{}) (*T, error) {
	v := new(T)
	err := r.table().Clauses(clause.Eq{
		Column: clause.PrimaryColumn,
		Value:  id,
	}).First(v).Error
	if err != nil {
		return nil, filterError(err)
	}
	return v, nil
}

func (r *DefaultRepository[T]) FindOne(query interface{}, conds ...interface{}) (*T, error) {
	v := new(T)
	err := r.where(query, conds...).First(v).Error
	if err != nil {
		return nil, filterError(err)
	}
	return v, nil
}

func (r *DefaultRepository[T]) FindOneWithOrder(order string, query interface{}, conds ...interface{}) (*T, error) {
	v := new(T)
	err := r.where(query, conds...).Order(order).First(v).Error
	if err != nil {
		return nil, filterError(err)
	}
	return v, err
}

func (r *DefaultRepository[T]) Find(query interface{}, conds ...interface{}) ([]T, error) {
	var l []T
	err := r.where(query, conds...).Find(&l).Error
	if err != nil {
		return nil, filterError(err)
	}
	return l, err
}

func (r *DefaultRepository[T]) FindWithOrder(order string, query interface{}, conds ...interface{}) ([]T, error) {
	var l []T
	err := r.where(query, conds...).Order(order).Find(&l).Error
	if err != nil {
		return nil, filterError(err)
	}
	return l, err
}

func (r *DefaultRepository[T]) Page(p Pageable, query interface{}, conds ...interface{}) (*Page[T], error) {
	ret := r.where(query, conds...)
	var count int64
	ret = ret.Count(&count)
	if p.Size > 0 {
		ret = ret.Offset(int(p.Page * p.Size)).Limit(int(p.Size))
	}
	if len(p.Sort) > 0 {
		ret = ret.Order(p.Sort)
	}
	var l []T
	err := ret.Find(&l).Error
	if err != nil {
		return nil, filterError(err)
	}
	totalPages := 0
	if count > 0 {
		totalPages = 1
		if p.Size > 0 {
			totalPages = int(math.Ceil(float64(count) / float64(p.Size)))
		}
	}
	return &Page[T]{
		Pageable:      p,
		TotalElements: int(count),
		TotalPages:    totalPages,
		Content:       l,
	}, nil
}

func (r *DefaultRepository[T]) tx(db *gorm.DB) *DefaultRepository[T] {
	return &DefaultRepository[T]{
		db:   db,
		name: r.name,
		mtx:  r.mtx,
		mtxs: r.mtxs,
	}
}

func (r *DefaultRepository[T]) Transaction(fc func(tx Repository[T]) error, opts ...*sql.TxOptions) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return fc(r.tx(tx))
	}, opts...)
}

func (r *DefaultRepository[T]) TransactionWithLock(fc func(tx Repository[T]) error, lock interface{}) error {
	mtx := r.getOrNewLock(lock)
	mtx.Lock()
	defer mtx.Unlock()
	return r.db.Transaction(func(tx *gorm.DB) error {
		return fc(r.tx(tx))
	})
}

func (r *DefaultRepository[T]) Begin(opts ...*sql.TxOptions) (tx Repository[T]) {
	return r.tx(r.db.Begin(opts...))
}

func (r *DefaultRepository[T]) Rollback() {
	r.db.Rollback()
}

func (r *DefaultRepository[T]) Commit() {
	r.db.Commit()
}

func (r *DefaultRepository[T]) DB() DB {
	return r.db
}

func cast(db DB) (*gorm.DB, error) {
	gdb, ok := db.(*gorm.DB)
	if !ok {
		return nil, errors.Errorf("not supported DB type:%T", db)
	}
	return gdb, nil
}

func (r *DefaultRepository[T]) WithDB(db DB) (tx Repository[T], err error) {
	gdb, err := cast(db)
	if err != nil {
		return nil, err
	}
	return r.tx(gdb), nil
}

func (r *DefaultRepository[T]) TransactionNested(db DB, fc func(tx Repository[T]) error, opts ...*sql.TxOptions) error {
	gdb, err := cast(db)
	if err != nil {
		return err
	}
	return gdb.Transaction(func(tx *gorm.DB) error {
		return fc(r.tx(tx))
	}, opts...)
}

func (r *DefaultRepository[T]) BeginNested(db DB, opts ...*sql.TxOptions) (tx Repository[T], err error) {
	gdb, err := cast(db)
	if err != nil {
		return nil, err
	}
	return r.tx(gdb.Begin(opts...)), nil
}

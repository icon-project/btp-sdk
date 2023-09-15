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
	"math"
	"reflect"
	"time"

	"gorm.io/gorm"
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

type DefaultRepository[T any] struct {
	db        *gorm.DB
	name      string
	model     T
	modelType reflect.Type
	sliceType reflect.Type
}

func NewDefaultRepository[T any](db *gorm.DB, name string, model T) (*DefaultRepository[T], error) {
	if err := db.Table(name).AutoMigrate(&model); err != nil {
		return nil, err
	}
	modelType := reflect.TypeOf(model)
	return &DefaultRepository[T]{
		db:        db,
		name:      name,
		model:     model,
		modelType: modelType,
		sliceType: reflect.SliceOf(modelType),
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

func (r *DefaultRepository[T]) Delete(query interface{}, conds ...interface{}) error {
	return r.table().Delete(query, conds...).Error
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
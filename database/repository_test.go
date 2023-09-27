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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	dbConfig = Config{
		Driver: DriverSQLite,
		DBName: ":memory:",
	}
)

type Struct struct {
	Model
	Field string
}

func assertEqualModel(t *testing.T, expected, actual Model) bool {
	if !assert.Equal(t, expected.ID, actual.ID) {
		return false
	}
	if !assert.True(t, expected.CreatedAt.Equal(actual.CreatedAt)) {
		return false
	}
	return assert.True(t, expected.UpdatedAt.Equal(actual.UpdatedAt))
}

func assertEqualStruct(t *testing.T, expected, actual Struct) bool {
	if !assertEqualModel(t, expected.Model, actual.Model) {
		return false
	}
	return assert.Equal(t, expected.Field, actual.Field)
}

func Test_Repository(t *testing.T) {
	db, err := OpenDatabase(dbConfig)
	if err != nil {
		assert.FailNow(t, err.Error())
	}
	r, err := NewDefaultRepository[Struct](db, "struct")
	if err != nil {
		assert.FailNow(t, err.Error())
	}

	var l []*Struct
	count, err := r.Count(nil)
	assert.NoError(t, err)
	assert.Equal(t, len(l), int(count))
	for i := 0; i < 2; i++ {
		s := &Struct{
			Field: fmt.Sprintf("field_%d", i),
		}
		err = r.Save(s)
		assert.NoError(t, err)
		assert.True(t, s.ID > 0)
		assert.False(t, time.Time{}.Equal(s.CreatedAt))
		assert.True(t, s.CreatedAt.Equal(s.UpdatedAt))

		rs, err := r.FindOne(Struct{
			Field: s.Field,
		})
		assert.NoError(t, err)
		assertEqualStruct(t, *s, *rs)

		rl, err := r.Find(Struct{
			Field: s.Field,
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, len(rl))
		assertEqualStruct(t, *s, rl[0])

		rs, err = r.FindOneWithOrder("field desc", nil)
		assert.NoError(t, err)
		assertEqualStruct(t, *s, *rs)

		l = append(l, s)
	}
	count, err = r.Count(nil)
	assert.NoError(t, err)
	assert.Equal(t, len(l), int(count))

	rl, err := r.Find(nil)
	assert.Equal(t, len(l), len(rl))
	for i, s := range l {
		assertEqualStruct(t, *s, rl[i])
	}

	rl, err = r.FindWithOrder("field desc", nil)
	for i, s := range l {
		assertEqualStruct(t, *s, rl[len(l)-1-i])
	}

	p := Pageable{}
	page, err := r.Page(p, nil)
	assert.NoError(t, err)
	assert.Equal(t, len(l), page.TotalElements)
	assert.Equal(t, len(l), len(page.Content))
	for i, s := range l {
		assertEqualStruct(t, *s, page.Content[i])
	}

	p.Size = 1
	page, err = r.Page(p, nil)
	assert.NoError(t, err)
	assert.Equal(t, len(l), page.TotalElements)
	assert.True(t, int(p.Size) >= len(page.Content))
	offset := int(p.Page * p.Size)
	for i := 0; i < int(p.Size); i++ {
		assertEqualStruct(t, *l[offset+i], page.Content[i])
	}

	p.Sort = "field desc"
	page, err = r.Page(p, nil)
	assert.NoError(t, err)
	assert.Equal(t, len(l), page.TotalElements)
	for i := 0; i < int(p.Size); i++ {
		assertEqualStruct(t, *l[len(l)-1-(offset+i)], page.Content[i])
	}

	for _, s := range l {
		exists, err := r.Exists(s)
		assert.NoError(t, err)
		assert.True(t, exists)

		err = r.Delete(s)
		assert.NoError(t, err)

		exists, err = r.Exists(s)
		assert.NoError(t, err)
		assert.False(t, exists)
	}
	count, err = r.Count(nil)
	assert.NoError(t, err)
	assert.Equal(t, 0, int(count))
}

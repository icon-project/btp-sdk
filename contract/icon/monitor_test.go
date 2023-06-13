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

package icon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func blockMonitor(t *testing.T) *BlockMonitor {
	a := adaptor(t)
	return a.BlockMonitor().(*BlockMonitor)
}

func Test_BlockMonitor(t *testing.T) {
	m := blockMonitor(t)
	h := int64(0)
	ch, err := m.Start(h, false)
	if err != nil {
		assert.FailNow(t, "fail to Start", err)
	}
	for {
		<-time.After(time.Second)
		if m.biFirst != nil {
			t.Logf("biFirst:%s", m.biFirst.String())
			break
		}
	}
	first := <-ch
	t.Logf("first:%s", first)
	<-time.After(time.Second * 10)
	for {
		select {
		case bi := <-ch:
			t.Logf("bi:%s", bi)
		}
	}
}

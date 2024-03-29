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

	"github.com/stretchr/testify/assert"

	"github.com/icon-project/btp-sdk/contract"
)

const (
	subscriptionBufferSize = 1
)

func finalityMonitor(t *testing.T, networkType string) contract.FinalityMonitor {
	a := adaptor(t, networkType)
	return a.FinalityMonitor()
}

func Test_FinalityMonitor(t *testing.T) {
	m := finalityMonitor(t, NetworkTypeIcon)
	s := m.Subscribe(subscriptionBufferSize)
	bi := <-s.C()
	t.Logf("first bi:%s", bi)
	s.Unsubscribe()
	for bi = range s.C() {
		t.Logf("buffered bi:%s", bi)
	}
	_, opened := <-s.C()
	assert.False(t, opened)
}

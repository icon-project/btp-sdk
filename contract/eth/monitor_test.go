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

package eth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func finalityMonitor(t *testing.T, networkType string) *FinalityMonitor {
	a := adaptor(t, networkType)
	return a.FinalityMonitor().(*FinalityMonitor)
}

func Test_FinalityMonitor(t *testing.T) {
	m := finalityMonitor(t, NetworkTypeEth)
	ch, err := m.Start()
	if err != nil {
		assert.FailNow(t, "fail to Start", err)
	}
loop:
	for {
		select {
		case bi := <-ch:
			t.Logf("bi:%s", bi)
			err = m.Stop()
			assert.NoError(t, err)
			break loop
		}
	}
}

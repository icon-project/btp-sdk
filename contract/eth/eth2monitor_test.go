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

	"github.com/icon-project/btp-sdk/contract"
)

func eth2BlockMonitor(t *testing.T) *Eth2BlockMonitor {
	a := adaptor(t, NetworkTypeEth2)
	return a.BlockMonitor().(*Eth2BlockMonitor)
}

func Test_Ether2BlockMonitor(t *testing.T) {
	m := eth2BlockMonitor(t)
	h := int64(0)
	ch, err := m.Start(h, false)
	if err != nil {
		assert.FailNow(t, "fail to Start", err)
	}
	var first contract.BlockInfo
loop:
	for {
		select {
		case bi := <-ch:
			t.Logf("bi:%s", bi)
			if first == nil {
				first = bi
			} else if bi.Status() == contract.BlockStatusFinalized && bi.Height() == first.Height() {
				err = m.Stop()
				assert.NoError(t, err)
				break loop
			}
		}
	}
}
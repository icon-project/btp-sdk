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
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/attestantio/go-eth2-client/spec"
	"github.com/attestantio/go-eth2-client/spec/bellatrix"
	"github.com/attestantio/go-eth2-client/spec/capella"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/icon-project/btp2-eth2/chain/eth2/client"
	"github.com/icon-project/btp2-eth2/chain/eth2/client/lightclient"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
)

const (
	DefaultBlockInfoPoolSize            = 1
	DefaultBlockInfoNotifyRetryInterval = time.Second
)

type Eth2BlockMonitor struct {
	c             *client.ConsensusLayer
	biMap         map[phase0.Slot]*Eth2BlockInfo
	biMapByHeight map[int64]*Eth2BlockInfo
	biFirst       *Eth2BlockInfo
	biLast        *Eth2BlockInfo
	biMtx         sync.RWMutex
	runCancel     context.CancelFunc
	runMtx        sync.RWMutex
	ch            chan contract.BlockInfo
	finalizedOnly bool
	opt           Eth2BlockMonitorOptions
	l             log.Logger
}

type Eth2BlockMonitorOptions struct {
	Endpoint string `json:"endpoint"`
}

func NewEth2BlockMonitor(options contract.Options, l log.Logger) (*Eth2BlockMonitor, error) {
	opt := &Eth2BlockMonitorOptions{}
	if err := contract.DecodeOptions(options, opt); err != nil {
		return nil, err
	}
	c, err := client.NewConsensusLayer(opt.Endpoint, l)
	if err != nil {
		return nil, err
	}
	return &Eth2BlockMonitor{
		c:             c,
		biMap:         make(map[phase0.Slot]*Eth2BlockInfo),
		biMapByHeight: make(map[int64]*Eth2BlockInfo),
		opt:           *opt,
		l:             l,
	}, nil
}

type Eth2BlockInfo struct {
	id     phase0.Hash32
	height int64
	status contract.BlockStatus
	slot   phase0.Slot
	parent phase0.Hash32
}

func (b *Eth2BlockInfo) ID() []byte {
	return b.id[:]
}

func (b *Eth2BlockInfo) Height() int64 {
	return b.height
}

func (b *Eth2BlockInfo) Status() contract.BlockStatus {
	return b.status
}

func (b *Eth2BlockInfo) Slot() uint64 {
	return uint64(b.slot)
}

func (b *Eth2BlockInfo) Parent() []byte {
	return b.parent[:]
}

func (b *Eth2BlockInfo) String() string {
	if b == nil {
		return "nil"
	}
	return fmt.Sprintf("Eth2BlockInfo{ID:%s,Height:%d,Status:%s,Parent:%s,Slot:%d}",
		hex.EncodeToString(b.id[:]), b.height, b.status, hex.EncodeToString(b.parent[:]), b.slot)
}

func (m *Eth2BlockMonitor) NewBlockInfo(s phase0.Slot) (*Eth2BlockInfo, error) {
	blk, err := m.c.RawBeaconBlock(strconv.FormatInt(int64(s), 10))
	if err != nil {
		return nil, err
	}
	bi := &Eth2BlockInfo{}
	switch blk.Version {
	case spec.DataVersionPhase0, spec.DataVersionAltair:
		return nil, errors.Errorf("not support at %s", blk.Version)
	case spec.DataVersionBellatrix:
		e := &bellatrix.ExecutionPayload{}
		if err = json.Unmarshal(blk.Data.Message.Body.ExecutionPayload, e); err != nil {
			return nil, errors.Wrapf(err, "failed to parse %s signed beacon block, err:%s", blk.Version, err.Error())
		}
		copy(bi.id[:], e.BlockHash[:])
		bi.height = int64(e.BlockNumber)
		bi.slot = s
		copy(bi.parent[:], e.ParentHash[:])
	case spec.DataVersionCapella:
		e := &capella.ExecutionPayload{}
		if err = json.Unmarshal(blk.Data.Message.Body.ExecutionPayload, e); err != nil {
			return nil, errors.Wrapf(err, "failed to parse %s signed beacon block, err:%s", blk.Version, err.Error())
		}
		copy(bi.id[:], e.BlockHash[:])
		bi.height = int64(e.BlockNumber)
		bi.slot = s
		copy(bi.parent[:], e.ParentHash[:])
	default:
		//unreachable codes
		return nil, errors.New("unknown version")
	}
	return bi, nil
}

func (m *Eth2BlockMonitor) addBlockInfo(ctx context.Context, bi *Eth2BlockInfo) {
	m.biMtx.Lock()
	defer m.biMtx.Unlock()
	if len(m.biMap) == 0 {
		m.biFirst = bi
	} else {
		if m.biFirst.height > bi.height {
			m.biFirst = bi
		}
	}

	if old, ok := m.biMap[bi.slot]; ok && old.height != bi.height {
		delete(m.biMapByHeight, old.height)
		if old.status == contract.BlockStatusFinalized {
			m.l.Panicf("try to update finalized BlockInfo old:%s new:%s", old.String(), bi.String())
		}
	}
	m.biMap[bi.slot] = bi
	m.biMapByHeight[bi.height] = bi
	m.l.Tracef("add BlockInfo %s", bi.String())
	if !m.finalizedOnly {
		m._notify(ctx, bi)
	}
}

func (m *Eth2BlockMonitor) _notify(ctx context.Context, bi *Eth2BlockInfo) {
	for {
		select {
		case m.ch <- bi:
			m.l.Tracef("_notify success BlockInfo %s", bi.String())
			return
		case <-ctx.Done():
			m.l.Infof("_notify stopped BlockInfo %s", bi.String())
			return
		default:
			m.l.Tracef("_notify failure BlockInfo %s", bi.String())
			<-time.After(DefaultBlockInfoNotifyRetryInterval)
		}
	}
}

func (m *Eth2BlockMonitor) _parent(bi *Eth2BlockInfo) *Eth2BlockInfo {
	for s := bi.slot - 1; s >= m.biFirst.slot; s-- {
		parent, ok := m.biMap[s]
		if !ok {
			m.l.Infof("not exists BlockInfo in _parent slot:%d, try make BlockInfo", s)
			var err error
			if parent, err = m.NewBlockInfo(s); err != nil {
				m.l.Warnf("fail to NewBlockInfo in _parent, slot:%d, err:%+v", s, err)
				//TODO consider empty BeaconBlock
				continue
			}
			m.biMap[s] = parent
		}
		if bytes.Equal(parent.id[:], bi.parent[:]) {
			return parent
		}
	}
	return nil
}

func (m *Eth2BlockMonitor) finalizeBlockInfo(ctx context.Context, s phase0.Slot) {
	m.biMtx.Lock()
	defer m.biMtx.Unlock()

	if m.biFirst.slot > s {
		m.l.Debugf("finalize out of range BlockInfo slot:%d, first:%d", s, m.biFirst.slot)
		return
	}

	bi, ok := m.biMap[s]
	if !ok {
		m.l.Panicf("not exists BlockInfo slot:%d", s)
	}

	last := m.biFirst.slot
	if m.biLast != nil {
		last = m.biLast.slot
	}
	m.biLast = bi
	bis := make([]*Eth2BlockInfo, 0)
	for bi.slot > last {
		parent := m._parent(bi)
		if parent == nil {
			m.l.Panicf("not exists parent BlockInfo %s", bi.String())
		}
		bi.status = contract.BlockStatusFinalized
		bis = append(bis, bi)
		bi = parent
	}
	if m.biFirst.slot == bi.slot {
		bi.status = contract.BlockStatusFinalized
		bis = append(bis, bi)
	}

	for i := len(bis) - 1; i >= 0; i-- {
		m._notify(ctx, bis[i])
	}
}

func (m *Eth2BlockMonitor) BlockInfo(height int64) contract.BlockInfo {
	m.biMtx.RLock()
	defer m.biMtx.RUnlock()
	if bi, ok := m.biMapByHeight[height]; ok {
		return bi
	}
	return nil
}

func (m *Eth2BlockMonitor) Start(height int64, finalizedOnly bool) (<-chan contract.BlockInfo, error) {
	m.runMtx.Lock()
	defer m.runMtx.Unlock()
	if m.runCancel != nil {
		return nil, errors.Errorf("already started")
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.runCancel = cancel
	m.ch = make(chan contract.BlockInfo, DefaultBlockInfoPoolSize)
	m.finalizedOnly = finalizedOnly
	m.c.LightClientEventsWithContext(ctx, func(update *lightclient.LightClientOptimisticUpdate) {
		if height > 0 && m.BlockInfo(height) == nil {
			//catchup
			bis := make([]*Eth2BlockInfo, 0)
			for s, biHeight := update.AttestedHeader.Beacon.Slot, int64(math.MaxInt64); height < biHeight; s-- {
				bi, err := m.NewBlockInfo(s)
				if err != nil {
					m.l.Infof("fail to create BlockInfo err:%+v", err)
					continue
				}
				bi.status = contract.BlockStatusProposed
				bis = append(bis, bi)
				biHeight = bi.height
			}
			for i := len(bis) - 1; i >= 0; i-- {
				m.addBlockInfo(ctx, bis[i])
			}
		} else {
			bi, err := m.NewBlockInfo(update.AttestedHeader.Beacon.Slot)
			if err != nil {
				m.l.Infof("fail to create BlockInfo err:%+v", err)
			} else {
				bi.status = contract.BlockStatusProposed
				m.addBlockInfo(ctx, bi)
			}
		}
	}, func(update *lightclient.LightClientFinalityUpdate) {
		m.finalizeBlockInfo(ctx, update.FinalizedHeader.Beacon.Slot)
	})
	return m.ch, nil
}

func (m *Eth2BlockMonitor) Stop() error {
	m.runMtx.Lock()
	defer m.runMtx.Unlock()
	if m.runCancel == nil {
		return errors.Errorf("already stopped")
	}
	m.l.Debugf("stop Eth2BlockMonitor")
	m.runCancel()
	m.ch = nil
	return nil
}

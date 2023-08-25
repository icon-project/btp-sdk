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
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
)

const (
	DefaultBlockInfoPoolSize            = 1
	DefaultMonitorRetryInterval         = time.Second
	DefaultBlockInfoNotifyRetryInterval = time.Second
)

var (
	rpcFinalizedBlockNumber = big.NewInt(int64(rpc.FinalizedBlockNumber))
)

type BlockInfo struct {
	id     common.Hash
	height int64
}

func (b *BlockInfo) ID() contract.BlockID {
	return NewBlockID(b.id)
}

func (b *BlockInfo) Height() int64 {
	return b.height
}

func (b *BlockInfo) String() string {
	return fmt.Sprintf("BlockInfo{ID:%s,Height:%d}",
		hex.EncodeToString(b.id[:]), b.height)
}

type BlockInfoJson struct {
	ID     common.Hash
	Height int64
}

func (b *BlockInfo) UnmarshalJSON(bytes []byte) error {
	v := &BlockInfoJson{}
	if err := json.Unmarshal(bytes, v); err != nil {
		return err
	}
	b.id = v.ID
	b.height = v.Height
	return nil
}

func (b *BlockInfo) MarshalJSON() ([]byte, error) {
	v := BlockInfoJson{
		ID:     b.id,
		Height: b.height,
	}
	return json.Marshal(v)
}

type FinalityMonitor struct {
	*ethclient.Client
	runCancel context.CancelFunc
	runMtx    sync.RWMutex
	bi        *BlockInfo
	biMtx     sync.RWMutex
	ch        chan contract.BlockInfo
	opt       FinalityMonitorOptions
	l         log.Logger
}

type FinalityMonitorOptions struct {
	PollingPeriodSec uint `json:"polling_period_sec"`
}

func NewFinalityMonitor(options contract.Options, c *ethclient.Client, l log.Logger) (*FinalityMonitor, error) {
	opt := &FinalityMonitorOptions{}
	if err := contract.DecodeOptions(options, opt); err != nil {
		return nil, err
	}
	return &FinalityMonitor{
		Client: c,
		opt:    *opt,
		l:      l,
	}, nil
}

func (m *FinalityMonitor) setLast(bi *BlockInfo) {
	m.biMtx.Lock()
	defer m.biMtx.Unlock()
	m.bi = bi
}

func (m *FinalityMonitor) getLast() *BlockInfo {
	m.biMtx.RLock()
	defer m.biMtx.RUnlock()
	return m.bi
}

func (m *FinalityMonitor) notify(ctx context.Context) {
	bi := &BlockInfo{
		height: 0,
	}
	for {
		last := m.getLast()
		if last == nil || bi.height >= last.height {
			<-time.After(DefaultBlockInfoNotifyRetryInterval)
			continue
		}
		bi = last
		select {
		case m.ch <- bi:
			m.l.Tracef("notify success BlockInfo %s", bi.String())
		case <-ctx.Done():
			m.l.Infof("notify stopped BlockInfo %s", bi.String())
			return
		default:
			m.l.Tracef("notify failure BlockInfo %s", bi.String())
			<-time.After(DefaultBlockInfoNotifyRetryInterval)
		}
	}
}

func (m *FinalityMonitor) monitor(ctx context.Context) {
	for {
		select {
		case <-time.After(DefaultMonitorRetryInterval):
			m.l.Debugln("try to MonitorBlock last:%v", m.getLast())
			if err := m.pollFinalizedBlock(ctx, func(h *types.Header) error {
				m.setLast(&BlockInfo{
					id:     h.Hash(),
					height: h.Number.Int64(),
				})
				return nil
			}); err != nil {
				m.l.Debugf("fail to MonitorBlock err:%+v", err)
			}
			m.l.Debugln("MonitorBlock stopped")
		case <-ctx.Done():
			m.l.Debugln("MonitorBlock context done")
			return
		}
	}
}

func (m *FinalityMonitor) LatestFinalizedBlock(ctx context.Context) (*types.Header, error) {
	return m.HeaderByNumber(ctx, rpcFinalizedBlockNumber)
}

func (m *FinalityMonitor) pollFinalizedBlock(ctx context.Context, cb func(h *types.Header) error) error {
	var current *big.Int

	period := time.Duration(m.opt.PollingPeriodSec) * time.Second
	for {
		select {
		case <-ctx.Done():
			m.l.Debugf("pollFinalizedBlock context done")
			return ctx.Err()
		default:
		}
		bh, err := m.LatestFinalizedBlock(ctx)
		if err != nil {
			if ctx.Err() == context.Canceled {
				m.l.Trace("pollFinalizedBlock context done ", current)
			} else {
				m.l.Warn("Unable to get finalized block ", current, err)
			}
			<-time.After(period)
			continue
		}
		if current == nil || current.Cmp(bh.Number) < 0 {
			if current == nil {
				m.l.Debugf("pollFinalizedBlock height:%v", bh.Number)
			}
			if err = cb(bh); err != nil {
				m.l.Errorf("pollFinalizedBlock height:%v callback return err:%+v", current, err)
				return err
			}
			current = bh.Number
		}
		<-time.After(period)
	}
}

func (m *FinalityMonitor) IsFinalized(height int64, id contract.BlockID) (bool, error) {
	if height < 1 {
		return false, errors.Errorf("invalid height:%d", height)
	}

	h, err := CommonHashOf(id)
	if err != nil {
		return false, errors.Wrapf(err, "invalid BlockID err:%v", err.Error())
	}
	last := m.getLast()
	if last == nil {
		var bh *types.Header
		if bh, err = m.LatestFinalizedBlock(context.Background()); err != nil {
			return false, err
		}
		last = &BlockInfo{
			id:     bh.Hash(),
			height: bh.Number.Int64(),
		}
	}
	if height > last.height {
		return false, nil
	}
	var th common.Hash
	if height == last.height {
		th = last.id
	} else {
		var bh *types.Header
		if bh, err = m.Client.HeaderByNumber(context.Background(), big.NewInt(height)); err != nil {
			return false, err
		}
		th = bh.Hash()
	}
	if !bytes.Equal(h[:], th[:]) {
		return false, contract.ErrMismatchBlockID
	}
	return true, nil
}

func (m *FinalityMonitor) HeightByID(id contract.BlockID) (int64, error) {
	h, err := CommonHashOf(id)
	if err != nil {
		return 0, errors.Wrapf(err, "invalid BlockID err:%v", err.Error())
	}
	bh, err := m.Client.HeaderByHash(context.Background(), h)
	if err != nil {
		return 0, err
	}
	return bh.Number.Int64(), nil
}

func (m *FinalityMonitor) Start() (<-chan contract.BlockInfo, error) {
	m.runMtx.Lock()
	defer m.runMtx.Unlock()
	if m.runCancel != nil {
		return nil, errors.Errorf("already started")
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.runCancel = cancel
	m.ch = make(chan contract.BlockInfo, DefaultBlockInfoPoolSize)
	go m.notify(ctx)
	go m.monitor(ctx)
	return m.ch, nil
}

func (m *FinalityMonitor) Stop() error {
	m.runMtx.Lock()
	defer m.runMtx.Unlock()
	if m.runCancel == nil {
		return errors.Errorf("already stopped")
	}
	m.runCancel()
	m.runCancel = nil
	m.ch = nil
	return nil
}

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
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
)

const (
	DefaultPollHeadInterval = 2 * time.Second
)

type BlockInfo struct {
	id     []byte
	height int64
	status contract.BlockStatus
	parent []byte
}

func (b *BlockInfo) ID() []byte {
	return b.id[:]
}

func (b *BlockInfo) Height() int64 {
	return b.height
}

func (b *BlockInfo) Status() contract.BlockStatus {
	return b.status
}

func (b *BlockInfo) Parent() []byte {
	return b.parent
}

func (b *BlockInfo) String() string {
	return fmt.Sprintf("BlockInfo{ID:%s,Height:%d,Status:%s,Parent:%s}",
		hex.EncodeToString(b.id[:]), b.height, b.status, hex.EncodeToString(b.parent[:]))
}

type BlockMonitor struct {
	*ethclient.Client
	biMap         map[int64]*BlockInfo
	biFirst       *BlockInfo
	biLast        *BlockInfo
	biMtx         sync.RWMutex
	runCancel     context.CancelFunc
	runMtx        sync.RWMutex
	ch            chan contract.BlockInfo
	finalizedOnly bool
	opt           BlockMonitorOptions
	l             log.Logger
}

type BlockMonitorOptions struct {
	FinalizeBlockCount uint
}

func NewBlockMonitor(options contract.Options, c *ethclient.Client, l log.Logger) (*BlockMonitor, error) {
	opt := &BlockMonitorOptions{}
	if err := contract.DecodeOptions(options, opt); err != nil {
		return nil, err
	}
	return &BlockMonitor{
		Client: c,
		biMap:  make(map[int64]*BlockInfo),
		opt:    *opt,
		l:      l,
	}, nil
}

func (m *BlockMonitor) addBlockInfo(ctx context.Context, bi *BlockInfo) {
	m.biMtx.Lock()
	defer m.biMtx.Unlock()

	if len(m.biMap) == 0 {
		m.biFirst = bi
	} else {
		if m.biFirst.height > bi.height {
			m.biFirst = bi
		}
	}

	m.biMap[bi.height] = bi
	m.l.Tracef("add BlockInfo %s", bi.String())
	if !m.finalizedOnly {
		m._notify(ctx, bi)
	}
}

func (m *BlockMonitor) finalizeBlockInfo(ctx context.Context, height int64) {
	m.biMtx.Lock()
	defer m.biMtx.Unlock()

	if m.biFirst.height > height {
		m.l.Debugf("finalize out of range BlockInfo height:%d, first:%d", height, m.biFirst.height)
		return
	}

	bi, ok := m.biMap[height]
	if !ok {
		m.l.Panicf("not exists BlockInfo height:%d", height)
	}

	if parent, ok := m.biMap[height-1]; m.biFirst.height == bi.height || (ok && bytes.Equal(parent.id, bi.parent)) {
		bi.status = contract.BlockStatusFinalized
		m.biLast = bi
		m._notify(ctx, bi)
	} else {
		m.l.Panicf("not exists parent BlockInfo %s", bi.String())
	}
}

func (m *BlockMonitor) _notify(ctx context.Context, bi *BlockInfo) {
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

func (m *BlockMonitor) BlockInfo(height int64) contract.BlockInfo {
	m.biMtx.RLock()
	defer m.biMtx.RUnlock()
	return m.biMap[height]
}

func (m *BlockMonitor) Start(height int64, finalizedOnly bool) (<-chan contract.BlockInfo, error) {
	m.runMtx.Lock()
	defer m.runMtx.Unlock()
	if m.runCancel != nil {
		return nil, errors.Errorf("already started")
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.runCancel = cancel
	m.ch = make(chan contract.BlockInfo, DefaultBlockInfoPoolSize)
	m.finalizedOnly = finalizedOnly
	bhHandleFunc := func(bh *types.Header) {
		bi := &BlockInfo{
			id:     bh.Hash().Bytes(),
			height: bh.Number.Int64(),
			status: contract.BlockStatusProposed,
			parent: bh.ParentHash.Bytes(),
		}
		m.addBlockInfo(ctx, bi)
		m.finalizeBlockInfo(ctx, bi.height-int64(m.opt.FinalizeBlockCount))
	}
	go func() {
		var h *big.Int
		for {
			select {
			case <-time.After(time.Second):
				m.l.Debugln("try to MonitorBlock")
				onBlockHeader := func(bh *types.Header) error {
					if h == nil && height > 0 {
						h = big.NewInt(height)
						for ; h.Cmp(bh.Number) < 0; h = h.Add(h, common.Big1) {
							m.l.Debugf("catchup Client.HeaderByNumber %v", h.Int64())
							if tbh, err := m.Client.HeaderByNumber(context.Background(), h); err != nil {
								m.l.Errorf("fail to HeaderByNumber(%v) err:%+v", h, err)
								return err
							} else {
								bhHandleFunc(tbh)
							}
						}
					}
					bhHandleFunc(bh)
					return nil
				}
				err := m.MonitorBySubscribeNewHead(ctx, onBlockHeader)
				if err != nil {
					if err == rpc.ErrNotificationsUnsupported {
						m.l.Debugf("fail to MonitorBySubscribeNewHead, try MonitorByPollHead")
						err = monitorByPollHead(m.Client, m.l, ctx, onBlockHeader)
					}
					m.l.Debugf("fail to MonitorBlock err:%+v", err)
				}
				m.l.Debugln("MonitorBlock stopped")
			case <-ctx.Done():
				m.l.Debugln("MonitorBlock context done")
				return
			}
		}
	}()
	return m.ch, nil
}

func (m *BlockMonitor) Stop() error {
	m.runMtx.Lock()
	defer m.runMtx.Unlock()
	if m.runCancel == nil {
		return errors.Errorf("already stopped")
	}
	m.runCancel()
	m.ch = nil
	return nil
}

func (m *BlockMonitor) MonitorBySubscribeNewHead(ctx context.Context, cb func(bh *types.Header) error) error {
	ch := make(chan *types.Header)
	s, err := m.Client.SubscribeNewHead(ctx, ch)
	if err != nil {
		return err
	}
	for {
		select {
		case err = <-s.Err():
			return err
		case bh := <-ch:
			if err = cb(bh); err != nil {
				return err
			}
		}
	}
}

func (m *BlockMonitor) MonitorByPollHead(ctx context.Context, cb func(bh *types.Header) error) error {
	n, err := m.Client.BlockNumber(ctx)
	if err != nil {
		return err
	}
	current := new(big.Int).SetUint64(n)
	for {
		select {
		case <-ctx.Done():
			m.l.Debugf("MonitorByPollHead context done")
			return ctx.Err()
		default:
		}
		var bh *types.Header
		if bh, err = m.Client.HeaderByNumber(ctx, current); err != nil {
			if ethereum.NotFound == err {
				m.l.Trace("Block not ready, will retry ", current)
			} else {
				m.l.Warn("Unable to get block ", current, err)
			}
			<-time.After(DefaultPollHeadInterval)
			continue
		}

		if err = cb(bh); err != nil {
			m.l.Errorf("Poll callback return err:%+v", err)
			return err
		}
		current.Add(current, big.NewInt(1))
	}
}

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
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/icon-project/btp2/chain/icon/client"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
)

const (
	DefaultBlockInfoPoolSize            = 1
	DefaultBlockInfoNotifyRetryInterval = time.Second
)

type BlockInfo struct {
	id     []byte
	height int64
}

func (b *BlockInfo) ID() []byte {
	return b.id[:]
}

func (b *BlockInfo) Height() int64 {
	return b.height
}

func (b *BlockInfo) Status() contract.BlockStatus {
	return contract.BlockStatusFinalized
}

func (b *BlockInfo) String() string {
	return fmt.Sprintf("BlockInfo{ID:%s,Height:%d,Status:%s}",
		hex.EncodeToString(b.id[:]), b.height, b.Status())
}

type BlockMonitor struct {
	*client.Client
	biMap     map[int64]*BlockInfo
	biFirst   *BlockInfo
	biLast    *BlockInfo
	biMtx     sync.RWMutex
	runCancel context.CancelFunc
	runMtx    sync.RWMutex
	ch        chan contract.BlockInfo
	l         log.Logger
}

func NewBlockMonitor(c *client.Client, l log.Logger) *BlockMonitor {
	return &BlockMonitor{
		Client: c,
		biMap:  make(map[int64]*BlockInfo),
		l:      l,
	}
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
	m._notify(ctx, bi)
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
	go func() {
		for {
			select {
			case <-time.After(time.Second):
				m.l.Debugln("try to MonitorBlock")
				p := &client.BlockRequest{
					Height: client.NewHexInt(height),
				}
				if err := m.Client.MonitorBlock(p, func(conn *websocket.Conn, v *client.BlockNotification) error {
					id, err := v.Hash.Value()
					if err != nil {
						return err
					}
					h, err := v.Height.Value()
					if err != nil {
						return err
					}
					height = h
					m.addBlockInfo(ctx, &BlockInfo{
						id:     id,
						height: h,
					})
					return nil
				}, func(conn *websocket.Conn) {
					m.l.Debugln("MonitorBlock connected")
				}, func(conn *websocket.Conn, err error) {
					m.l.Debugf("fail to MonitorBlock err:%+v", err)
				}); err != nil {
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

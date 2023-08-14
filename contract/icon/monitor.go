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
	"bytes"
	"context"
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
	DefaultMonitorRetryInterval         = time.Second
	DefaultBlockInfoNotifyRetryInterval = time.Second
)

type BlockInfo struct {
	id     HexBytes
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
		b.id.String(), b.height)
}

type FinalityMonitor struct {
	*client.Client
	runCancel context.CancelFunc
	runMtx    sync.RWMutex
	bi        *BlockInfo
	biMtx     sync.RWMutex
	ch        chan contract.BlockInfo
	l         log.Logger
}

func NewFinalityMonitor(c *client.Client, l log.Logger) *FinalityMonitor {
	return &FinalityMonitor{
		Client: c,
		l:      l,
	}
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
			last := m.getLast()
			if last == nil {
				blk, err := m.Client.GetLastBlock()
				if err != nil {
					m.l.Debugf("fail to MonitorBlock GetLastBlock err:%+v", err)
					continue
				}
				id, _ := blk.BlockHash.Value()
				last = &BlockInfo{
					id:     id,
					height: blk.Height,
				}
				m.setLast(last)
			}
			m.l.Debugln("try to MonitorBlock last:%v", last)
			p := &client.BlockRequest{
				Height: client.NewHexInt(last.height),
			}
			if err := m.Client.MonitorBlock(p, func(conn *websocket.Conn, v *client.BlockNotification) error {
				id, err := v.Hash.Value()
				if err != nil {
					return err
				}
				height, err := v.Height.Value()
				if err != nil {
					return err
				}
				m.setLast(&BlockInfo{
					id:     id,
					height: height,
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
}

func (m *FinalityMonitor) IsFinalized(height int64, id contract.BlockID) (bool, error) {
	if height < 1 {
		return false, errors.Errorf("invalid height:%d", height)
	}
	hb, err := HexBytesOf(id)
	if err != nil {
		return false, errors.Wrapf(err, "invalid BlockID err:%v", err.Error())
	}
	last := m.getLast()
	if last == nil {
		return false, errors.Errorf("not started")
	}
	if height > last.height {
		return false, nil
	}
	var thb HexBytes
	if height == last.height {
		thb = last.id
	} else {
		p := &client.BlockHeightParam{
			Height: client.NewHexInt(height),
		}
		var blk *client.Block
		if blk, err = m.Client.GetBlockByHeight(p); err != nil {
			return false, err
		}

		if thb, err = HexBytesOf(blk.BlockHash); err != nil {
			return false, err
		}
	}
	if !bytes.Equal(hb, thb) {
		return false, contract.ErrMismatchBlockID
	}
	return true, nil
}

func (m *FinalityMonitor) BlockInfo(height int64) contract.BlockInfo {
	m.biMtx.RLock()
	defer m.biMtx.RUnlock()
	return nil
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
	m.setLast(nil)
	return nil
}

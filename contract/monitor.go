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

package contract

import (
	"context"
	"sync"
	"time"

	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"
)

const (
	DefaultFinalityMonitorRetryIntervalSec = 1
)

type DefaultFinalitySubscription struct {
	CH              <-chan BlockInfo
	UnsubscribeFunc func()
}

func (s *DefaultFinalitySubscription) C() <-chan BlockInfo {
	return s.CH
}

func (s *DefaultFinalitySubscription) Unsubscribe() {
	s.UnsubscribeFunc()
}

func (s *DefaultFinalitySubscription) Serve(ctx context.Context, cb func(info BlockInfo)) {
	for {
		select {
		case v, ok := <-s.CH:
			if !ok {
				return
			}
			cb(v)
		case <-ctx.Done():
			return
		}
	}
}

type DefaultFinalityMonitorOptions struct {
	RetryIntervalSec uint `json:"retry_interval_sec"`
}

type DefaultFinalityMonitor struct {
	FinalitySupplier
	m    map[*DefaultFinalitySubscription]chan BlockInfo
	last BlockInfo

	ctx    context.Context
	cancel context.CancelFunc
	mtx    sync.RWMutex
	opt    DefaultFinalityMonitorOptions
	l      log.Logger
}

func NewDefaultFinalityMonitor(options Options, fs FinalitySupplier, l log.Logger) (*DefaultFinalityMonitor, error) {
	opt := DefaultFinalityMonitorOptions{}
	if err := DecodeOptions(options, &opt); err != nil {
		return nil, err
	}
	if opt.RetryIntervalSec == 0 {
		opt.RetryIntervalSec = DefaultFinalityMonitorRetryIntervalSec
	}
	return &DefaultFinalityMonitor{
		FinalitySupplier: fs,
		m:                make(map[*DefaultFinalitySubscription]chan BlockInfo),
		opt:              opt,
		l:                l,
	}, nil
}

func (m *DefaultFinalityMonitor) _remove(k *DefaultFinalitySubscription) {
	if ch, ok := m.m[k]; ok {
		close(ch)
		delete(m.m, k)
	}
}

func (m *DefaultFinalityMonitor) notify(v BlockInfo) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.last = v
	for k, ch := range m.m {
		select {
		case ch <- v:
		default:
			m._remove(k)
		}
	}
}

func (m *DefaultFinalityMonitor) Subscriptions() int {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	return len(m.m)
}

func (m *DefaultFinalityMonitor) _isStarted() bool {
	return m.cancel != nil
}

func (m *DefaultFinalityMonitor) IsStarted() bool {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	return m._isStarted()
}

func (m *DefaultFinalityMonitor) _start() {
	if m._isStarted() {
		return
	}
	retryInterval := time.Duration(m.opt.RetryIntervalSec) * time.Second
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx = ctx
	m.cancel = cancel
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if err := m.FinalitySupplier.Serve(ctx, m.getLast(), func(bi BlockInfo) {
				m.notify(bi)
			}); err != nil {
				m.l.Debugf("fail to Serve err:%+v", err)
			}
			<-time.After(retryInterval)
		}
	}()
}

func (m *DefaultFinalityMonitor) Start() {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m._start()
}

func (m *DefaultFinalityMonitor) _stop() {
	if !m._isStarted() {
		return
	}
	m.cancel()
	m.cancel = nil

	for k := range m.m {
		m._remove(k)
	}
}

func (m *DefaultFinalityMonitor) Stop() {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m._stop()
}

func (m *DefaultFinalityMonitor) getLast() BlockInfo {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	return m.last
}

func (m *DefaultFinalityMonitor) Last() (BlockInfo, error) {
	if m.IsStarted() {
		return m.getLast(), nil
	} else {
		return m.FinalitySupplier.Latest()
	}
}

func (m *DefaultFinalityMonitor) IsFinalized(height int64, id BlockID) (bool, error) {
	if height < 1 {
		return false, errors.Errorf("invalid height:%d", height)
	}
	last, err := m.Last()
	if err != nil {
		return false, err
	}
	if height > last.Height() {
		return false, nil
	} else if height == last.Height() {
		return last.EqualID(id)
	} else {
		ret, err := m.FinalitySupplier.HeightByID(id)
		if err != nil {
			return false, err
		}
		return height == ret, nil
	}
}

func (m *DefaultFinalityMonitor) Subscribe(size uint) FinalitySubscription {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m._start()

	ch := make(chan BlockInfo, size)
	s := &DefaultFinalitySubscription{
		CH: ch,
	}
	s.UnsubscribeFunc = func() {
		m.mtx.Lock()
		defer m.mtx.Unlock()
		m._remove(s)
		if len(m.m) == 0 {
			m._stop()
		}
	}
	m.m[s] = ch
	return s
}

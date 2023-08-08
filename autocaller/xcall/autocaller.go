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

package xcall

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"
	"github.com/icon-project/btp2/common/types"

	"github.com/icon-project/btp-sdk/autocaller"
	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/service"
	"github.com/icon-project/btp-sdk/service/xcall"
)

const (
	EventCallMessageSent     = "CallMessageSent"
	EventCallMessage         = "CallMessage"
	EventRollbackMessage     = "RollbackMessage"
	MethodExecuteCall        = "executeCall"
	MethodExecuteRollback    = "executeRollback"
	ReasonInvalidRequestId   = "InvalidRequestId"
	ReasonInvalidSerialNum   = "InvalidSerialNum"
	ReasonRollbackNotEnabled = "RollbackNotEnabled"
	TaskCall                 = "call"
	TaskRollback             = "rollback"
)

func init() {
	autocaller.RegisterFactory(xcall.ServiceName, NewAutoCaller)
}

type AutoCaller struct {
	s         service.Service
	nMap      map[string]Network
	runCancel context.CancelFunc
	runMtx    sync.RWMutex
	l         log.Logger

	//FIXME use db instead
	monitorHeightMap   map[string]int64                                //network:height
	callMessageMap     map[string]map[contract.Integer]CallMessage     //network:reqId:CallMessage
	rollbackMessageMap map[string]map[contract.Integer]RollbackMessage //network:sn:RollbackMessage
}

type Network struct {
	NetworkType  string
	Adaptor      contract.Adaptor
	Signer       service.Signer
	Options      AutoCallerOptions
	EventFilters []contract.EventFilter
}

type AutoCallerOptions struct {
	InitHeight     int64              `json:"init_height"`
	NetworkAddress string             `json:"network_address"`
	Contracts      []contract.Address `json:"contracts"`
}

func NewAutoCaller(s service.Service, networks map[string]autocaller.Network, l log.Logger) (autocaller.AutoCaller, error) {
	if s.Name() != xcall.ServiceName {
		return nil, errors.Errorf("invalid service name:%s", s.Name())
	}

	nMap := make(map[string]Network)
	signers := make(map[string]service.Signer)
	for network, n := range networks {
		opt := &AutoCallerOptions{}
		if err := contract.DecodeOptions(n.Options, &opt); err != nil {
			return nil, err
		}
		nMap[network] = Network{
			NetworkType: n.NetworkType,
			Adaptor:     n.Adaptor,
			Signer:      n.Signer,
			Options:     *opt,
		}
		signers[network] = n.Signer
	}
	for fromNetwork, fn := range nMap {
		cmsParams := make([]contract.Params, 0)
		cmParams := make([]contract.Params, 0)
		for _, fa := range fn.Options.Contracts {
			from := types.BtpAddress(fmt.Sprintf("btp://%s/%s", fn.Options.NetworkAddress, fa))
			for toNetwork, tn := range nMap {
				if toNetwork == fromNetwork {
					continue
				}
				for _, ta := range tn.Options.Contracts {
					to := types.BtpAddress(fmt.Sprintf("btp://%s/%s", tn.Options.NetworkAddress, ta))
					cmsParams = append(cmsParams, contract.Params{
						"_from": contract.Address(from.ContractAddress()),
						"_to":   to.String(),
					})
					cmParams = append(cmParams, contract.Params{
						"_from": to.String(),
						"_to":   from.ContractAddress(),
					})
				}
			}
		}
		nameToParams := map[string][]contract.Params{
			EventCallMessageSent: cmsParams,
			EventCallMessage:     cmParams,
			EventRollbackMessage: nil,
		}
		l.Debugf("fromNetwork:%s nameToParams:%v", fromNetwork, nameToParams)
		efs, err := s.EventFilters(fromNetwork, nameToParams)
		if err != nil {
			return nil, err
		}
		fn.EventFilters = efs[:]
		nMap[fromNetwork] = fn
	}
	ss, err := service.NewSignerService(s, signers, l)
	if err != nil {
		return nil, err
	}
	return &AutoCaller{
		s:    ss,
		nMap: nMap,
		l:    l,
		//
		monitorHeightMap:   make(map[string]int64),
		callMessageMap:     make(map[string]map[contract.Integer]CallMessage),
		rollbackMessageMap: make(map[string]map[contract.Integer]RollbackMessage),
	}, nil
}

func (c *AutoCaller) Name() string {
	return c.s.Name()
}

func (c *AutoCaller) Tasks() []string {
	return []string{TaskCall, TaskRollback}
}

func (c *AutoCaller) Start() error {
	c.runMtx.Lock()
	defer c.runMtx.Unlock()
	if c.runCancel != nil {
		return errors.Errorf("already started")
	}
	ctx, cancel := context.WithCancel(context.Background())
	for network, n := range c.nMap {
		height, ok, err := c.getMonitorHeight(network)
		if err != nil {
			cancel()
			return err
		}
		if !ok {
			if err = c.setMonitorHeight(network, n.Options.InitHeight); err != nil {
				cancel()
				return err
			}
			height = n.Options.InitHeight
		}
		go c.monitorEvent(ctx, network, n.EventFilters, height)
	}
	c.runCancel = cancel
	return nil
}

func (c *AutoCaller) Stop() error {
	c.runMtx.Lock()
	defer c.runMtx.Unlock()
	if c.runCancel == nil {
		return errors.Errorf("already stopped")
	}
	c.runCancel()
	c.runCancel = nil
	return nil
}

func (c *AutoCaller) Find(fp autocaller.FindParam) ([]interface{}, error) {
	ret := make([]interface{}, 0)
	switch fp.Task {
	case TaskCall:
		for _, m := range c.callMessageMap {
			for _, msg := range m {
				ret = append(ret, msg)
			}
		}
	case TaskRollback:
		for _, m := range c.rollbackMessageMap {
			for _, msg := range m {
				ret = append(ret, msg)
			}
		}
	case "":
		for _, m := range c.callMessageMap {
			for _, msg := range m {
				ret = append(ret, msg)
			}
		}
		for _, m := range c.rollbackMessageMap {
			for _, msg := range m {
				ret = append(ret, msg)
			}
		}
	default:
		return nil, errors.Errorf("not support action:%s", fp.Task)
	}
	return ret, nil
}

func (c *AutoCaller) monitorEvent(ctx context.Context, network string, efs []contract.EventFilter, height int64) {
	c.l.Debugf("monitorEvent network:%s height:%v", network, height)
	for {
		select {
		case <-time.After(time.Second):
			if err := c.s.MonitorEvent(ctx, network, func(e contract.Event) error {
				c.l.Tracef("monitorEvent callback network:%s event:%s height:%v", network, e.Name(), e.BlockHeight())
				if height < e.BlockHeight() {
					height = e.BlockHeight()
					if err := c.setMonitorHeight(network, height); err != nil {
						return err
					}
				}
				switch e.Name() {
				case EventCallMessage:
					return c.onCallMessage(network, e)
				case EventRollbackMessage:
					return c.onRollbackMessage(network, e)
				}
				return nil
			}, efs, height); err != nil {
				c.l.Debugf("MonitorEvent stopped network:%s err:%v", network, err)
			}
		case <-ctx.Done():
			c.l.Debugln("MonitorEvent context done")
			return
		}
	}
}

func (c *AutoCaller) getMonitorHeight(network string) (int64, bool, error) {
	height, ok := c.monitorHeightMap[network]
	if !ok {
		return -1, false, nil
	}
	return height, true, nil
}

func (c *AutoCaller) setMonitorHeight(network string, height int64) error {
	c.monitorHeightMap[network] = height
	return nil
}

func (c *AutoCaller) onCallMessage(network string, e contract.Event) error {
	cm, err := NewCallMessage(network, e)
	if err != nil {
		return err
	}
	old, ok, err := c.getCallMessage(network, cm.ReqId)
	if err != nil {
		return err
	}
	if ok && old.Sent {
		//skip
		c.l.Debugf("skip old CallMessage:{Network:%s,Sn:%v,ReqID:%v,BlockHeight:%d}",
			cm.Network, cm.Sn, cm.ReqId, cm.BlockHeight())
		return nil
	}
	txID, err := c.s.Invoke(network, MethodExecuteCall, contract.Params{
		"_reqId": cm.ReqId,
		"_data":  cm.Data,
	}, contract.Options{
		"Estimate": true,
	})
	if err != nil {
		if ee, ok := err.(contract.EstimateError); ok && ee.Reason() == ReasonInvalidRequestId {
			cm.Sent = true
			c.l.Debugf("skip %s CallMessage:{Network:%s,Sn:%v,ReqID:%v,BlockHeight:%d}",
				ee.Reason(), cm.Network, cm.Sn, cm.ReqId, cm.BlockHeight())
			return c.saveCallMessage(network, cm)
		}
		return err
	}
	cm.SentTxID = txID
	cm.Sent = true
	c.l.Debugf("CallMessage:{Network:%s,Sn:%v,ReqID:%v,SentTxID:%v}", cm.Network, cm.Sn, cm.ReqId, cm.SentTxID)
	return c.saveCallMessage(network, cm)
}

func (c *AutoCaller) getCallMessage(network string, reqID contract.Integer) (cm CallMessage, ok bool, err error) {
	var m map[contract.Integer]CallMessage
	if m, ok = c.callMessageMap[network]; !ok {
		return
	}
	if cm, ok = m[reqID]; !ok {
		return
	}
	return
}

func (c *AutoCaller) saveCallMessage(network string, cm CallMessage) error {
	m, ok := c.callMessageMap[network]
	if !ok {
		m = make(map[contract.Integer]CallMessage)
		c.callMessageMap[network] = m
	}
	m[cm.ReqId] = cm
	return nil
}

func (c *AutoCaller) onRollbackMessage(network string, e contract.Event) error {
	rm, err := NewRollbackMessage(network, e)
	if err != nil {
		return err
	}
	old, ok, err := c.getRollbackMessage(network, rm.Sn)
	if err != nil {
		return err
	}
	if ok && old.Sent {
		//skip
		c.l.Debugf("skip old RollbackMessage:{Network:%s,Sn:%v,BlockHeight:%d}",
			rm.Network, rm.Sn, rm.BlockHeight())
		return nil
	}
	txID, err := c.s.Invoke(network, MethodExecuteRollback, contract.Params{
		"_sn": rm.Sn,
	}, nil)
	if err != nil {
		if ee, ok := err.(contract.EstimateError); ok &&
			(ee.Reason() == ReasonInvalidSerialNum || ee.Reason() == ReasonRollbackNotEnabled) {
			rm.Sent = true
			c.l.Debugf("skip %s RollbackMessage:{Network:%s,Sn:%v,BlockHeight:%d}",
				ee.Reason(), rm.Network, rm.Sn, rm.BlockHeight())
			return c.saveRollbackMessage(network, rm)
		}
		return err
	}
	rm.SentTxID = txID
	rm.Sent = true
	c.l.Debugf("RollbackMessage:{Network:%s,Sn:%v,SentTxID:%v}", rm.Network, rm.Sn, rm.SentTxID)
	return c.saveRollbackMessage(network, rm)
}

func (c *AutoCaller) getRollbackMessage(network string, sn contract.Integer) (rm RollbackMessage, ok bool, err error) {
	var m map[contract.Integer]RollbackMessage
	if m, ok = c.rollbackMessageMap[network]; !ok {
		return
	}
	if rm, ok = m[sn]; !ok {
		return
	}
	return
}

func (c *AutoCaller) saveRollbackMessage(network string, rm RollbackMessage) error {
	m, ok := c.rollbackMessageMap[network]
	if !ok {
		m = make(map[contract.Integer]RollbackMessage)
		c.rollbackMessageMap[network] = m
	}
	m[rm.Sn] = rm
	return nil
}

type CallMessage struct {
	contract.Event
	Network  string
	SentTxID contract.TxID
	Sent     bool

	From  contract.String
	To    contract.String
	Sn    contract.Integer
	ReqId contract.Integer //pk
	Data  contract.Bytes
}

func eventIndexedValue(p interface{}) interface{} {
	if eivp, ok := p.(contract.EventIndexedValueWithParam); ok {
		return eivp.Param()
	}
	if eiv, ok := p.(contract.EventIndexedValue); ok {
		return fmt.Sprintf("%s", eiv)
	}
	return p
}

func NewCallMessage(network string, e contract.Event) (evt CallMessage, err error) {
	if e.Name() != EventCallMessage {
		err = errors.Errorf("mismatch event name:%s expected:%s", e.Name(), EventCallMessage)
		return
	}
	evt.Event = e
	evt.Network = network
	p := e.Params()
	if evt.From, err = contract.StringOf(eventIndexedValue(p["_from"])); err != nil {
		return
	}
	if evt.To, err = contract.StringOf(eventIndexedValue(p["_to"])); err != nil {
		return
	}
	if evt.Sn, err = contract.IntegerOf(p["_sn"]); err != nil {
		return
	}
	if evt.ReqId, err = contract.IntegerOf(p["_reqId"]); err != nil {
		return
	}
	if evt.Data, err = contract.BytesOf(p["_data"]); err != nil {
		return
	}
	return
}

type RollbackMessage struct {
	contract.Event
	Network  string
	SentTxID contract.TxID
	Sent     bool

	Sn contract.Integer //pk
}

func NewRollbackMessage(network string, e contract.Event) (evt RollbackMessage, err error) {
	if e.Name() != EventRollbackMessage {
		err = errors.Errorf("mismatch event name:%s expected:%s", e.Name(), EventRollbackMessage)
		return
	}
	evt.Event = e
	evt.Network = network
	p := e.Params()
	if evt.Sn, err = contract.IntegerOf(p["_sn"]); err != nil {
		return
	}
	return
}

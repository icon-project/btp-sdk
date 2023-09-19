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
	"gorm.io/gorm"

	"github.com/icon-project/btp-sdk/autocaller"
	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/database"
	"github.com/icon-project/btp-sdk/service"
	"github.com/icon-project/btp-sdk/service/xcall"
)

const (
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

	cr *CallRepository
	rr *RollbackRepository
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

func NewAutoCaller(s service.Service, networks map[string]autocaller.Network, db *gorm.DB, l log.Logger) (autocaller.AutoCaller, error) {
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
		cmParams := make([]contract.Params, 0)
		for _, fa := range fn.Options.Contracts {
			from := types.BtpAddress(fmt.Sprintf("btp://%s/%s", fn.Options.NetworkAddress, fa))
			for toNetwork, tn := range nMap {
				if toNetwork == fromNetwork {
					continue
				}
				for _, ta := range tn.Options.Contracts {
					to := types.BtpAddress(fmt.Sprintf("btp://%s/%s", tn.Options.NetworkAddress, ta))
					cmParams = append(cmParams, contract.Params{
						"_from": to.String(),
						"_to":   from.ContractAddress(),
					})
				}
			}
		}
		nameToParams := map[string][]contract.Params{
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
	cr, err := NewCallRepository(db)
	if err != nil {
		return nil, err
	}
	rr, err := NewRollbackRepository(db)
	if err != nil {
		return nil, err
	}
	return &AutoCaller{
		s:    ss,
		nMap: nMap,
		l:    l,
		//
		cr: cr,
		rr: rr,
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
		go c.monitorEvent(ctx, network, n.EventFilters, n.Options.InitHeight)
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

func (c *AutoCaller) Find(fp autocaller.FindParam) (*database.Page[any], error) {
	switch fp.Task {
	case TaskCall:
		page, err := c.cr.Page(fp.Pageable, fp.Query)
		if err != nil {
			return nil, err
		}
		return page.ToAny(), err
	case TaskRollback:
		page, err := c.rr.Page(fp.Pageable, fp.Query)
		if err != nil {
			return nil, err
		}
		return page.ToAny(), err
	default:
		return nil, errors.Errorf("not found task:%s", fp.Task)
	}
}

func (c *AutoCaller) monitorEvent(ctx context.Context, network string, efs []contract.EventFilter, initHeight int64) {
	for {
		select {
		case <-time.After(time.Second):
			height, err := c.getMonitorHeight(network)
			if err != nil {
				c.l.Debugf("monitorEvent fail to getMonitorHeight network:%s err:%v", network, err)
				continue
			}
			if height < 1 {
				height = initHeight
			}
			c.l.Debugf("monitorEvent network:%s height:%v", network, height)
			if err = c.s.MonitorEvent(ctx, network, func(e contract.Event) error {
				c.l.Tracef("monitorEvent callback network:%s event:%s height:%v", network, e.Name(), e.BlockHeight())
				switch e.Name() {
				case EventCallMessage:
					return c.onCallMessage(network, e)
				case EventRollbackMessage:
					return c.onRollbackMessage(network, e)
				}
				return nil
			}, efs, height); err != nil {
				c.l.Debugf("monitorEvent stopped network:%s err:%v", network, err)
			}
		case <-ctx.Done():
			c.l.Debugln("monitorEvent context done")
			return
		}
	}
}

func (c *AutoCaller) getMonitorHeight(network string) (height int64, err error) {
	var (
		cm *Call
		rm *Rollback
	)
	if cm, err = c.cr.FindOneByNetworkOrderByEventHeightDesc(network); err != nil {
		return
	}
	if rm, err = c.rr.FindOneByNetworkOrderByEventHeightDesc(network); err != nil {
		return
	}
	if cm != nil {
		height = cm.EventHeight
	}
	if rm != nil && height < rm.EventHeight {
		height = rm.EventHeight
	}
	return height, nil
}

func (c *AutoCaller) onCallMessage(network string, e contract.Event) error {
	cm, err := NewCall(network, e)
	if err != nil {
		return err
	}
	old, err := c.cr.FindOneByNetworkAndReqID(network, cm.ReqId)
	if err != nil {
		return err
	}
	if old != nil {
		if old.Sent {
			c.l.Debugf("skip old CallMessage:{Network:%s,Sn:%v,ReqID:%v,EventHeight:%d}",
				cm.Network, cm.Sn, cm.ReqId, cm.EventHeight)
			return nil
		}
		cm.Task = old.Task
	}
	txID, err := c.s.Invoke(network, MethodExecuteCall, contract.Params{
		"_reqId": cm.ReqId,
		"_data":  cm.Data,
	}, contract.Options{
		"estimate": true,
	})
	if err != nil {
		if ee, ok := err.(contract.EstimateError); ok && ee.Reason() == ReasonInvalidRequestId {
			//TODO to ensure event processed, retry until CallExecuted(_reqId) emitted
			cm.Sent = true
			c.l.Debugf("skip %s CallMessage:{Network:%s,Sn:%v,ReqID:%v,EventHeight:%d}",
				ee.Reason(), cm.Network, cm.Sn, cm.ReqId, cm.EventHeight)
			return c.cr.Save(cm)
		}
		return err
	}
	cm.TxID = fmt.Sprintf("%s", txID)
	cm.Sent = true
	c.l.Debugf("CallMessage:{Network:%s,Sn:%v,ReqID:%v,TxID:%v}", cm.Network, cm.Sn, cm.ReqId, cm.TxID)
	return c.cr.Save(cm)
}

func (c *AutoCaller) onRollbackMessage(network string, e contract.Event) error {
	rm, err := NewRollback(network, e)
	if err != nil {
		return err
	}
	old, err := c.rr.FindOneByNetworkAndSn(network, rm.Sn)
	if err != nil {
		return err
	}
	if old != nil {
		if old.Sent {
			c.l.Debugf("skip old RollbackMessage:{Network:%s,Sn:%v,EventHeight:%d}",
				rm.Network, rm.Sn, rm.EventHeight)
			return nil
		}
		rm.Task = old.Task
	}
	txID, err := c.s.Invoke(network, MethodExecuteRollback, contract.Params{
		"_sn": rm.Sn,
	}, contract.Options{
		"estimate": true,
	})
	if err != nil {
		if ee, ok := err.(contract.EstimateError); ok &&
			(ee.Reason() == ReasonInvalidSerialNum || ee.Reason() == ReasonRollbackNotEnabled) {
			//TODO to ensure event processed, retry until RollbackExecuted(_sn) emitted
			rm.Sent = true
			c.l.Debugf("skip %s RollbackMessage:{Network:%s,Sn:%v,EventHeight:%d}",
				ee.Reason(), rm.Network, rm.Sn, rm.EventHeight)
			return c.rr.Save(rm)
		}
		return err
	}
	rm.TxID = fmt.Sprintf("%s", txID)
	rm.Sent = true
	c.l.Debugf("RollbackMessage:{Network:%s,Sn:%v,TxID:%v}", rm.Network, rm.Sn, rm.TxID)
	return c.rr.Save(rm)
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

func stringOf(v interface{}) (string, error) {
	s, err := contract.StringOf(v)
	if err != nil {
		return "", err
	}
	return string(s), nil
}

func uint64Of(v interface{}) (uint64, error) {
	i, err := contract.IntegerOf(v)
	if err != nil {
		return 0, err
	}
	return i.AsUint64()
}

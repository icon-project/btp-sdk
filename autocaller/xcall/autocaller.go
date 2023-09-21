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
	EventCallExecuted        = "CallExecuted"
	EventRollbackMessage     = "RollbackMessage"
	EventRollbackExecuted    = "RollbackExecuted"
	MethodExecuteCall        = "executeCall"
	MethodExecuteRollback    = "executeRollback"
	ReasonInvalidRequestId   = "InvalidRequestId"
	ReasonInvalidSerialNum   = "InvalidSerialNum"
	ReasonRollbackNotEnabled = "RollbackNotEnabled"
	TaskCall                 = "call"
	TaskRollback             = "rollback"
	DefaultGetResultInterval = 2 * time.Second
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
			EventCallMessage:      cmParams,
			EventCallExecuted:     nil,
			EventRollbackMessage:  nil,
			EventRollbackExecuted: nil,
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
		cr:   cr,
		rr:   rr,
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

func (c *AutoCaller) onDatabaseError(err error, format string, args ...interface{}) {
	c.l.Errorf("err:%+v msg:%s", err, fmt.Sprintf(format, args...))
}

func (c *AutoCaller) monitorEvent(ctx context.Context, network string, efs []contract.EventFilter, initHeight int64) {
	for {
		select {
		case <-time.After(time.Second):
			height, err := c.getMonitorHeight(network)
			if err != nil {
				c.onDatabaseError(err, "monitorEvent fail to getMonitorHeight network:%s", network)
				continue
			}
			if height < 1 {
				height = initHeight
			}
			c.l.Debugf("monitorEvent network:%s height:%v", network, height)
			if err = c.s.MonitorEvent(ctx, network, func(e contract.Event) error {
				c.l.Tracef("monitorEvent callback network:%s event:%s height:%v", network, e.Name(), e.BlockHeight())
				switch e.Name() {
				case EventCallMessage, EventCallExecuted:
					return c.onCallEvent(network, e)
				case EventRollbackMessage, EventRollbackExecuted:
					return c.onRollbackEvent(network, e)
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

func (c *AutoCaller) onCallEvent(network string, e contract.Event) error {
	cm, err := NewCall(network, e)
	if err != nil {
		return err
	}
	found, err := c.cr.FindOneByNetworkAndReqID(cm.Network, cm.ReqId)
	if err != nil {
		c.onDatabaseError(err, "fail to FindOneByNetworkAndReqID network:%s, reqID:%v",
			cm.Network, cm.ReqId)
		return err
	}
	switch e.Name() {
	case EventCallMessage:
		if found != nil {
			switch found.State {
			case autocaller.TaskStateNone:
				cm.Task = found.Task
			case autocaller.TaskStateSending:
				//TODO [TBD] found.Event.Equal(cm.Event)
				go c.callResult(found)
				return nil
			default:
				c.l.Debugf("skip found CallMessage:{Network:%s,Sn:%v,ReqID:%v,EventHeight:%d}",
					cm.Network, cm.Sn, cm.ReqId, cm.EventHeight)
				return nil
			}
		}
		c.executeCall(cm)
	case EventCallExecuted:
		if found != nil {
			if found.State == autocaller.TaskStateDone {
				c.l.Debugf("skip found CallExecuted:{Network:%s,ReqID:%v,EventHeight:%d}",
					cm.Network, cm.ReqId, e.BlockHeight())
				return nil
			}
			found.State = autocaller.TaskStateDone
			found.TxID = cm.TxID
			found.BlockHeight = cm.BlockHeight
			found.BlockID = cm.BlockID
		}
		if found == nil {
			c.l.Infof("missing Call CallExecuted:{Network:%s,ReqID:%v,EventHeight:%d}",
				cm.Network, cm.ReqId, e.BlockHeight())
			found = cm
		}
		if err = c.cr.Save(found); err != nil {
			c.onDatabaseError(err, "fail to Save CallExecuted:{Network:%s,ReqID:%v,EventHeight:%d}",
				cm.Network, cm.ReqId, e.BlockHeight())
		}
		return err
	}
	return nil
}

func (c *AutoCaller) executeCall(cm *Call) {
	txID, err := c.s.Invoke(cm.Network, MethodExecuteCall, contract.Params{
		"_reqId": cm.ReqId,
		"_data":  cm.Data,
	}, contract.Options{
		"estimate": true,
	})
	if err != nil {
		c.onCallError(cm, err)
		return
	}
	cm.State = autocaller.TaskStateSending
	cm.TxID = fmt.Sprintf("%s", txID)
	c.l.Debugf("CallMessage:{Network:%s,Sn:%v,ReqID:%v,TxID:%v}", cm.Network, cm.Sn, cm.ReqId, cm.TxID)
	if _, err = c.cr.SaveIfFoundStateIsNotDone(cm); err != nil {
		c.onDatabaseError(err, "executeCall fail to SaveIfFoundStateIsNotDone")
	}
	go c.callResult(cm)
}

func (c *AutoCaller) onCallError(cm *Call, err error) {
	if ee, ok := err.(contract.EstimateError); ok && ee.Reason() == ReasonInvalidRequestId {
		c.l.Debugf("skip %s CallMessage:{Network:%s,Sn:%v,ReqID:%v,EventHeight:%d} TxID:%s",
			ee.Reason(), cm.Network, cm.Sn, cm.ReqId, cm.EventHeight, cm.TxID)
		cm.State = autocaller.TaskStateSkip
		cm.TxID = ""
	} else {
		cm.State = autocaller.TaskStateError
		cm.Failure = err.Error()
	}
	if _, re := c.cr.SaveIfFoundStateIsNotDone(cm); re != nil {
		c.onDatabaseError(re, "onCallError fail to SaveIfFoundStateIsNotDone")
	}
}

func (c *AutoCaller) callResult(cm *Call) {
	c.l.Debugf("callResult TxID:%s", cm.TxID)
	txr, err := c.nMap[cm.Network].Adaptor.GetResult(cm.TxID)
	if err != nil {
		if contract.ErrorCodeNotFoundTransaction.Equals(err) {
			cm.TxID = ""
			c.executeCall(cm)
			return
		}
		c.l.Debugf("callResult fail to GetResult TxID:%s err:%+v", cm.TxID, err)
		<-time.After(DefaultGetResultInterval)
		go c.callResult(cm)
		return
	}
	if !txr.Success() {
		c.onCallError(cm, txr.Failure().(error))
		return
	}
	cm.State = autocaller.TaskStateDone
	cm.BlockID = fmt.Sprintf("%s", txr.BlockID())
	cm.BlockHeight = txr.BlockHeight()
	if _, err = c.cr.SaveIfFoundStateIsNotDone(cm); err != nil {
		c.onDatabaseError(err, "callResult fail to SaveIfFoundStateIsNotDone")
	}
}

func (c *AutoCaller) onRollbackEvent(network string, e contract.Event) error {
	rm, err := NewRollback(network, e)
	if err != nil {
		return err
	}
	found, err := c.rr.FindOneByNetworkAndSn(network, rm.Sn)
	if err != nil {
		c.onDatabaseError(err, "fail to FindOneByNetworkAndSn network:%s, sn:%v",
			rm.Network, rm.Sn)
		return err
	}
	switch e.Name() {
	case EventRollbackMessage:
		if found != nil {
			switch found.State {
			case autocaller.TaskStateNone:
				rm.Task = found.Task
			case autocaller.TaskStateSending:
				//TODO [TBD] found.Event.Equal(rm.Event)
				go c.rollbackResult(rm)
				return nil
			default:
				c.l.Debugf("skip found RollbackMessage:{Network:%s,Sn:%v,EventHeight:%d}",
					rm.Network, rm.Sn, rm.EventHeight)
				return nil
			}
		}
		c.executeRollback(rm)
	case EventRollbackExecuted:
		if found != nil {
			if found.State == autocaller.TaskStateDone {
				c.l.Debugf("skip found RollbackExecuted:{Network:%s,Sn:%v,EventHeight:%d}",
					rm.Network, rm.Sn, e.BlockHeight())
				return nil
			}
			found.State = autocaller.TaskStateDone
			found.TxID = rm.TxID
			found.BlockHeight = rm.BlockHeight
			found.BlockID = rm.BlockID
		}
		if found == nil {
			c.l.Infof("missing Rollback RollbackExecuted:{Network:%s,Sn:%v,EventHeight:%d}",
				rm.Network, rm.Sn, e.BlockHeight())
			found = rm
		}
		if err = c.rr.Save(found); err != nil {
			c.onDatabaseError(err, "fail to Save RollbackExecuted:{Network:%s,Sn:%v,EventHeight:%d}",
				rm.Network, rm.Sn, e.BlockHeight())
		}
		return err
	}
	return nil
}

func (c *AutoCaller) executeRollback(rm *Rollback) {
	txID, err := c.s.Invoke(rm.Network, MethodExecuteRollback, contract.Params{
		"_sn": rm.Sn,
	}, contract.Options{
		"estimate": true,
	})
	if err != nil {
		c.onRollbackError(rm, err)
		return
	}
	rm.State = autocaller.TaskStateSending
	rm.TxID = fmt.Sprintf("%s", txID)
	c.l.Debugf("RollbackMessage:{Network:%s,Sn:%v,TxID:%v}", rm.Network, rm.Sn, rm.TxID)
	if _, err = c.rr.SaveIfFoundStateIsNotDone(rm); err != nil {
		c.onDatabaseError(err, "executeRollback fail to SaveIfFoundStateIsNotDone")
	}
	go c.rollbackResult(rm)
}

func (c *AutoCaller) onRollbackError(rm *Rollback, err error) {
	if ee, ok := err.(contract.EstimateError); ok &&
		(ee.Reason() == ReasonInvalidSerialNum || ee.Reason() == ReasonRollbackNotEnabled) {
		c.l.Debugf("skip %s RollbackMessage:{Network:%s,Sn:%v,EventHeight:%d}",
			ee.Reason(), rm.Network, rm.Sn, rm.EventHeight)
		rm.State = autocaller.TaskStateSkip
		rm.TxID = ""
	} else {
		rm.State = autocaller.TaskStateError
		rm.Failure = err.Error()
	}
	if _, re := c.rr.SaveIfFoundStateIsNotDone(rm); re != nil {
		c.onDatabaseError(re, "onRollbackError fail to SaveIfFoundStateIsNotDone")
	}
}

func (c *AutoCaller) rollbackResult(rm *Rollback) {
	c.l.Debugf("rollbackResult TxID:%s", rm.TxID)
	txr, err := c.nMap[rm.Network].Adaptor.GetResult(rm.TxID)
	if err != nil {
		if contract.ErrorCodeNotFoundTransaction.Equals(err) {
			rm.TxID = ""
			c.executeRollback(rm)
			return
		}
		c.l.Debugf("rollbackResult fail to GetResult TxID:%s err:%+v", rm.TxID, err)
		<-time.After(DefaultGetResultInterval)
		go c.rollbackResult(rm)
		return
	}
	if !txr.Success() {
		c.onRollbackError(rm, txr.Failure().(error))
		return
	}
	rm.State = autocaller.TaskStateDone
	rm.BlockID = fmt.Sprintf("%s", txr.BlockID())
	rm.BlockHeight = txr.BlockHeight()
	if _, err = c.rr.SaveIfFoundStateIsNotDone(rm); err != nil {
		c.onDatabaseError(err, "rollbackResult fail to SaveIfFoundStateIsNotDone")
	}
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

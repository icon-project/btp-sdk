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
	"fmt"

	"github.com/icon-project/btp2/common/errors"
	"gorm.io/gorm"

	"github.com/icon-project/btp-sdk/autocaller"
	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/database"
	"github.com/icon-project/btp-sdk/service/xcall"
)

const (
	TablePrefix              = "autocaller_" + xcall.ServiceName
	CallTable                = TablePrefix + "_call"
	RollbackTable            = TablePrefix + "_rollback"
	orderByEventHeightDesc   = "event_height desc"
	ExecuteResultCodeSuccess = 0
)

type Call struct {
	autocaller.Task
	// EventHeight height of CallMessage event
	EventHeight int64 `json:"event_height"`
	// From BTPAddress of caller
	From string `json:"from"`
	// To address of callee
	To string `json:"to"`
	// Sn serial number of the request, generated in source-chain
	Sn uint64 `json:"sn"`
	// ReqId serial number of the request, generated in destination-chain
	ReqId uint64 `json:"req_id" gorm:"index"`
	// Data calldata
	Data []byte `json:"data"`
	// ExecResultCode execution result code
	ExecResultCode int64 `json:"exec_result_code"`
	// ExecResultMsg execution result message
	ExecResultMsg string `json:"exec_result_msg"`
}

func NewCall(network string, e contract.Event) (*Call, error) {
	m := &Call{
		Task: autocaller.Task{
			Name:    TaskCall,
			Network: network,
		},
	}
	p := e.Params()
	var err error
	switch e.Name() {
	case EventCallMessage:
		m.EventHeight = e.BlockHeight()
		if m.From, err = stringOf(eventIndexedValue(p["_from"])); err != nil {
			return nil, err
		}
		if m.To, err = stringOf(eventIndexedValue(p["_to"])); err != nil {
			return nil, err
		}
		if m.Sn, err = uint64Of(p["_sn"]); err != nil {
			return nil, err
		}
		if m.Data, err = contract.BytesOf(p["_data"]); err != nil {
			return nil, err
		}
	case EventCallExecuted:
		m.State = autocaller.TaskStateDone
		m.TxID = fmt.Sprintf("%s", e.TxID())
		m.BlockHeight = e.BlockHeight()
		m.BlockID = fmt.Sprintf("%s", e.BlockID())
		if m.ExecResultCode, err = int64Of(p["_code"]); err != nil {
			return nil, err
		}
		if m.ExecResultMsg, err = stringOf(p["_msg"]); err != nil {
			return nil, err
		}
	default:
		return nil, errors.Errorf("fail to NewCall event name:%s", e.Name())
	}
	if m.ReqId, err = uint64Of(p["_reqId"]); err != nil {
		return nil, err
	}
	return m, nil
}

type Rollback struct {
	autocaller.Task
	// EventHeight height of ResponseMessage event
	EventHeight int64 `json:"event_height"`
	// From address of caller
	From string `json:"from"`
	// To BTPAddress of callee
	To string `json:"to"`
	// Sn serial number of the request
	Sn uint64 `json:"sn" gorm:"index"`
	// ExecResultCode execution result code
	ExecResultCode int64 `json:"exec_result_code"`
	// ExecResultMsg execution result message
	ExecResultMsg string `json:"exec_result_msg"`
}

func NewRollback(network string, e contract.Event) (*Rollback, error) {
	m := &Rollback{
		Task: autocaller.Task{
			Name:    TaskRollback,
			Network: network,
		},
	}
	p := e.Params()
	var err error
	switch e.Name() {
	case EventCallMessageSent:
		if m.From, err = stringOf(eventIndexedValue(p["_from"])); err != nil {
			return nil, err
		}
		if m.To, err = stringOf(eventIndexedValue(p["_to"])); err != nil {
			return nil, err
		}
	case EventResponseMessage:
		m.EventHeight = e.BlockHeight()
		var code int64
		if code, err = int64Of(p["_code"]); err != nil {
			return nil, err
		}
		if code == ExecuteResultCodeSuccess {
			m.State = autocaller.TaskStateNotApplicable
		}
	case EventRollbackExecuted:
		m.State = autocaller.TaskStateDone
		m.TxID = fmt.Sprintf("%s", e.TxID())
		m.BlockHeight = e.BlockHeight()
		m.BlockID = fmt.Sprintf("%s", e.BlockID())
		if m.ExecResultCode, err = int64Of(p["_code"]); err != nil {
			return nil, err
		}
		if m.ExecResultMsg, err = stringOf(p["_msg"]); err != nil {
			return nil, err
		}
	default:
		return nil, errors.Errorf("fail to NewRollback event name:%s", e.Name())
	}
	if m.Sn, err = uint64Of(p["_sn"]); err != nil {
		return nil, err
	}
	return m, nil
}

type CallRepository struct {
	database.Repository[Call]
}

func (r *CallRepository) FindOneByNetworkAndReqID(network string, reqID uint64) (*Call, error) {
	return r.FindOne(&Call{
		Task: autocaller.Task{
			Network: network,
		},
		ReqId: reqID,
	})
}

func (r *CallRepository) FindOneByNetworkOrderByEventHeightDesc(network string) (*Call, error) {
	return r.FindOneWithOrder(orderByEventHeightDesc, &Call{
		Task: autocaller.Task{
			Network: network,
		},
	})
}

func (r *CallRepository) SaveIfFoundStateIsNotDone(v *Call) (bool, error) {
	if v.ID == 0 {
		err := r.Repository.Save(v)
		return true, err
	}
	return r.Repository.SaveIf(v, func(found *Call) bool {
		return found == nil || found.State != autocaller.TaskStateDone
	})
}

func NewCallRepository(db *gorm.DB) (*CallRepository, error) {
	r, err := database.NewDefaultRepository[Call](db, CallTable)
	if err != nil {
		return nil, err
	}
	return &CallRepository{
		Repository: r,
	}, nil
}

type RollbackRepository struct {
	database.Repository[Rollback]
}

func (r *RollbackRepository) FindOneByNetworkAndSn(network string, sn uint64) (*Rollback, error) {
	return r.FindOne(&Rollback{
		Task: autocaller.Task{
			Network: network,
		},
		Sn: sn,
	})
}

func (r *RollbackRepository) FindOneByNetworkOrderByEventHeightDesc(network string) (*Rollback, error) {
	return r.FindOneWithOrder(orderByEventHeightDesc, &Rollback{
		Task: autocaller.Task{
			Network: network,
		},
	})
}

func (r *RollbackRepository) SaveIfFoundStateIsNotDone(v *Rollback) (bool, error) {
	if v.ID == 0 {
		err := r.Repository.Save(v)
		return true, err
	}
	return r.Repository.SaveIf(v, func(found *Rollback) bool {
		return found == nil || found.State != autocaller.TaskStateDone
	})
}

func NewRollbackRepository(db *gorm.DB) (*RollbackRepository, error) {
	r, err := database.NewDefaultRepository[Rollback](db, RollbackTable)
	if err != nil {
		return nil, err
	}
	return &RollbackRepository{
		Repository: r,
	}, nil
}

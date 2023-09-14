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
	"github.com/icon-project/btp2/common/errors"
	"gorm.io/gorm"

	"github.com/icon-project/btp-sdk/autocaller"
	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/database"
	"github.com/icon-project/btp-sdk/service/xcall"
)

const (
	TablePrefix            = "autocaller_" + xcall.ServiceName
	CallTable              = TablePrefix + "_call"
	RollbackTable          = TablePrefix + "_rollback"
	orderByEventHeightDesc = "event_height desc"
)

type Call struct {
	autocaller.Task
	From  string `json:"from"`
	To    string `json:"to"`
	Sn    uint64 `json:"sn"`
	ReqId uint64 `json:"req_id" gorm:"index"`
	Data  []byte `json:"data"`

	EventHeight int64 `json:"event_height"`
}

func NewCall(network string, e contract.Event) (*Call, error) {
	m := &Call{
		Task: autocaller.Task{
			Name:    TaskCall,
			Network: network,
		},
		EventHeight: e.BlockHeight(),
	}
	if e.Name() != EventCallMessage {
		return nil, errors.Errorf("mismatch event name:%s expected:%s", e.Name(), EventCallMessage)
	}
	p := e.Params()
	var err error
	if m.From, err = stringOf(eventIndexedValue(p["_from"])); err != nil {
		return nil, err
	}
	if m.To, err = stringOf(eventIndexedValue(p["_to"])); err != nil {
		return nil, err
	}
	if m.Sn, err = uint64Of(p["_sn"]); err != nil {
		return nil, err
	}
	if m.Sn, err = uint64Of(p["_reqId"]); err != nil {
		return nil, err
	}
	if m.Data, err = contract.BytesOf(p["_data"]); err != nil {
		return nil, err
	}
	return m, nil
}

type Rollback struct {
	autocaller.Task
	Sn uint64 `json:"sn" gorm:"index"`

	EventHeight int64 `json:"event_height"`
}

func NewRollback(network string, e contract.Event) (*Rollback, error) {
	m := &Rollback{
		Task: autocaller.Task{
			Name:    TaskRollback,
			Network: network,
		},
		EventHeight: e.BlockHeight(),
	}
	if e.Name() != EventRollbackMessage {
		return nil, errors.Errorf("mismatch event name:%s expected:%s", e.Name(), EventRollbackMessage)
	}
	p := e.Params()
	var err error
	if m.Sn, err = uint64Of(p["_sn"]); err != nil {
		return nil, err
	}
	return m, nil
}

type CallRepository struct {
	*database.DefaultRepository[Call]
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

func NewCallRepository(db *gorm.DB) (*CallRepository, error) {
	r, err := database.NewDefaultRepository(db, CallTable, Call{})
	if err != nil {
		return nil, err
	}
	return &CallRepository{
		DefaultRepository: r,
	}, nil
}

type RollbackRepository struct {
	*database.DefaultRepository[Rollback]
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

func NewRollbackRepository(db *gorm.DB) (*RollbackRepository, error) {
	r, err := database.NewDefaultRepository(db, RollbackTable, Rollback{})
	if err != nil {
		return nil, err
	}
	return &RollbackRepository{
		DefaultRepository: r,
	}, nil
}

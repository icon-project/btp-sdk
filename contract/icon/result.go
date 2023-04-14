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

	"github.com/icon-project/btp2/chain/icon/client"
	"github.com/icon-project/btp2/common/errors"

	"github.com/icon-project/btp-sdk/contract"
)

type TxResult struct {
	*client.TransactionResult
	events []contract.BaseEvent
}

func (r *TxResult) Success() bool {
	return r.Status == client.ResultStatusSuccess
}

func (r *TxResult) Events() []contract.BaseEvent {
	return r.events
}

func (r *TxResult) Revert() interface{} {
	//TODO implement me
	panic("implement me")
}

func NewTxResult(txr *client.TransactionResult) (contract.TxResult, error) {
	r := &TxResult{
		TransactionResult: txr,
		events:            make([]contract.BaseEvent, len(txr.EventLogs)),
	}
	for i, l := range txr.EventLogs {
		r.events[i] = &BaseEvent{
			addr:      contract.Address(l.Addr),
			signature: EventSignature(l.Indexed[0]),
			indexed:   len(l.Indexed) - 1,
			values:    append(l.Indexed[1:], l.Data...),
		}
	}
	return r, nil
}

func (h *Handler) GetResult(id contract.TxID) (contract.TxResult, error) {
	txh, ok := id.(*client.HexBytes)
	if !ok {
		return nil, errors.Errorf("fail GetResult, invalid type %T", id)
	}
	p := &client.TransactionHashParam{
		Hash: *txh,
	}
	txr, err := h.a.GetTransactionResult(p)
	if err != nil {
		return nil, err
	}
	return NewTxResult(txr)
}

type BaseEvent struct {
	addr      contract.Address
	signature EventSignature
	indexed   int
	values    []string
}

func (e *BaseEvent) Address() contract.Address {
	return e.addr
}

func (e *BaseEvent) Signature() contract.EventSignature {
	return e.signature
}

func (e *BaseEvent) Indexed() int {
	return e.indexed
}

func (e *BaseEvent) IndexedValue(i int) contract.EventIndexedValue {
	if i <= e.indexed {
		return EventIndexedValue(e.values[i])
	}
	return nil
}

type EventSignature string

func (s EventSignature) Match(v string) bool {
	return string(s) == v
}

type EventIndexedValue string

func (i EventIndexedValue) Match(v interface{}) bool {
	switch t := v.(type) {
	case contract.Integer:
		return string(i) == string(t)
	case contract.String:
		return string(i) == string(t)
	case contract.Address:
		return string(i) == string(t)
	case contract.Bytes:
		return string(i) == string(client.NewHexBytes(t))
	case contract.Boolean:
		return string(i) == string(NewHexBool(bool(t)))
	}
	return false
}

type Event struct {
	*BaseEvent
	params contract.Params
}

func (e *Event) Params() contract.Params {
	return e.params
}

const (
	failIfNotMatchedInEventFilter = true
)

type EventFilter struct {
	spec    contract.EventSpec
	address contract.Address
	params  contract.Params
}

func (e *EventFilter) Filter(event contract.BaseEvent) (contract.Event, error) {
	if e.address != event.Address() {
		if failIfNotMatchedInEventFilter {
			return nil, errors.Errorf("address expect:%v actual:%v",
				e.address, event.Address())
		}
		return nil, nil
	}
	if !event.Signature().Match(e.spec.Signature) {
		if failIfNotMatchedInEventFilter {
			return nil, errors.Errorf("signature expect:%v actual:%v",
				e.spec.Signature, event.Signature().(*EventSignature))
		}
		return nil, nil
	}
	if e.spec.Indexed != event.Indexed() {
		if failIfNotMatchedInEventFilter {
			return nil, errors.Errorf("indexed expect:%v actual:%v",
				e.spec.Indexed, event.Indexed())
		}
		return nil, nil
	}
	l, ok := event.(*BaseEvent)
	if !ok {
		return nil, errors.Errorf("invalid type event %T", event)
	}
	params := make(contract.Params)
	for i, s := range e.spec.Inputs {
		v, err := decode(s.Type, l.values[i])
		if err != nil {
			return nil, err
		}
		params[s.Name] = v
		if p, exists := e.params[s.Name]; exists {
			equals := false
			switch s.Type.TypeID {
			case contract.TInteger:
				equals = p.(contract.Integer) == v.(contract.Integer)
			case contract.TString:
				equals = p.(contract.String) == v.(contract.String)
			case contract.TAddress:
				equals = p.(contract.Address) == v.(contract.Address)
			case contract.TBytes:
				equals = bytes.Equal(p.(contract.Bytes), v.(contract.Bytes))
			case contract.TBoolean:
				equals = p.(contract.Boolean) == v.(contract.Boolean)
			default:
				panic("unreachable code")
			}
			if !equals {
				if failIfNotMatchedInEventFilter {
					return nil, errors.Errorf("equality name:%s expect:%v actual:%v",
						s.Name, p, v)
				}
				return nil, nil
			}
		}
	}
	return &Event{
		BaseEvent: l,
		params:    params,
	}, nil
}

func (e *EventFilter) Signature() string {
	return e.spec.Signature
}

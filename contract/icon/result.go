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
	h, err := txr.BlockHeight.Value()
	if err != nil {
		return nil, err
	}
	bh, err := txr.BlockHash.Value()
	if err != nil {
		return nil, err
	}
	txh, err := txr.TxHash.Value()
	if err != nil {
		return nil, err
	}
	for i, l := range txr.EventLogs {
		r.events[i] = &BaseEvent{
			blockHeight: h,
			blockHash:   bh,
			txHash:      txh,
			indexInTx:   i,
			addr:        contract.Address(l.Addr),
			sigMatcher:  SignatureMatcher(l.Indexed[0]),
			indexed:     len(l.Indexed) - 1,
			values:      append(l.Indexed[1:], l.Data...),
		}
	}
	return r, nil
}

func NewBaseEvents(logs []struct {
	Addr    client.Address `json:"scoreAddress"`
	Indexed []string       `json:"indexed"`
	Data    []string       `json:"data"`
}) []contract.BaseEvent {
	events := make([]contract.BaseEvent, len(logs))
	for i, l := range logs {
		events[i] = &BaseEvent{
			addr:       contract.Address(l.Addr),
			sigMatcher: SignatureMatcher(l.Indexed[0]),
			indexed:    len(l.Indexed) - 1,
			values:     append(l.Indexed[1:], l.Data...),
		}
	}
	return events
}

type BaseEvent struct {
	blockHeight int64
	blockHash   []byte
	txHash      []byte
	indexInTx   int
	addr        contract.Address
	sigMatcher  SignatureMatcher
	indexed     int
	values      []string
}

func (e *BaseEvent) Address() contract.Address {
	return e.addr
}

func (e *BaseEvent) MatchSignature(v string) bool {
	return e.sigMatcher.Match(v)
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

type SignatureMatcher string

func (s SignatureMatcher) Match(v string) bool {
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
	signature string
	params    contract.Params
}

func (e *Event) Signature() string {
	return e.signature
}

func (e *Event) Params() contract.Params {
	return e.params
}

func NewEvent(spec contract.EventSpec, be *BaseEvent) (*Event, error) {
	params := make(contract.Params)
	for i, s := range spec.Inputs {
		v, err := decode(s.Type, be.values[i])
		if err != nil {
			return nil, err
		}
		params[s.Name] = v
	}
	return &Event{
		BaseEvent: be,
		signature: spec.Signature,
		params:    params,
	}, nil
}

const (
	failIfNotMatchedInEventFilter = true
)

type EventFilter struct {
	spec    contract.EventSpec
	address contract.Address
	params  contract.Params
}

func (f *EventFilter) Filter(event contract.BaseEvent) (contract.Event, error) {
	if f.address != event.Address() {
		if failIfNotMatchedInEventFilter {
			return nil, errors.Errorf("address expect:%v actual:%v",
				f.address, event.Address())
		}
		return nil, nil
	}
	if !event.MatchSignature(f.spec.Signature) {
		if failIfNotMatchedInEventFilter {
			return nil, errors.Errorf("signature expect:%v actual:%v",
				f.spec.Signature, event.(*BaseEvent).sigMatcher)
		}
		return nil, nil
	}
	if f.spec.Indexed != event.Indexed() {
		if failIfNotMatchedInEventFilter {
			return nil, errors.Errorf("indexed expect:%v actual:%v",
				f.spec.Indexed, event.Indexed())
		}
		return nil, nil
	}
	be, ok := event.(*BaseEvent)
	if !ok {
		return nil, errors.Errorf("invalid type event %T", event)
	}
	e, err := NewEvent(f.spec, be)
	if err != nil {
		return nil, err
	}
	for k, v := range f.params {
		if p, exists := e.params[k]; exists {
			equals := false
			if equals, err = contract.EqualParam(f.spec.InputMap[k].Type, p, v); err != nil {
				return nil, err
			}
			if !equals {
				if failIfNotMatchedInEventFilter {
					return nil, errors.Errorf("equality name:%s expect:%v actual:%v",
						k, p, v)
				}
				return nil, nil
			}
		} else {
			return nil, errors.Errorf("not exists param in event name:%s", k)
		}
	}
	return e, nil
}

func (f *EventFilter) Signature() string {
	return f.spec.Signature
}

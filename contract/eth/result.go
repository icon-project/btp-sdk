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

package eth

import (
	"bytes"
	"encoding/hex"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/icon-project/btp2/common/errors"

	"github.com/icon-project/btp-sdk/contract"
)

type TxResult struct {
	*types.Receipt
	events []contract.BaseEvent
}

func (r *TxResult) Success() bool {
	return r.Status == types.ReceiptStatusSuccessful
}

func (r *TxResult) Events() []contract.BaseEvent {
	return r.events
}

func (r *TxResult) Revert() interface{} {
	//TODO implement me
	panic("implement me")
}

func NewTxResult(txr *types.Receipt) (contract.TxResult, error) {
	r := &TxResult{
		Receipt: txr,
		events:  make([]contract.BaseEvent, len(txr.Logs)),
	}
	for i, l := range txr.Logs {
		r.events[i] = &BaseEvent{
			Log:       l,
			signature: EventSignature(l.Topics[0].String()),
			indexed:   len(l.Topics) - 1,
		}
	}
	return r, nil
}

type BaseEvent struct {
	*types.Log
	signature EventSignature
	indexed   int
}

func (e *BaseEvent) Address() contract.Address {
	return contract.Address(e.Log.Address.String())
}

func (e *BaseEvent) Signature() contract.EventSignature {
	return e.signature
}

func (e *BaseEvent) Indexed() int {
	return e.indexed
}

func (e *BaseEvent) IndexedValue(i int) contract.EventIndexedValue {
	if i <= e.indexed {
		return EventIndexedValue(e.Topics[i+1].String())
	}
	return nil
}

type EventSignature string

func (s EventSignature) Match(v string) bool {
	return string(s) == crypto.Keccak256Hash([]byte(v)).String()
}

type EventIndexedValue string

func (i EventIndexedValue) Match(v interface{}) bool {
	var tv interface{}
	switch t := v.(type) {
	case contract.Integer:
		tv, _ = t.AsBigInt()
	case contract.String:
		tv = string(t)
	case contract.Address:
		tv = string(t)
	case contract.Bytes:
		tv = []byte(t)
	case contract.Boolean:
		tv = bool(t)
	}
	if topics, err := abi.MakeTopics([]interface{}{tv}); err == nil {
		return string(i) == topics[0][0].String()
	}
	return false
}

type HashValue []byte

func (h HashValue) Match(v interface{}) bool {
	var b []byte
	switch t := v.(type) {
	case contract.String:
		b = []byte(t)
	case contract.Bytes:
		b = t
	default:
		return false
	}
	return bytes.Equal(h, crypto.Keccak256Hash(b).Bytes())
}

func (h HashValue) Bytes() []byte {
	return h
}

func (h HashValue) String() string {
	return hex.EncodeToString(h)
}

type Event struct {
	contract.BaseEvent
	params contract.Params
}

func (e *Event) Params() contract.Params {
	return e.params
}

const (
	failIfNotMatchedInEventFilter = true
)

type EventFilter struct {
	in      contract.EventSpec
	out     abi.Event
	indexed abi.Arguments
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
	if !event.Signature().Match(e.out.Sig) {
		if failIfNotMatchedInEventFilter {
			return nil, errors.Errorf("signature expect:%v actual:%v",
				e.out.Sig, event.Signature().(*EventSignature))
		}
		return nil, nil
	}
	if event.Indexed() != e.in.Indexed {
		if failIfNotMatchedInEventFilter {
			return nil, errors.Errorf("indexed expect:%v actual:%v",
				e.in.Indexed, event.Indexed())
		}
		return nil, nil
	}
	l, ok := event.(*BaseEvent)
	if !ok {
		return nil, errors.Errorf("invalid type event %T", event)
	}
	out := make(map[string]interface{})
	if err := abi.ParseTopicsIntoMap(out, e.indexed, l.Topics[1:]); err != nil {
		return nil, errors.Wrapf(err, "fail to ParseTopicsIntoMap err:%s", err.Error())
	}
	if err := e.out.Inputs.UnpackIntoMap(out, l.Data); err != nil {
		return nil, errors.Wrapf(err, "fail to UnpackIntoMap err:%s", err.Error())
	}

	params := make(contract.Params)
	for i, s := range e.in.Inputs {
		if i < e.in.Indexed && (s.Type.TypeID == contract.TString || s.Type.TypeID == contract.TBytes) {
			//struct type, array, ...
			v := HashValue(l.Topics[i+1].Bytes())
			params[s.Name] = v
			if p, exists := e.params[s.Name]; exists {
				if !v.Match(p) {
					if failIfNotMatchedInEventFilter {
						return nil, errors.Errorf("name:%s expect:%v actual:%v",
							s.Name, p, v)
					}
					return nil, nil
				}
			}
		} else {
			v, err := decode(s.Type, out[s.Name])
			if err != nil {
				return nil, err
			}
			params[s.Name] = v
			if p, exists := e.params[s.Name]; exists {
				equals, err := compareEventValue(s.Type, p, v)
				if err != nil {
					return nil, err
				}
				if !equals {
					if failIfNotMatchedInEventFilter {
						return nil, errors.Errorf("name:%s expect:%v actual:%v",
							s.Name, p, v)
					}
					return nil, nil
				}
			}
		}
	}
	return &Event{
		BaseEvent: event,
		params:    params,
	}, nil
}

func compareEventValue(s contract.TypeSpec, p, v interface{}) (equals bool, err error) {
	if s.Dimension > 0 {
		//compare array
		return false, errors.New("not implemented compare array")
	}
	switch s.TypeID {
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
	case contract.TStruct:
		m1, ok := p.(map[string]interface{})
		if !ok {
			return false, errors.Errorf("invalid type p:%T", v)
		}
		m2, ok := v.(map[string]interface{})
		if !ok {
			return false, errors.Errorf("invalid type v:%T", v)
		}
		for n, f := range s.Resolved.FieldMap {
			if ret, err := compareEventValue(f.Type, m1[n], m2[n]); err != nil {
				return false, err
			} else if !ret {
				return false, nil
			}
		}
		return true, nil
	default:
		return false, errors.New("unreachable code")
	}
	return equals, nil
}

func (e *EventFilter) Signature() string {
	return e.out.Sig
}

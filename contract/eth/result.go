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
	"math/big"
	"reflect"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
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
		return Topic(e.Topics[i+1].Bytes())
	}
	return nil
}

type EventSignature string

func (s EventSignature) Match(v string) bool {
	return string(s) == crypto.Keccak256Hash([]byte(v)).String()
}

type Topic []byte

func (t Topic) Match(value interface{}) bool {
	switch v := value.(type) {
	case contract.HashValue:
		return bytes.Equal(t, v.Bytes())
	case common.Hash:
		return bytes.Equal(t, v.Bytes())
	default:
		nt, _ := NewTopic(value)
		return bytes.Equal(t, nt.Bytes())
	}
}

func (t Topic) Bytes() []byte {
	return t
}

func (t Topic) String() string {
	return hex.EncodeToString(t)
}

func NewTopic(value interface{}) (Topic, error) {
	switch v := value.(type) {
	case contract.Integer, contract.Boolean, contract.Address:
		return makeWords(value)
	case contract.String:
		return crypto.Keccak256([]byte(v)), nil
	case contract.Bytes:
		return crypto.Keccak256(v), nil
	default:
		if b, err := makeWords(value); err != nil {
			return nil, err
		} else {
			return crypto.Keccak256(b), nil
		}
	}
}

func makeWords(value interface{}) ([]byte, error) {
	switch v := value.(type) {
	case contract.Bytes:
		b := []byte(v)
		return common.RightPadBytes(b, (len(b)+31)/32*32), nil
	case contract.String:
		b := []byte(v)
		return common.RightPadBytes(b, (len(b)+31)/32*32), nil
	case contract.Integer:
		bi, err := v.AsBigInt()
		if err != nil {
			return nil, err
		}
		return math.U256Bytes(bi), nil
	case contract.Boolean:
		var bi *big.Int
		if v {
			bi = common.Big1
		} else {
			bi = common.Big0
		}
		return math.PaddedBigBytes(bi, 32), nil
	case contract.Address:
		b := common.HexToAddress(string(v)).Bytes()
		return common.LeftPadBytes(b, 32), nil
	case contract.Struct:
		var b []byte
		for _, f := range v.Fields {
			packed, err := makeWords(f.Value)
			if err != nil {
				return nil, err
			}
			b = append(b, packed...)
		}
		return b, nil
	default:
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.Array, reflect.Slice:
			var b []byte
			for i := 0; i < rv.Len(); i++ {
				packed, err := makeWords(rv.Index(i).Interface())
				if err != nil {
					return nil, err
				}
				b = append(b, packed...)
			}
			return b, nil
		default:
			return nil, errors.Errorf("not supported type:%T", v)
		}
	}
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
	in           contract.EventSpec
	out          abi.Event
	outIndexed   abi.Arguments
	address      contract.Address
	params       contract.Params
	hashedParams map[string]Topic
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
	for i, arg := range e.outIndexed {
		if arg.Type.T == abi.TupleTy {
			out[arg.Name] = l.Topics[i+1]
		} else {
			if err := abi.ParseTopicsIntoMap(out, abi.Arguments{arg}, l.Topics[i+1:i+2]); err != nil {
				return nil, errors.Wrapf(err, "fail to ParseTopicsIntoMap err:%s", err.Error())
			}
		}
	}
	if err := e.out.Inputs.NonIndexed().UnpackIntoMap(out, l.Data); err != nil {
		return nil, errors.Wrapf(err, "fail to UnpackIntoMap err:%s", err.Error())
	}

	params := make(contract.Params)
	for i, s := range e.in.Inputs {
		if i < e.in.Indexed && (s.Type.Dimension > 0 || s.Type.TypeID == contract.TStruct ||
			s.Type.TypeID == contract.TString || s.Type.TypeID == contract.TBytes) {
			v := Topic(l.Topics[i+1].Bytes())
			params[s.Name] = v
			if p, exists := e.hashedParams[s.Name]; exists {
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
				equals, err := equalEventValue(s.Type, p, v)
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

func equalEventValue(s contract.TypeSpec, p, v interface{}) (equals bool, err error) {
	if s.Dimension > 0 {
		return equalEventArrayValue(s, 1, reflect.ValueOf(p), reflect.ValueOf(v))
	} else {
		return equalEventPrimitiveValue(s, p, v)
	}
}

func equalEventPrimitiveValue(s contract.TypeSpec, p, v interface{}) (equals bool, err error) {
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
		var m1, m2 map[string]interface{}
		if m1, err = structToMap(p); err != nil {
			return false, err
		}
		if m2, err = structToMap(v); err != nil {
			return false, err
		}
		for n, f := range s.Resolved.FieldMap {
			if equals, err = equalEventValue(f.Type, m1[n], m2[n]); err != nil {
				return false, err
			} else if !equals {
				return false, nil
			}
		}
		return true, nil
	default:
		return false, errors.New("unreachable code")
	}
	return equals, nil
}

func equalEventArrayValue(s contract.TypeSpec, dimension int, p, v reflect.Value) (equals bool, err error) {
	if p.Len() != v.Len() {
		return false, errors.New("mismatch array length")
	}
	for i := 0; i < p.Len(); i++ {
		if s.Dimension == dimension {
			equals, err = equalEventPrimitiveValue(s, p.Index(i).Interface(), v.Index(i).Interface())
		} else {
			equals, err = equalEventArrayValue(s, dimension+1, p.Index(i), v.Index(i))
		}
		if err != nil {
			return false, err
		}
		if !equals {
			return false, nil
		}
	}
	return true, nil
}

func structToMap(obj interface{}) (map[string]interface{}, error) {
	var m map[string]interface{}
	st, ok := obj.(contract.Struct)
	if ok {
		m = st.Params()
	} else {
		if m, ok = obj.(contract.Params); !ok {
			if m, ok = obj.(map[string]interface{}); !ok {
				return nil, errors.Errorf("fail encodeStruct, invalid type %T", obj)
			}
		}
	}
	return m, nil
}

func (e *EventFilter) Signature() string {
	return e.out.Sig
}

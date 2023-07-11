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
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/icon-project/btp2/common/errors"

	"github.com/icon-project/btp-sdk/contract"
)

func NewTxID(txh common.Hash) contract.TxID {
	return txh.Bytes()
}

func NewBlockID(bh common.Hash) contract.BlockID {
	return bh.Bytes()
}

type TxFailure struct {
	Error string      `json:"error"`
	Code  int         `json:"code"`
	Data  interface{} `json:"data,omitempty"`
}

func NewTxFailure(err error) (*TxFailure, error) {
	txf := &TxFailure{}
	if e, ok := err.(rpc.Error); ok {
		txf.Code = e.ErrorCode()
	} else {
		return nil, err
	}
	if de, ok := err.(rpc.DataError); ok {
		txf.Data = de.ErrorData()
	} else {
		return nil, err
	}
	return txf, nil
}

type TxResult struct {
	*types.Receipt
	events  []contract.BaseEvent
	failure *TxFailure
}

func IsSuccess(txr *types.Receipt) bool {
	return txr.Status == types.ReceiptStatusSuccessful
}

func (r *TxResult) Success() bool {
	return IsSuccess(r.Receipt)
}

func (r *TxResult) Events() []contract.BaseEvent {
	return r.events
}

func (r *TxResult) Failure() interface{} {
	return r.failure
}

func (r *TxResult) BlockID() contract.BlockID {
	return NewBlockID(r.Receipt.BlockHash)
}

func (r *TxResult) BlockHeight() int64 {
	return r.Receipt.BlockNumber.Int64()
}

func (r *TxResult) TxID() contract.TxID {
	return NewTxID(r.Receipt.TxHash)
}

func (r *TxResult) Format(f fmt.State, c rune) {
	switch c {
	case 'v', 's':
		if f.Flag('+') {
			events := make([]string, len(r.events))
			for i, e := range r.events {
				events[i] = fmt.Sprintf("%+v", e)
			}
			fmt.Fprintf(f, "TxResult{Receipt{%+v},numOfEvents:%d,events:{%s},failure:%+v}",
				r.Receipt, len(r.events), strings.Join(events, ","), r.failure)
		} else {
			events := make([]string, len(r.events))
			for i, e := range r.events {
				events[i] = fmt.Sprintf("%v", e)
			}
			fmt.Fprintf(f, "TxResult{success:%v,numOfEvents:%d,events:{%s},failure:%v,blockID:%s,blockheight:%d,txID:%s}",
				r.Success(), len(r.events), strings.Join(events, ","), r.Failure(),
				r.BlockHash.Hex(), r.BlockNumber, r.TxHash.Hex())
		}
	}
}

func NewTxResult(txr *types.Receipt, failure *TxFailure) contract.TxResult {
	r := &TxResult{
		Receipt: txr,
		events:  make([]contract.BaseEvent, len(txr.Logs)),
		failure: failure,
	}
	for i, l := range txr.Logs {
		r.events[i] = NewBaseEvent(l)
	}
	return r
}

type TxResultJson struct {
	Raw     *types.Receipt
	Failure *TxFailure
}

func (r *TxResult) UnmarshalJSON(b []byte) error {
	v := &TxResultJson{}
	if err := json.Unmarshal(b, v); err != nil {
		return err
	}
	txr := NewTxResult(v.Raw, v.Failure)
	*r = *txr.(*TxResult)
	return nil
}

func (r *TxResult) MarshalJSON() ([]byte, error) {
	v := TxResultJson{
		Raw:     r.Receipt,
		Failure: r.failure,
	}
	return json.Marshal(v)
}

type BaseEvent struct {
	*types.Log
	sigMatcher SignatureMatcher
	indexed    int
}

func (e *BaseEvent) Address() contract.Address {
	return contract.Address(e.Log.Address.String())
}

func (e *BaseEvent) SignatureMatcher() contract.SignatureMatcher {
	return e.sigMatcher
}

func (e *BaseEvent) Indexed() int {
	return e.indexed
}

func (e *BaseEvent) IndexedValue(i int) contract.EventIndexedValue {
	if i < e.indexed {
		return Topic(e.Topics[i+1].Bytes())
	}
	return nil
}

func (e *BaseEvent) BlockID() contract.BlockID {
	return NewBlockID(e.Log.BlockHash)
}

func (e *BaseEvent) BlockHeight() int64 {
	return int64(e.Log.BlockNumber)
}

func (e *BaseEvent) TxID() contract.TxID {
	return NewTxID(e.Log.TxHash)
}

func (e *BaseEvent) Identifier() int {
	return int(e.Log.Index)
}

func (e *BaseEvent) Format(f fmt.State, c rune) {
	indexedValues := make([]string, 0)
	for i := 0; i < e.indexed; i++ {
		indexedValues = append(indexedValues, hex.EncodeToString(e.IndexedValue(i).(Topic)))
	}
	switch c {
	case 'v', 's':
		if f.Flag('+') {
			fmt.Fprintf(f, "BaseEvent{blockHeight:%d,blockHash:%s,txHash:%s,indexInBlock:%d,addr:%s,signature:%s,indexed:%d,indexedValues:{%s},data:%s}",
				e.BlockNumber, hex.EncodeToString(e.BlockHash.Bytes()), hex.EncodeToString(e.TxHash.Bytes()), e.Identifier(),
				e.Address(), e.sigMatcher, e.indexed, strings.Join(indexedValues, ","), hex.EncodeToString(e.Data))
		} else {
			fmt.Fprintf(f, "BaseEvent{addr:%s,signature:%s,indexed:%d,indexedValues:{%s},data:%s}",
				e.Address(), e.sigMatcher, e.indexed, strings.Join(indexedValues, ","), hex.EncodeToString(e.Data))
		}
	}
}

func NewBaseEvent(l *types.Log) *BaseEvent {
	return &BaseEvent{
		Log:        l,
		sigMatcher: SignatureMatcher(l.Topics[0].String()),
		indexed:    len(l.Topics) - 1,
	}
}

type BaseEventJson struct {
	Raw *types.Log
}

func (e *BaseEvent) MarshalJSON() ([]byte, error) {
	v := BaseEventJson{
		Raw: e.Log,
	}
	return json.Marshal(v)
}

func (e *BaseEvent) UnmarshalJSON(b []byte) error {
	v := &BaseEventJson{}
	if err := json.Unmarshal(b, v); err != nil {
		return err
	}
	be := NewBaseEvent(v.Raw)
	*e = *be
	return nil
}

type SignatureMatcher string

func (s SignatureMatcher) Match(v string) bool {
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

type TopicWithParam struct {
	Topic
	spec  contract.NameAndTypeSpec
	param interface{}
}

func (t *TopicWithParam) Spec() contract.NameAndTypeSpec {
	return t.spec
}

func (t *TopicWithParam) Param() interface{} {
	return t.param
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

func (e *Event) Format(f fmt.State, c rune) {
	switch c {
	case 'v', 's':
		if f.Flag('+') {
			fmt.Fprintf(f, "Event{blockHeight:%d,blockHash:%s,txHash:%s,indexInBlock:%d,addr:%s,signature:%s,indexed:%d,params:%v}",
				e.BlockNumber, hex.EncodeToString(e.BlockHash.Bytes()), hex.EncodeToString(e.TxHash.Bytes()), e.Identifier(),
				e.Address(), e.signature, e.indexed, e.params)
		} else {
			fmt.Fprintf(f, "Event{addr:%s,signature:%s,indexed:%d,params:%v}",
				e.Address(), e.signature, e.indexed, e.params)
		}
	}
}

type EventJson struct {
	BaseEvent *BaseEvent
	Signature string
	Params    contract.Params
}

func (e *Event) MarshalJSON() ([]byte, error) {
	v := EventJson{
		BaseEvent: e.BaseEvent,
		Signature: e.Signature(),
		Params:    e.Params(),
	}
	return json.Marshal(v)
}

func (e *Event) UnmarshalJSON(b []byte) error {
	v := &EventJson{}
	if err := json.Unmarshal(b, v); err != nil {
		return err
	}
	*e = Event{
		BaseEvent: v.BaseEvent,
		signature: v.Signature,
		params:    v.Params,
	}
	return nil
}

func NewEvent(in contract.EventSpec, out abi.Event, outIndexed abi.Arguments, be *BaseEvent) (*Event, error) {
	m := make(map[string]interface{})
	for i, arg := range outIndexed {
		if arg.Type.T == abi.TupleTy {
			m[arg.Name] = be.Topics[i+1]
		} else {
			if err := abi.ParseTopicsIntoMap(m, abi.Arguments{arg}, be.Topics[i+1:i+2]); err != nil {
				return nil, errors.Wrapf(err, "fail to ParseTopicsIntoMap err:%s", err.Error())
			}
		}
	}
	if err := out.Inputs.NonIndexed().UnpackIntoMap(m, be.Data); err != nil {
		return nil, errors.Wrapf(err, "fail to UnpackIntoMap err:%s", err.Error())
	}
	params := make(contract.Params)
	for i, s := range in.Inputs {
		if i < in.Indexed && (s.Type.Dimension > 0 || s.Type.TypeID == contract.TStruct ||
			s.Type.TypeID == contract.TString || s.Type.TypeID == contract.TBytes) {
			v := Topic(be.Topics[i+1].Bytes())
			params[s.Name] = v
		} else {
			v, err := decode(s.Type, m[s.Name])
			if err != nil {
				return nil, err
			}
			params[s.Name] = v
		}
	}
	return &Event{
		BaseEvent: be,
		signature: out.Sig,
		params:    params,
	}, nil
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

func (f *EventFilter) Filter(event contract.BaseEvent) (contract.Event, error) {
	if f.address != event.Address() {
		if failIfNotMatchedInEventFilter {
			return nil, errors.Errorf("address expect:%v actual:%v",
				f.address, event.Address())
		}
		return nil, nil
	}
	if !event.SignatureMatcher().Match(f.out.Sig) {
		if failIfNotMatchedInEventFilter {
			return nil, errors.Errorf("signature expect:%v actual:%v",
				f.out.Sig, event.(*BaseEvent).sigMatcher)
		}
		return nil, nil
	}
	if event.Indexed() != f.in.Indexed {
		if failIfNotMatchedInEventFilter {
			return nil, errors.Errorf("indexed expect:%v actual:%v",
				f.in.Indexed, event.Indexed())
		}
		return nil, nil
	}
	be, ok := event.(*BaseEvent)
	if !ok {
		return nil, errors.Errorf("invalid type event %T", event)
	}
	e, err := NewEvent(f.in, f.out, f.outIndexed, be)
	if err != nil {
		return nil, err
	}
	for k, v := range f.params {
		if p, exists := e.params[k]; exists {
			s := f.in.InputMap[k]
			if hp, hashed := f.hashedParams[k]; hashed && (s.Type.Dimension > 0 || s.Type.TypeID == contract.TStruct ||
				s.Type.TypeID == contract.TString || s.Type.TypeID == contract.TBytes) {
				if !hp.Match(p) {
					if failIfNotMatchedInEventFilter {
						return nil, errors.Errorf("name:%s expect:%v actual:%v",
							s.Name, p, v)
					}
					return nil, nil
				}
				e.params[k] = &TopicWithParam{
					Topic: hp,
					spec:  *f.in.InputMap[k],
					param: v,
				}
			} else {
				equals := false
				if equals, err = contract.EqualParam(s.Type, p, v); err != nil {
					return nil, err
				}
				if !equals {
					if failIfNotMatchedInEventFilter {
						return nil, errors.Errorf("equality name:%s expect:%v actual:%v",
							k, p, v)
					}
					return nil, nil
				}
			}
		} else {
			return nil, errors.Errorf("not exists param in event name:%s", k)
		}
	}
	return e, nil
}

func (f *EventFilter) Signature() string {
	return f.out.Sig
}

func (f *EventFilter) Address() contract.Address {
	return f.address
}

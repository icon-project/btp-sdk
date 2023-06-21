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
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/icon-project/btp2/chain/icon/client"
	"github.com/icon-project/btp2/common/errors"

	"github.com/icon-project/btp-sdk/contract"
)

type TxResult struct {
	*client.TransactionResult
	events      []contract.BaseEvent
	blockHeight int64
	blockHash   []byte
}

func (r *TxResult) Success() bool {
	return r.Status == client.ResultStatusSuccess
}

func (r *TxResult) Events() []contract.BaseEvent {
	return r.events
}

func (r *TxResult) Failure() interface{} {
	return r.TransactionResult.Failure
}

func NewTxResult(txr *client.TransactionResult, blockHeight int64, blockHash []byte) (contract.TxResult, error) {
	r := &TxResult{
		TransactionResult: txr,
		events:            make([]contract.BaseEvent, len(txr.EventLogs)),
		blockHeight:       blockHeight,
		blockHash:         blockHash,
	}
	txh, err := txr.TxHash.Value()
	if err != nil {
		return nil, err
	}
	for i, l := range txr.EventLogs {
		r.events[i] = &BaseEvent{
			blockHeight: blockHeight,
			blockHash:   blockHash,
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

type TxResultJson struct {
	Raw         *client.TransactionResult
	BlockHeight int64
	BlockHash   []byte
}

func (r *TxResult) UnmarshalJSON(bytes []byte) error {
	v := &TxResultJson{}
	if err := json.Unmarshal(bytes, v); err != nil {
		return err
	}
	txr, err := NewTxResult(v.Raw, v.BlockHeight, v.BlockHash)
	if err != nil {
		return err
	}
	*r = *txr.(*TxResult)
	return nil
}

func (r *TxResult) MarshalJSON() ([]byte, error) {
	v := TxResultJson{
		Raw:         r.TransactionResult,
		BlockHeight: r.blockHeight,
		BlockHash:   r.blockHash,
	}
	return json.Marshal(v)
}

type BaseEvent struct {
	blockHeight int64
	blockHash   []byte
	txHash      []byte
	txIndex     int
	indexInTx   int
	addr        contract.Address
	sigMatcher  SignatureMatcher
	indexed     int
	values      []string
}

func (e *BaseEvent) Address() contract.Address {
	return e.addr
}

func (e *BaseEvent) SignatureMatcher() contract.SignatureMatcher {
	return e.sigMatcher
}

func (e *BaseEvent) Indexed() int {
	return e.indexed
}

func (e *BaseEvent) IndexedValue(i int) contract.EventIndexedValue {
	if i < e.indexed {
		return EventIndexedValue(e.values[i])
	}
	return nil
}

func (e *BaseEvent) BlockID() contract.BlockID {
	return e.blockHash
}

func (e *BaseEvent) BlockHeight() int64 {
	return e.blockHeight
}

func (e *BaseEvent) TxID() contract.TxID {
	return e.txHash
}

func (e *BaseEvent) IndexInTx() int {
	return e.indexInTx
}
func (e *BaseEvent) Format(f fmt.State, c rune) {
	switch c {
	case 'v', 's':
		if f.Flag('+') {
			fmt.Fprintf(f, "BaseEvent{blockHeight:%d,blockHash:%s,txHash:%s,txIndex:%d,indexInTx:%d,addr:%s,signature:%s,indexed:%d,values:{%s}}",
				e.blockHeight, hex.EncodeToString(e.blockHash), hex.EncodeToString(e.txHash), e.txIndex, e.indexInTx,
				e.addr, e.sigMatcher, e.indexed, strings.Join(e.values, ","))
		} else {
			fmt.Fprintf(f, "BaseEvent{addr:%s,signature:%s,indexed:%d,values:{%s}}",
				e.addr, e.sigMatcher, e.indexed, strings.Join(e.values, ","))
		}
	}
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

type EventIndexedValueWithParam struct {
	EventIndexedValue
	spec  contract.NameAndTypeSpec
	param interface{}
}

func (i EventIndexedValueWithParam) Spec() contract.NameAndTypeSpec {
	return i.spec
}

func (i EventIndexedValueWithParam) Param() interface{} {
	return i.param
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
			fmt.Fprintf(f, "Event{blockHeight:%d,blockHash:%s,txHash:%s,txIndex:%d,indexInTx:%d,addr:%s,signature:%s,indexed:%d,params:%v}",
				e.blockHeight, hex.EncodeToString(e.blockHash), hex.EncodeToString(e.txHash), e.txIndex, e.indexInTx,
				e.addr, e.signature, e.indexed, e.params)
		} else {
			fmt.Fprintf(f, "Event{addr:%s,signature:%s,indexed:%d,params:%v}",
				e.addr, e.signature, e.indexed, e.params)
		}
	}
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
	if !event.SignatureMatcher().Match(f.spec.Signature) {
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
			s := f.spec.InputMap[k]
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
			idx := f.spec.NameToIndex[k]
			if idx < e.indexed {
				e.params[k] = &EventIndexedValueWithParam{
					EventIndexedValue: EventIndexedValue(e.values[idx]),
					spec:              *s,
					param:             v,
				}
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

func (f *EventFilter) Address() contract.Address {
	return f.address
}

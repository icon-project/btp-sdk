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

func (r *TxResult) BlockID() contract.BlockID {
	return r.blockHash
}

func (r *TxResult) BlockHeight() int64 {
	return r.blockHeight
}

func (r *TxResult) TxID() contract.TxID {
	txh, _ := r.TransactionResult.TxHash.Value()
	return txh
}

func (r *TxResult) Format(f fmt.State, c rune) {
	switch c {
	case 'v', 's':
		if f.Flag('+') {
			events := make([]string, len(r.events))
			for i, e := range r.events {
				events[i] = fmt.Sprintf("%+v", e)
			}
			fmt.Fprintf(f, "TxResult{TransactionResult{%+v},numOfEvents:%d,events:{%s},blockHash:%s,blockHeight:%d}",
				r.TransactionResult, len(r.events), strings.Join(events, ","), hex.EncodeToString(r.blockHash), r.blockHeight)
		} else {
			events := make([]string, len(r.events))
			for i, e := range r.events {
				events[i] = fmt.Sprintf("%v", e)
			}
			fmt.Fprintf(f, "TxResult{success:%v,numOfEvents:%d,events:{%s},failure:%v,blockID:%s,blockheight:%d,txID:%s}",
				r.Success(), len(r.events), strings.Join(events, ","), r.Failure(),
				hex.EncodeToString(r.blockHash), r.blockHeight, r.TxHash)
		}
	}
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
	txi, err := txr.TxIndex.Int()
	if err != nil {
		return nil, err
	}
	for i, el := range txr.EventLogs {
		r.events[i] = NewBaseEvent(el, blockHeight, blockHash, txh, txi, i)
	}
	return r, nil
}

type TxResultJson struct {
	Raw         *client.TransactionResult
	BlockHeight int64
	BlockHash   []byte
}

func (r *TxResult) UnmarshalJSON(b []byte) error {
	v := &TxResultJson{}
	if err := json.Unmarshal(b, v); err != nil {
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
	client.EventLog
	blockHeight int64
	blockHash   []byte
	txHash      []byte
	txIndex     int
	indexInTx   int
	sigMatcher  SignatureMatcher
	indexed     int
}

func (e *BaseEvent) Address() contract.Address {
	return contract.Address(e.EventLog.Addr)
}

func (e *BaseEvent) SignatureMatcher() contract.SignatureMatcher {
	return e.sigMatcher
}

func (e *BaseEvent) Indexed() int {
	return e.indexed
}

func (e *BaseEvent) IndexedValue(i int) contract.EventIndexedValue {
	if i < e.indexed {
		return EventIndexedValue(e.EventLog.Indexed[i+1])
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

func (e *BaseEvent) Identifier() int {
	return e.indexInTx
}
func (e *BaseEvent) Format(f fmt.State, c rune) {
	switch c {
	case 'v', 's':
		if f.Flag('+') {
			fmt.Fprintf(f, "BaseEvent{blockHeight:%d,blockHash:%s,txHash:%s,txIndex:%d,indexInTx:%d,addr:%s,signature:%s,indexed:%d,indexedValues:{%s},data:{%s}}",
				e.blockHeight, hex.EncodeToString(e.blockHash), hex.EncodeToString(e.txHash), e.txIndex, e.indexInTx,
				e.Address(), e.sigMatcher, e.indexed, strings.Join(e.EventLog.Indexed[1:], ","), strings.Join(e.EventLog.Data, ","))
		} else {
			fmt.Fprintf(f, "BaseEvent{addr:%s,signature:%s,indexed:%d,indexedValues:{%s},data:{%s}}",
				e.Address(), e.sigMatcher, e.indexed, strings.Join(e.EventLog.Indexed[1:], ","), strings.Join(e.EventLog.Data, ","))
		}
	}
}

func NewBaseEvent(el client.EventLog, blockHeight int64, blockHash, txHash []byte, txIndex, indexInTx int) *BaseEvent {
	return &BaseEvent{
		EventLog:    el,
		blockHeight: blockHeight,
		blockHash:   blockHash,
		txHash:      txHash,
		txIndex:     txIndex,
		indexInTx:   indexInTx,
		sigMatcher:  SignatureMatcher(el.Indexed[0]),
		indexed:     len(el.Indexed) - 1,
	}
}

type BaseEventJson struct {
	Raw         client.EventLog
	BlockHeight int64
	BlockHash   []byte
	TxHash      []byte
	TxIndex     int
	IndexInTx   int
}

func (e *BaseEvent) MarshalJSON() ([]byte, error) {
	v := BaseEventJson{
		Raw:         e.EventLog,
		BlockHeight: e.blockHeight,
		BlockHash:   e.blockHash,
		TxHash:      e.txHash,
		TxIndex:     e.txIndex,
		IndexInTx:   e.indexInTx,
	}
	return json.Marshal(v)
}

func (e *BaseEvent) UnmarshalJSON(b []byte) error {
	v := &BaseEventJson{}
	if err := json.Unmarshal(b, v); err != nil {
		return err
	}
	be := NewBaseEvent(v.Raw, v.BlockHeight, v.BlockHash, v.TxHash, v.TxIndex, v.IndexInTx)
	*e = *be
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

func (i EventIndexedValueWithParam) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.EventIndexedValue)
}

type Event struct {
	*BaseEvent
	signature string
	name      string
	params    contract.Params
}

func (e *Event) Signature() string {
	return e.signature
}

func (e *Event) Name() string {
	return e.name
}

func (e *Event) Params() contract.Params {
	return e.params
}

func (e *Event) Format(f fmt.State, c rune) {
	switch c {
	case 'v', 's':
		if f.Flag('+') {
			fmt.Fprintf(f, "Event{blockHeight:%d,blockHash:%s,txHash:%s,txIndex:%d,indexInTx:%d,addr:%s,signature:%s,indexed:%d,name:%s,params:%v}",
				e.blockHeight, hex.EncodeToString(e.blockHash), hex.EncodeToString(e.txHash), e.txIndex, e.indexInTx,
				e.Address(), e.signature, e.indexed, e.name, e.params)
		} else {
			fmt.Fprintf(f, "Event{addr:%s,signature:%s,indexed:%d,params:%v}",
				e.Address(), e.signature, e.indexed, e.params)
		}
	}
}

type EventJson struct {
	BaseEvent *BaseEvent
	Signature string
	Name      string
	Params    contract.Params
}

func (e *Event) MarshalJSON() ([]byte, error) {
	v := EventJson{
		BaseEvent: e.BaseEvent,
		Signature: e.Signature(),
		Name:      e.Name(),
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
		name:      v.Name,
		params:    v.Params,
	}
	return nil
}

func NewEvent(spec contract.EventSpec, be *BaseEvent) (*Event, error) {
	params := make(contract.Params)
	for i, s := range spec.Inputs {
		var raw interface{}
		if i < spec.Indexed {
			raw = be.EventLog.Indexed[i+1]
		} else {
			raw = be.EventLog.Data[i-spec.Indexed]
		}
		v, err := decode(s.Type, raw)
		if err != nil {
			return nil, err
		}
		params[s.Name] = v
	}
	return &Event{
		BaseEvent: be,
		signature: spec.Signature,
		name:      spec.Name,
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
					EventIndexedValue: e.IndexedValue(idx).(EventIndexedValue),
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

func (f *EventFilter) Spec() contract.EventSpec {
	return f.spec
}

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

package contract

import (
	"context"
	"encoding/json"

	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"
)

type Params map[string]interface{}
type ReturnValue interface{}
type TxID interface{}
type BlockID interface{}
type TxResult interface {
	Success() bool
	Events() []BaseEvent
	Failure() interface{}
	BlockID() BlockID
	BlockHeight() int64
	TxID() TxID
}
type BaseEvent interface {
	Address() Address
	SignatureMatcher() SignatureMatcher
	Indexed() int
	IndexedValue(i int) EventIndexedValue
	BlockID() BlockID
	BlockHeight() int64
	TxID() TxID
	Identifier() int
}

type Event interface {
	BaseEvent
	Signature() string
	Name() string
	Params() Params
}

type EventFilter interface {
	Filter(event BaseEvent) (Event, error)
	Signature() string
	Address() Address
	Spec() EventSpec
}

type SignatureMatcher interface {
	Match(v string) bool
}

type EventIndexedValue interface {
	Match(v interface{}) bool
}

type EventIndexedValueWithParam interface {
	EventIndexedValue
	Spec() NameAndTypeSpec
	Param() interface{}
}

type EventCallback func(e Event) error
type Handler interface {
	Invoke(method string, params Params, options Options) (TxID, error)
	Call(method string, params Params, options Options) (ReturnValue, error)
	EventFilter(name string, params Params) (EventFilter, error)
	Spec() Spec
	Address() Address
	MonitorEvent(ctx context.Context, cb EventCallback, nameToParams map[string][]Params, height int64) error
}

type BaseEventCallback func(e BaseEvent) error

type BlockInfo interface {
	ID() BlockID
	Height() int64
	EqualID(id BlockID) (bool, error)
}

type FinalityMonitor interface {
	IsFinalized(height int64, id BlockID) (bool, error)
	HeightByID(id BlockID) (int64, error)
	Subscribe(size uint) FinalitySubscription
}

type FinalitySubscription interface {
	C() <-chan BlockInfo
	Unsubscribe()
	Serve(ctx context.Context, cb func(info BlockInfo) error) error
}

type FinalitySupplier interface {
	Latest() (BlockInfo, error)
	HeightByID(id BlockID) (int64, error)
	Serve(context.Context, BlockInfo, func(BlockInfo)) error
}

type Adaptor interface {
	NetworkType() string
	GetResult(id TxID) (TxResult, error)
	Handler(spec []byte, address Address) (Handler, error)
	FinalityMonitor() FinalityMonitor
	MonitorEvent(ctx context.Context, cb EventCallback, efs []EventFilter, height int64) error
	MonitorBaseEvent(ctx context.Context, cb BaseEventCallback, sigToAddrs map[string][]Address, height int64) error
}

type Options map[string]interface{}
type AdaptorFactory func(networkType string, endpoint string, opt Options, l log.Logger) (Adaptor, error)

var (
	afMap = make(map[string]AdaptorFactory)
)

func RegisterAdaptorFactory(cf AdaptorFactory, networkTypes ...string) {
	for _, networkType := range networkTypes {
		if _, ok := afMap[networkType]; ok {
			log.Panicln("already registered networkType:" + networkType)
		}
		afMap[networkType] = cf
	}
}

func NewAdaptor(networkType string, endpoint string, opt Options, l log.Logger) (Adaptor, error) {
	if cf, ok := afMap[networkType]; ok {
		l = l.WithFields(log.Fields{log.FieldKeyChain: networkType, log.FieldKeyModule: "contract"})
		return cf(networkType, endpoint, opt, l)
	}
	return nil, errors.New("not supported networkType:" + networkType)
}

type SpecFactory func(b []byte) (*Spec, error)

var (
	sfMap = make(map[string]SpecFactory)
)

func RegisterSpecFactory(sf SpecFactory, networkTypes ...string) {
	for _, networkType := range networkTypes {
		if _, ok := sfMap[networkType]; ok {
			log.Panicln("already registered networkType:" + networkType)
		}
		sfMap[networkType] = sf
	}
}

func NewSpec(networkType string, b []byte) (*Spec, error) {
	if sf, ok := sfMap[networkType]; ok {
		return sf(b)
	}
	return nil, errors.New("not supported networkType:" + networkType)
}

func MustNewSpec(networkType string, b []byte) *Spec {
	s, err := NewSpec(networkType, b)
	if err != nil {
		log.Panicf("fail to NewSpec err:%v", err)
	}
	return s
}

func EncodeOptions(v interface{}) (Options, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, errors.Wrapf(err, "fail to EncodeOptions, err:%s", err.Error())
	}
	options := make(Options)
	if err = json.Unmarshal(b, &options); err != nil {
		return nil, errors.Wrapf(err, "fail to EncodeOptions, err:%s", err.Error())
	}
	return options, nil
}

func DecodeOptions(options Options, v interface{}) error {
	b, err := json.Marshal(options)
	if err != nil {
		return errors.Wrapf(err, "fail to DecodeOptions err:%s", err.Error())
	}
	if err = json.Unmarshal(b, v); err != nil {
		return errors.Wrapf(err, "fail to DecodeOptions err:%s", err.Error())
	}
	return nil
}

type LogLevel log.Level

func (l LogLevel) Level() log.Level {
	return log.Level(l)
}
func (l LogLevel) MarshalJSON() ([]byte, error) {
	ll := log.Level(l)
	if ll > log.TraceLevel || ll < log.PanicLevel {
		return nil, errors.New("out of range log.Level")
	}
	return json.Marshal(ll.String())
}

func (l *LogLevel) UnmarshalJSON(input []byte) error {
	var str string
	err := json.Unmarshal(input, &str)
	if err != nil {
		return err
	}
	v, err := log.ParseLevel(str)
	if err != nil {
		return err
	}
	*l = LogLevel(v)
	return nil
}

func NewSignatureToAddressesMap(fs []EventFilter) map[string][]Address {
	sigToAddrs := make(map[string][]Address)
	for _, f := range fs {
		addrs, ok := sigToAddrs[f.Signature()]
		tAddr := f.Address()
		if !ok {
			addrs = make([]Address, 0)
		}
		exists := false
		for _, addr := range addrs {
			if addr == tAddr {
				exists = true
			}
		}
		if !exists {
			sigToAddrs[f.Signature()] = append(addrs, tAddr)
		}
	}
	return sigToAddrs
}

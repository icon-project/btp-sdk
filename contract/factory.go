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
	"encoding/json"
	"fmt"

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
	Revert() interface{}
}
type BaseEvent interface {
	Address() Address
	SignatureMatcher() SignatureMatcher
	Indexed() int
	IndexedValue(i int) EventIndexedValue
	BlockID() BlockID
	BlockHeight() int64
	TxID() TxID
	IndexInTx() int
}

type Event interface {
	BaseEvent
	Signature() string
	Params() Params
}

type EventFilter interface {
	Filter(event BaseEvent) (Event, error)
	Signature() string
	Address() Address
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

type EventCallback func(e Event)
type Handler interface {
	Invoke(method string, params Params, options Options) (TxID, error)
	Call(method string, params Params, options Options) (ReturnValue, error)
	EventFilter(name string, params Params) (EventFilter, error)
	Spec() Spec
	Address() Address
	MonitorEvent(cb EventCallback, nameToParams map[string][]Params, height int64) error
}

type BaseEventCallback func(e BaseEvent)

const (
	BlockStatusFinalized BlockStatus = iota
	BlockStatusDropped
	BlockStatusProposed
)

type BlockStatus int

func (b BlockStatus) String() string {
	switch b {
	case BlockStatusFinalized:
		return "Finalized"
	case BlockStatusDropped:
		return "Dropped"
	case BlockStatusProposed:
		return "Proposed"
	default:
		return fmt.Sprintf("Unknown %d", b)
	}
}

type BlockInfo interface {
	ID() []byte
	Height() int64
	Status() BlockStatus
}

type BlockMonitor interface {
	Start(height int64, finalizedOnly bool) (<-chan BlockInfo, error)
	Stop() error
	BlockInfo(height int64) BlockInfo
}

type Adaptor interface {
	GetResult(id TxID) (TxResult, error)
	Handler(spec []byte, address Address) (Handler, error)
	BlockMonitor() BlockMonitor
	MonitorEvent(cb EventCallback, efs []EventFilter, height int64) error
	MonitorBaseEvent(cb BaseEventCallback, sigToAddrs map[string][]Address, height int64) error
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
		return cf(networkType, endpoint, opt, l)
	}
	return nil, errors.New("not supported networkType:" + networkType)
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

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

	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"
)

type Params map[string]interface{}
type ReturnValue interface{}
type TxID interface{}
type TxResult interface {
	Success() bool
	Events() []BaseEvent
	Revert() interface{}
}
type BaseEvent interface {
	Address() Address
	Signature() EventSignature
	Indexed() int
	IndexedValue(i int) EventIndexedValue
}

type Event interface {
	BaseEvent
	Params() Params
}

type EventFilter interface {
	Filter(event BaseEvent) (Event, error)
	Signature() string
}

type EventSignature interface {
	Match(v string) bool
}

type EventIndexedValue interface {
	Match(v interface{}) bool
}

type Handler interface {
	Invoke(method string, params Params, options Options) (TxID, error)
	GetResult(id TxID) (TxResult, error)
	Call(method string, params Params, options Options) (ReturnValue, error)
	EventFilter(name string, params Params) (EventFilter, error)
	Spec() Spec
}

type Adaptor interface {
	Handler(spec []byte, address Address) (Handler, error)
}

type Options map[string]interface{}
type AdaptorFactory func(endpoint string, opt Options, l log.Logger) (Adaptor, error)

var (
	afMap = make(map[string]AdaptorFactory)
)

func RegisterAdaptorFactory(networkType string, cf AdaptorFactory) {
	if _, ok := afMap[networkType]; ok {
		log.Panicln("already registered networkType:" + networkType)
	}
	afMap[networkType] = cf
}

func NewAdaptor(networkType string, endpoint string, opt Options, l log.Logger) (Adaptor, error) {
	if cf, ok := afMap[networkType]; ok {
		return cf(endpoint, opt, l)
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

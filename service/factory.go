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

package service

import (
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
)

type Service interface {
	Name() string
	Invoke(network, method string, params contract.Params, options contract.Options) (contract.TxID, error)
	Call(network, method string, params contract.Params, options contract.Options) (contract.ReturnValue, error)
}

type Network struct {
	NetworkType string
	Adaptor     contract.Adaptor
	Options     contract.Options
}

type Factory func(map[string]Network, log.Logger) (Service, error)

var (
	fMap = make(map[string]Factory)
)

func RegisterFactory(serviceName string, sf Factory) {
	if _, ok := fMap[serviceName]; ok {
		log.Panicln("already registered serviceName:" + serviceName)
	}
	fMap[serviceName] = sf
}

func NewService(name string, networks map[string]Network, l log.Logger) (Service, error) {
	if f, ok := fMap[name]; ok {
		return f(networks, l)
	}
	return nil, errors.Errorf("not found service name:%s", name)
}

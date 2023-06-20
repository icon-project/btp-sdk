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

type DefaultServiceOptions struct {
	ContractAddress contract.Address
}

type DefaultService struct {
	name string
	m    map[string]contract.Handler
	l    log.Logger
}

func NewDefaultService(name string, networks map[string]Network, typeToSpec map[string][]byte, l log.Logger) (*DefaultService, error) {
	hMap := make(map[string]contract.Handler)
	for network, n := range networks {
		opt := &DefaultServiceOptions{}
		if err := contract.DecodeOptions(n.Options, &opt); err != nil {
			return nil, err
		}
		h, err := n.Adaptor.Handler(typeToSpec[n.NetworkType], opt.ContractAddress)
		if err != nil {
			return nil, err
		}
		hMap[network] = h
	}
	return &DefaultService{
		name: name,
		m:    hMap,
		l:    l,
	}, nil
}

func (s *DefaultService) Name() string {
	return s.name
}

func (s *DefaultService) handler(network string) (contract.Handler, error) {
	h, ok := s.m[network]
	if !ok {
		return nil, errors.Errorf("not found handler network:%s", network)
	}
	return h, nil
}

func (s *DefaultService) Invoke(network, method string, params contract.Params, options contract.Options) (contract.TxID, error) {
	h, err := s.handler(network)
	if err != nil {
		return nil, err
	}
	return h.Invoke(method, params, options)
}

func (s *DefaultService) Call(network, method string, params contract.Params, options contract.Options) (contract.ReturnValue, error) {
	h, err := s.handler(network)
	if err != nil {
		return nil, err
	}
	return h.Call(method, params, options)
}

type Handlers map[string]contract.Handler

func NewHandlers(a contract.Adaptor, addrToSpec map[contract.Address][]byte) (Handlers, error) {
	hs := make(Handlers)
	for addr, spec := range addrToSpec {
		h, err := a.Handler(spec, addr)
		if err != nil {
			return nil, err
		}
		for _, method := range h.Spec().Methods {
			if _, ok := hs[method.Name]; ok {
				return nil, errors.Errorf("duplicated method %s", method.Name)
			}
			hs[method.Name] = h
		}
	}
	return hs, nil
}

type MultiContractService struct {
	name string
	m    map[string]Handlers
	l    log.Logger
}

type MultiContractServiceOption map[string]contract.Address

const (
	MultiContractServiceOptionNameDefault = "default"
)

func getSpec(optToTypeToSpec map[string]map[string][]byte, optionName, networkType string) ([]byte, error) {
	typeToSpec, ok := optToTypeToSpec[optionName]
	if !ok {
		return nil, errors.Errorf("not found typeToSpecMap optionName:%s", optionName)
	}
	spec, ok := typeToSpec[networkType]
	if !ok {
		return nil, errors.Errorf("not found spec optionName:%s, networkType:%s", optionName, networkType)
	}
	return spec, nil
}

func NewMultiContractService(name string, networks map[string]Network, optToTypeToSpec map[string]map[string][]byte, l log.Logger) (*MultiContractService, error) {
	m := make(map[string]Handlers)
	for network, n := range networks {
		opt := make(MultiContractServiceOption)
		if err := contract.DecodeOptions(n.Options, &opt); err != nil {
			return nil, err
		}
		addrToSpec := make(map[contract.Address][]byte)
		for optionName, addr := range opt {
			spec, err := getSpec(optToTypeToSpec, optionName, n.NetworkType)
			if err != nil {
				return nil, err
			}
			addrToSpec[addr] = spec
		}
		hs, err := NewHandlers(n.Adaptor, addrToSpec)
		if err != nil {
			return nil, err
		}
		m[network] = hs
	}
	return &MultiContractService{
		name: name,
		m:    m,
		l:    l,
	}, nil
}

func (s *MultiContractService) Name() string {
	return s.name
}

func (s *MultiContractService) handler(network, method string) (contract.Handler, error) {
	hs, ok := s.m[network]
	if !ok {
		return nil, errors.Errorf("not found handler network:%s", network)
	}
	h, ok := hs[method]
	if !ok {
		return nil, errors.Errorf("not found method:%s", method)
	}
	return h, nil
}

func (s *MultiContractService) Invoke(network, method string, params contract.Params, options contract.Options) (contract.TxID, error) {
	h, err := s.handler(network, method)
	if err != nil {
		return nil, err
	}
	return h.Invoke(method, params, options)
}

func (s *MultiContractService) Call(network, method string, params contract.Params, options contract.Options) (contract.ReturnValue, error) {
	h, err := s.handler(network, method)
	if err != nil {
		return nil, err
	}
	return h.Call(method, params, options)
}

func FilterNetworksByTypes(networks map[string]Network, types [][]string) (ret []map[string]Network, rest map[string]Network) {
	ret = make([]map[string]Network, len(types))
	rest = make(map[string]Network)
	for i, l := range types {
		m := make(map[string]Network)
	netLoop:
		for netName, n := range networks {
			for _, nt := range l {
				if nt == n.NetworkType {
					m[netName] = n
					break netLoop
				}
			}
		}
		ret[i] = m
	}
	for netName, n := range networks {
		filtered := false
		for _, m := range ret {
			if _, ok := m[netName]; ok {
				filtered = true
				break
			}
		}
		if !filtered {
			rest[netName] = n
		}
	}
	return
}

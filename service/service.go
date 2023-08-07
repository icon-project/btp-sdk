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
	"context"

	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
)

type DefaultServiceOptions struct {
	ContractAddress contract.Address `json:"contract_address"`
}

type DefaultServiceNetwork struct {
	NetworkType string
	Adaptor     contract.Adaptor
	Options     DefaultServiceOptions
	Handler     contract.Handler
}
type DefaultService struct {
	name string
	nMap map[string]string
	m    map[string]DefaultServiceNetwork
	spec Spec
	l    log.Logger
}

func NewDefaultService(name string, networks map[string]Network, typeToSpec map[string][]byte, l log.Logger) (*DefaultService, error) {
	m := make(map[string]DefaultServiceNetwork)
	nMap := make(map[string]string)
	spec := NewSpec("", name)
	for network, n := range networks {
		opt := &DefaultServiceOptions{}
		if err := contract.DecodeOptions(n.Options, &opt); err != nil {
			return nil, err
		}
		h, err := n.Adaptor.Handler(typeToSpec[n.NetworkType], opt.ContractAddress)
		if err != nil {
			return nil, err
		}
		spec.Merge(h.Spec(), n.NetworkType)
		m[network] = DefaultServiceNetwork{
			NetworkType: n.NetworkType,
			Adaptor:     n.Adaptor,
			Options:     *opt,
			Handler:     h,
		}
		nMap[network] = n.NetworkType
	}
	return &DefaultService{
		name: name,
		m:    m,
		nMap: nMap,
		spec: spec,
		l:    l,
	}, nil
}

func (s *DefaultService) Name() string {
	return s.name
}

func (s *DefaultService) Networks() map[string]string {
	return s.nMap
}

func (s *DefaultService) Spec() Spec {
	return s.spec
}

func (s *DefaultService) network(network string) (DefaultServiceNetwork, error) {
	n, ok := s.m[network]
	if !ok {
		return n, errors.Errorf("not found network:%s", network)
	}
	return n, nil
}

func (s *DefaultService) Invoke(network, method string, params contract.Params, options contract.Options) (contract.TxID, error) {
	n, err := s.network(network)
	if err != nil {
		return nil, err
	}
	return n.Handler.Invoke(method, params, options)
}

func (s *DefaultService) Call(network, method string, params contract.Params, options contract.Options) (contract.ReturnValue, error) {
	n, err := s.network(network)
	if err != nil {
		return nil, err
	}
	return n.Handler.Call(method, params, options)
}

func (s *DefaultService) EventFilters(network string, nameToParams map[string][]contract.Params) ([]contract.EventFilter, error) {
	n, err := s.network(network)
	if err != nil {
		return nil, err
	}
	return EventFilters(nameToParams, func(k string) (contract.Handler, error) {
		return n.Handler, nil
	})
}

func (s *DefaultService) MonitorEvent(ctx context.Context, network string, cb contract.EventCallback, efs []contract.EventFilter, height int64) error {
	n, err := s.network(network)
	if err != nil {
		return err
	}
	return n.Adaptor.MonitorEvent(ctx, cb, efs, height)
}

type HandlerSupplier func(k string) (contract.Handler, error)

func EventFilters(nameToParams map[string][]contract.Params, hs HandlerSupplier) ([]contract.EventFilter, error) {
	var (
		efs = make([]contract.EventFilter, 0)
		h   contract.Handler
		err error
		ef  contract.EventFilter
	)
	for name, l := range nameToParams {
		if h, err = hs(name); err != nil {
			return nil, err
		}
		if len(l) == 0 {
			l = []contract.Params{nil}
		}
		for _, params := range l {
			if ef, err = h.EventFilter(name, params); err != nil {
				return nil, err
			}
			efs = append(efs, ef)
		}
	}
	return efs, nil
}

type MultiContractServiceNetwork struct {
	NetworkType    string
	Adaptor        contract.Adaptor
	Options        MultiContractServiceOption
	NameToHandlers map[string]contract.Handler
}

func (n *MultiContractServiceNetwork) Handler(methodOrEvent string) (contract.Handler, error) {
	h, ok := n.NameToHandlers[methodOrEvent]
	if !ok {
		return nil, errors.Errorf("not found handler methodOrEvent:%s", methodOrEvent)
	}
	return h, nil
}

type MultiContractService struct {
	name string
	nMap map[string]string
	m    map[string]MultiContractServiceNetwork
	spec Spec
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
	m := make(map[string]MultiContractServiceNetwork)
	ss := NewSpec("", name)
	nMap := make(map[string]string)
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
		nameToHandlers := make(map[string]contract.Handler)
		for addr, spec := range addrToSpec {
			h, err := n.Adaptor.Handler(spec, addr)
			if err != nil {
				return nil, err
			}
			for _, method := range h.Spec().Methods {
				if _, ok := nameToHandlers[method.Name]; ok {
					return nil, errors.Errorf("duplicated method %s", method.Name)
				}
				nameToHandlers[method.Name] = h
			}
			for _, event := range h.Spec().Events {
				if _, ok := nameToHandlers[event.Name]; ok {
					return nil, errors.Errorf("duplicated event %s", event.Name)
				}
				nameToHandlers[event.Name] = h
			}
			ss.Merge(h.Spec(), n.NetworkType)
		}
		m[network] = MultiContractServiceNetwork{
			NetworkType:    n.NetworkType,
			Adaptor:        n.Adaptor,
			Options:        opt,
			NameToHandlers: nameToHandlers,
		}
		nMap[network] = n.NetworkType
	}
	return &MultiContractService{
		name: name,
		nMap: nMap,
		m:    m,
		spec: ss,
		l:    l,
	}, nil
}

func (s *MultiContractService) Name() string {
	return s.name
}

func (s *MultiContractService) Networks() map[string]string {
	return s.nMap
}

func (s *MultiContractService) Spec() Spec {
	return s.spec
}

func (s *MultiContractService) network(network string) (MultiContractServiceNetwork, error) {
	n, ok := s.m[network]
	if !ok {
		return n, errors.Errorf("not found handler network:%s", network)
	}
	return n, nil
}

func (s *MultiContractService) handler(network, methodOrEvent string) (contract.Handler, error) {
	n, err := s.network(network)
	if err != nil {
		return nil, err
	}
	return n.Handler(methodOrEvent)
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

func (s *MultiContractService) EventFilters(network string, nameToParams map[string][]contract.Params) ([]contract.EventFilter, error) {
	n, err := s.network(network)
	if err != nil {
		return nil, err
	}
	return EventFilters(nameToParams, n.Handler)
}

func (s *MultiContractService) MonitorEvent(ctx context.Context, network string, cb contract.EventCallback, efs []contract.EventFilter, height int64) error {
	n, err := s.network(network)
	if err != nil {
		return err
	}
	return n.Adaptor.MonitorEvent(ctx, cb, efs, height)
}

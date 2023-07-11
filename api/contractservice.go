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

package api

import (
	"context"
	"fmt"

	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/service"
)

type ContractService struct {
	name    string
	network string
	a       contract.Adaptor
	h       contract.Handler
	spec    service.Spec
	l       log.Logger
}

func NewContractService(a contract.Adaptor, spec []byte, address contract.Address, network string, l log.Logger) (service.Service, error) {
	h, err := a.Handler(spec, address)
	if err != nil {
		return nil, err
	}
	name := ContractServiceName(network, address)
	ss := service.NewSpec("", name)
	ss.Merge(h.Spec())
	return &ContractService{
		name:    name,
		network: network,
		a:       a,
		h:       h,
		spec:    ss,
		l:       l}, nil
}

func ContractServiceName(network string, address contract.Address) string {
	return fmt.Sprintf("%s|%s", network, address)
}

func (s *ContractService) Name() string {
	return s.name
}

func (s *ContractService) Spec() service.Spec {
	return s.spec
}

func (s *ContractService) Invoke(network, method string, params contract.Params, options contract.Options) (contract.TxID, error) {
	if network != s.network {
		return nil, errors.Errorf("not supported network:%s", network)
	}
	return s.h.Invoke(method, params, options)
}

func (s *ContractService) Call(network, method string, params contract.Params, options contract.Options) (contract.ReturnValue, error) {
	if network != s.network {
		return nil, errors.Errorf("not supported network:%s", network)
	}
	return s.h.Call(method, params, options)
}

func (s *ContractService) EventFilters(network string, nameToParams map[string][]contract.Params) ([]contract.EventFilter, error) {
	if network != s.network {
		return nil, errors.Errorf("not supported network:%s", network)
	}
	return service.EventFilters(nameToParams, func(k string) (contract.Handler, error) {
		return s.h, nil
	})
}

func (s *ContractService) MonitorEvent(ctx context.Context, network string, cb contract.EventCallback, efs []contract.EventFilter, height int64) error {
	if network != s.network {
		return errors.Errorf("not supported network:%s", network)
	}
	return s.a.MonitorEvent(ctx, cb, efs, height)
}

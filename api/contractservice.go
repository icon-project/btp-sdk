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
	"fmt"

	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/service"
)

type ContractService struct {
	name string
	h    contract.Handler
	l    log.Logger
}

func NewContractService(a contract.Adaptor, spec []byte, address contract.Address, network string, l log.Logger) (service.Service, error) {
	h, err := a.Handler(spec, address)
	if err != nil {
		return nil, err
	}
	return &ContractService{
		name: ContractServiceName(network, address),
		h:    h,
		l:    l}, nil
}

func ContractServiceName(network string, address contract.Address) string {
	return fmt.Sprintf("%s|%s", network, address)
}

func (s *ContractService) Name() string {
	return s.name
}

func (s *ContractService) Invoke(network, method string, params contract.Params, options contract.Options) (contract.TxID, error) {
	return s.h.Invoke(method, params, options)
}

func (s *ContractService) Call(network, method string, params contract.Params, options contract.Options) (contract.ReturnValue, error) {
	return s.h.Call(method, params, options)
}

func (s *ContractService) MonitorEvent(network string, cb contract.EventCallback, nameToParams map[string][]contract.Params, height int64) error {
	return s.h.MonitorEvent(cb, nameToParams, height)
}

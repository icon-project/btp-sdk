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

package bmc

import (
	"encoding/base64"

	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/contract/eth"
	"github.com/icon-project/btp-sdk/contract/icon"
	"github.com/icon-project/btp-sdk/service"
)

const (
	ServiceName                        = "bmc"
	MultiContractServiceOptionNameBMCM = "bmcm"
)

var (
	optToTypeToSpec = map[string]map[string][]byte{
		service.MultiContractServiceOptionNameDefault: {
			icon.NetworkTypeIcon: []byte(iconContractSpec),
			eth.NetworkTypeEth:   []byte(ethContractSpec),
			eth.NetworkTypeEth2:  []byte(ethContractSpec),
		},
		MultiContractServiceOptionNameBMCM: {
			eth.NetworkTypeEth:  []byte(ethBMCMContractSpec),
			eth.NetworkTypeEth2: []byte(ethBMCMContractSpec),
		},
	}

	iconSpec = contract.MustNewSpec(icon.NetworkTypeIcon, []byte(iconContractSpec))
	ethSpec  = contract.MustNewSpec(eth.NetworkTypeEth, []byte(ethContractSpec))

	mergeInfos = map[string]service.MergeInfo{
		"getServices": {
			Flag: service.MethodOverloadOutput,
			Spec: iconSpec,
		},
		"getVerifiers": {
			Flag: service.MethodOverloadOutput,
			Spec: iconSpec,
		},
		"getRoutes": {
			Flag: service.MethodOverloadOutput,
			Spec: iconSpec,
		},
		"getStatus": {
			Flag: service.MethodOverloadOutput,
			Spec: iconSpec,
		},
		"handleRelayMessage": {
			Flag: service.MethodOverloadInputs,
			Spec: ethSpec,
		},
	}
)

func init() {
	service.RegisterFactory(ServiceName, NewService)
}

type ServiceOptions struct {
	ContractAddress contract.Address
}

type Service struct {
	*service.MultiContractService
	spec service.Spec
	l    log.Logger
}

func NewService(networks map[string]service.Network, l log.Logger) (service.Service, error) {
	s, err := service.NewMultiContractService(ServiceName, networks, optToTypeToSpec, l)
	if err != nil {
		return nil, err
	}
	ss := service.CopySpec(s.Spec())
	if err = ss.MergeOverloads(mergeInfos); err != nil {
		return nil, err
	}
	return &Service{
		MultiContractService: s,
		spec:                 ss,
		l:                    l,
	}, nil
}

func (s *Service) Spec() service.Spec {
	return s.spec
}

func (s *Service) Invoke(network, method string, params contract.Params, options contract.Options) (contract.TxID, error) {
	switch method {
	case "handleRelayMessage":
		b, err := contract.BytesOf(params["_msg"])
		if err != nil {
			return nil, err
		}
		params["_msg"] = base64.StdEncoding.EncodeToString(b)
	}
	return s.MultiContractService.Invoke(network, method, params, options)
}

func (s *Service) Call(network, method string, params contract.Params, options contract.Options) (contract.ReturnValue, error) {
	ret, err := s.MultiContractService.Call(network, method, params, options)
	if err != nil {
		return nil, err
	}
	switch method {
	case "getServices", "getVerifiers", "getRoutes":
		if l, ok := ret.([]contract.Struct); ok {
			m := make(map[string]interface{})
			for _, st := range l {
				k, err := contract.StringOf(st.Fields[0].Value)
				if err != nil {
					return nil, err
				}
				m[string(k)] = st.Fields[1].Value
			}
			return m, nil
		}
	case "getStatus":
		if st, ok := ret.(contract.Struct); ok {
			to := s.spec.Methods[method].Output.Resolved
			t := make(contract.Params)
			for i, f := range st.Fields {
				t[to.Fields[i].Name] = f.Value
			}
			return contract.StructOfWithSpec(*to, t)
		}
	}
	return ret, nil
}

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
	*service.MergedService
	l log.Logger
}

func NewService(networks map[string]service.Network, l log.Logger) (service.Service, error) {
	s, err := service.NewMultiContractService(ServiceName, networks, optToTypeToSpec, l)
	if err != nil {
		return nil, err
	}
	ms, err := service.NewMergedService(s, &mergeHandler{l: l}, l)
	if err != nil {
		return nil, err
	}
	return &Service{
		MergedService: ms,
		l:             l,
	}, nil
}

type mergeHandler struct {
	l log.Logger
}

func (h *mergeHandler) MergeMethodOverloads(s *service.MethodSpec) (bool, error) {
	switch s.Name {
	case "getServices", "getVerifiers", "getRoutes", "getStatus":
		return true, s.MergeOverloads(iconSpec.MethodMap[s.Name], service.MethodOverloadOutput)
	case "handleRelayMessage":
		return true, s.MergeOverloads(ethSpec.MethodMap[s.Name], service.MethodOverloadInputs)
	}
	return false, nil
}

func (h *mergeHandler) MergeEventOverloads(s *service.EventSpec) (bool, error) {
	return false, nil
}

func (h *mergeHandler) HandleMethodInputs(name string, org, merged map[string]*contract.NameAndTypeSpec, params contract.Params) (contract.Params, error) {
	switch name {
	case "handleRelayMessage":
		inputName := "_msg"
		b, err := contract.BytesOf(params[inputName])
		if err != nil {
			return nil, err
		}
		params[inputName] = base64.StdEncoding.EncodeToString(b)
	}
	return params, nil
}

func (h *mergeHandler) HandleMethodOutput(name string, org, merged contract.TypeSpec, r contract.ReturnValue) (contract.ReturnValue, error) {
	switch name {
	case "getServices", "getVerifiers", "getRoutes":
		if l, ok := r.([]contract.Struct); ok {
			ret := make(contract.Params)
			for _, st := range l {
				k, err := contract.StringOf(st.Fields[0].Value)
				if err != nil {
					return nil, err
				}
				ret[string(k)] = st.Fields[1].Value
			}
			return ret, nil
		}
	case "getStatus":
		if st, ok := r.(contract.Struct); ok {
			to := merged.Resolved
			t := make(contract.Params)
			for i, f := range st.Fields {
				t[to.Fields[i].Name] = f.Value
			}
			return contract.StructOfWithSpec(*to, t)
		}
	}
	return r, nil
}

func (h *mergeHandler) HandleEventFilterParams(s *service.EventSpec, o *service.EventOverload, l []contract.Params) ([]contract.Params, error) {
	return l, nil
}

func (h *mergeHandler) HandleEvent(s *service.EventSpec, o *service.EventOverload, e contract.Event) error {
	return nil
}

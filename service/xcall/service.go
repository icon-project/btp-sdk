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

package xcall

import (
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract/eth"
	"github.com/icon-project/btp-sdk/contract/icon"
	"github.com/icon-project/btp-sdk/service"
)

const (
	ServiceName = "xcall"
)

var (
	typeToSpec = map[string][]byte{
		icon.NetworkTypeIcon: []byte(iconContractSpec),
		eth.NetworkTypeEth:   []byte(ethContractSpec),
		eth.NetworkTypeEth2:  []byte(ethContractSpec),
	}
)

func init() {
	service.RegisterFactory(ServiceName, NewService)
}

type Service struct {
	*service.DefaultService
	l log.Logger
}

func NewService(networks map[string]service.Network, l log.Logger) (service.Service, error) {
	s, err := service.NewDefaultService(ServiceName, networks, typeToSpec, l)
	if err != nil {
		return nil, err
	}
	return &Service{
		DefaultService: s,
		l:              l,
	}, nil
}

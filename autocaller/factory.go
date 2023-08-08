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

package autocaller

import (
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/service"
)

type AutoCaller interface {
	Name() string
	Tasks() []string
	Start() error
	Stop() error
	Find(FindParam) ([]interface{}, error) //TODO support Pagination
}

type FindParam struct {
	Task string `json:"task" query:"task"`
	//TODO condition with common fields {Network,SendTxID,Sent}
	//TODO condition with fields map[string]interface{}
	//TODO support Pagination with Pageable
}

type Network struct {
	NetworkType string
	Adaptor     contract.Adaptor
	Signer      service.Signer
	Options     contract.Options
}

type Factory func(service.Service, map[string]Network, log.Logger) (AutoCaller, error)

var (
	fMap = make(map[string]Factory)
)

func RegisterFactory(serviceName string, sf Factory) {
	if _, ok := fMap[serviceName]; ok {
		log.Panicln("already registered autocaller:" + serviceName)
	}
	fMap[serviceName] = sf
	log.Tracef("RegisterFactory autocaller:%s", serviceName)
}

func NewAutoCaller(name string, s service.Service, networks map[string]Network, l log.Logger) (AutoCaller, error) {
	if f, ok := fMap[name]; ok {
		l = l.WithFields(log.Fields{log.FieldKeyChain: name, log.FieldKeyModule: "autocaller"})
		return f(s, networks, l)
	}
	return nil, errors.Errorf("not found autocaller name:%s", name)
}

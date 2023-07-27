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
	"context"
	"os"
	"testing"
	"time"

	"github.com/icon-project/btp2/common/log"
	"github.com/icon-project/btp2/common/wallet"
	"github.com/stretchr/testify/assert"

	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/contract/eth"
	"github.com/icon-project/btp-sdk/contract/icon"
	"github.com/icon-project/btp-sdk/service"
)

const (
	networkIconTest = "icon_test"
	networkEth2Test = "eth2_test"
)

var (
	configs = map[string]TestConfig{
		networkIconTest: {
			Endpoint: "http://localhost:9080/api/v3/icon_dex",
			Wallet: MustLoadWallet(
				"../../example/javascore/src/test/resources/keystore.json",
				"../../example/javascore/src/test/resources/keysecret"),
			NetworkType: icon.NetworkTypeIcon,
			AdaptorOption: icon.AdaptorOption{
				NetworkID: "0x3",
			},
			ServiceOption: service.MultiContractServiceOption{
				service.MultiContractServiceOptionNameDefault: "cx2093dd8134f26df11aa928c86b6f5bac64a1cf83",
			},
		},
		networkEth2Test: {
			Endpoint: "http://localhost:8545",
			Wallet: MustLoadWallet(
				"../../example/solidity/test/keystore.json",
				"../../example/solidity/test/keysecret"),
			//NetworkType: eth.NetworkTypeEth,
			//AdaptorOption: eth.AdaptorOption{
			//	BlockMonitor: MustEncodeOptions(eth.BlockMonitorOptions{
			//		FinalizeBlockCount: 3,
			//	}),
			//},
			NetworkType: eth.NetworkTypeEth2,
			AdaptorOption: eth.AdaptorOption{
				BlockMonitor: MustEncodeOptions(eth.Eth2BlockMonitorOptions{
					Endpoint: "http://localhost:9596",
				}),
			},
			ServiceOption: service.MultiContractServiceOption{
				service.MultiContractServiceOptionNameDefault: "0xDc64a140Aa3E981100a9becA4E685f962f0cF6C9",
				MultiContractServiceOptionNameBMCM:            "0x5FbDB2315678afecb367f032d93F642f64180aa3",
			},
		},
	}

	nameAndParams = map[string][]contract.Params{
		"BTPEvent": {
			{"_src": contract.String("0x3.icon")},
			{"_src": contract.String("0x25d6efd.eth2")},
		},
	}
)

type TestConfig struct {
	NetworkType   string
	Endpoint      string
	Wallet        wallet.Wallet
	AdaptorOption interface{}
	ServiceOption service.MultiContractServiceOption
}

func MustReadFile(f string) []byte {
	b, err := os.ReadFile(f)
	if err != nil {
		log.Panicf("fail to ReadFile err:%+v", err)
	}
	return b
}

func MustLoadWallet(keyStoreFile, keyStoreSecret string) wallet.Wallet {
	w, err := wallet.DecryptKeyStore(MustReadFile(keyStoreFile), MustReadFile(keyStoreSecret))
	if err != nil {
		log.Panicf("keyStoreFile:%s, keyStoreSecret:%s, %+v",
			keyStoreFile, keyStoreSecret, err)
	}
	return w
}

func MustEncodeOptions(v interface{}) contract.Options {
	opt, err := contract.EncodeOptions(v)
	if err != nil {
		log.Panicf("%+v", err)
	}
	return opt
}

func adaptor(t *testing.T, networkType string) contract.Adaptor {
	config := configs[networkType]
	opt, err := contract.EncodeOptions(config.AdaptorOption)
	if err != nil {
		assert.FailNow(t, "fail to EncodeOptions", err)
	}
	l := log.GlobalLogger()
	a, err := contract.NewAdaptor(config.NetworkType, config.Endpoint, opt, l)
	if err != nil {
		assert.FailNow(t, "fail to NewAdaptor", err)
	}
	return a
}

func Test_Service(t *testing.T) {
	networks := make(map[string]service.Network)
	for network, config := range configs {
		networks[network] = service.Network{
			NetworkType: config.NetworkType,
			Adaptor:     adaptor(t, network),
			Options:     MustEncodeOptions(config.ServiceOption),
		}
	}
	l := log.GlobalLogger()
	s, err := service.NewService(ServiceName, networks, l)
	if err != nil {
		assert.FailNow(t, "fail to NewService", err)
	}
	assert.Equal(t, ServiceName, s.Name())
	var r contract.ReturnValue
	for network := range networks {
		for _, name := range []string{"getServices", "getVerifiers", "getRoutes"} {
			r, err = s.Call(network, name, nil, nil)
			assert.NoError(t, err)
			assert.IsType(t, contract.Params{}, r)
			t.Logf("%T %+v", r, r)
		}

		r, err = s.Call(network, "getLinks", nil, nil)
		assert.NoError(t, err)
		t.Logf("%T %+v", r, r)

		r, err = s.Call(network, "getStatus", contract.Params{"_link": r.([]contract.String)[0]}, nil)
		assert.NoError(t, err)
		_, err = contract.StructOfWithSpec(*iconSpec.MethodMap["getStatus"].Output.Resolved, r)
		assert.NoError(t, err)
		t.Logf("%T %+v", r, r)
	}
}

func handler(t *testing.T, a contract.Adaptor, config TestConfig) contract.Handler {
	spec := optToTypeToSpec[service.MultiContractServiceOptionNameDefault][config.NetworkType]
	addr := config.ServiceOption[service.MultiContractServiceOptionNameDefault]
	h, err := a.Handler(spec, addr)
	if err != nil {
		assert.FailNow(t, "fail to NewHandler", err)
	}
	return h
}

func eventFilters(h contract.Handler, nameAndParams map[string][]contract.Params) ([]contract.EventFilter, error) {
	efs := make([]contract.EventFilter, 0)
	for _, s := range h.Spec().Events {
		if ps, ok := nameAndParams[s.Name]; len(nameAndParams) == 0 || ok {
			if len(ps) == 0 {
				ps = []contract.Params{nil}
			}
			for _, p := range ps {
				ef, err := h.EventFilter(s.Name, p)
				if err != nil {
					return nil, err
				}
				efs = append(efs, ef)
			}
		}
	}
	return efs, nil
}

func Test_AdaptorMonitorEvents(t *testing.T) {
	args := []struct {
		networkType string
		height      int64
	}{
		{
			networkType: icon.NetworkTypeIcon,
			height:      1050779,
		},
		{
			networkType: eth.NetworkTypeEth,
			height:      168136,
		},
	}
	for _, arg := range args {
		config := configs[arg.networkType]
		a := adaptor(t, arg.networkType)
		h := handler(t, a, config)
		efs, err := eventFilters(h, nameAndParams)
		if err != nil {
			assert.FailNow(t, "fail to get EventFilter err:%s", err.Error())
		}
		go func(nt string, height int64) {
			logger := log.GlobalLogger().WithFields(log.Fields{log.FieldKeyChain: nt})
			err = a.MonitorEvent(context.Background(), func(e contract.Event) error {
				logger.Infof("%s: %+v, src:%v", nt, e, TryActualParam(e, "_src"))
				return nil
			}, efs, height)
			assert.NoError(t, err)
		}(arg.networkType, arg.height)
	}
	<-time.After(10 * time.Minute)
}

func TryActualParam(e contract.Event, name string) interface{} {
	p := e.Params()[name]
	if eivp, ok := p.(contract.EventIndexedValueWithParam); ok {
		return eivp.Param()
	}
	return p
}

func Test_HandlerMonitorEvent(t *testing.T) {
	args := []struct {
		networkType string
		height      int64
	}{
		{
			networkType: icon.NetworkTypeIcon,
			height:      1050779,
		},
		{
			networkType: eth.NetworkTypeEth,
			height:      168136,
		},
	}
	for _, arg := range args {
		config := configs[arg.networkType]
		a := adaptor(t, arg.networkType)
		h := handler(t, a, config)
		go func(nt string, height int64) {
			logger := log.GlobalLogger().WithFields(log.Fields{log.FieldKeyChain: nt})
			err := h.MonitorEvent(context.Background(), func(e contract.Event) error {
				logger.Infof("%s: %+v", nt, e)
				return nil
			}, nameAndParams, height)
			assert.NoError(t, err)
		}(arg.networkType, arg.height)
	}
	<-time.After(10 * time.Minute)
}

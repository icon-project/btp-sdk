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
	"os"
	"testing"

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
			NetworkType: eth.NetworkTypeEth2,
			AdaptorOption: eth.AdaptorOption{
				FinalityMonitor: MustEncodeOptions(eth.FinalityMonitorOptions{
					PollingPeriodSec: 3,
				}),
			},
			ServiceOption: service.MultiContractServiceOption{
				service.MultiContractServiceOptionNameDefault: "0xDc64a140Aa3E981100a9becA4E685f962f0cF6C9",
				MultiContractServiceOptionNameBMCM:            "0x5FbDB2315678afecb367f032d93F642f64180aa3",
			},
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

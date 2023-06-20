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
			ServiceOption: service.DefaultServiceOptions{
				ContractAddress: "cx784b438d386170c24a12d4de3df9809d066b6258",
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
			ServiceOption: service.DefaultServiceOptions{
				ContractAddress: "0x0DCd1Bf9A1b36cE34237eEaFef220932846BCD82",
			},
		},
	}
)

type TestConfig struct {
	NetworkType   string
	Endpoint      string
	Wallet        wallet.Wallet
	AdaptorOption interface{}
	ServiceOption service.DefaultServiceOptions
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

func adaptor(t *testing.T, network string) contract.Adaptor {
	config := configs[network]
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
	s, err := NewService(networks, l)
	if err != nil {
		assert.FailNow(t, "fail to NewService", err)
	}
	assert.Equal(t, ServiceName, s.Name())
	//s.Invoke()
}

func handler(t *testing.T, a contract.Adaptor, config TestConfig) contract.Handler {
	spec := typeToSpec[config.NetworkType]
	addr := config.ServiceOption.ContractAddress
	h, err := a.Handler(spec, addr)
	if err != nil {
		assert.FailNow(t, "fail to NewHandler", err)
	}
	return h
}

func eventFilters(h contract.Handler) ([]contract.EventFilter, error) {
	efs := make([]contract.EventFilter, 0)
	for _, s := range h.Spec().Events {
		ef, err := h.EventFilter(s.Name, nil)
		if err != nil {
			return nil, err
		}
		efs = append(efs, ef)
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
			height:      968789,
		},
		{
			networkType: eth.NetworkTypeEth,
			height:      113534,
		},
	}
	for _, arg := range args {
		config := configs[arg.networkType]
		a := adaptor(t, arg.networkType)
		h := handler(t, a, config)
		efs, err := eventFilters(h)
		if err != nil {
			assert.FailNow(t, "fail to get EventFilter err:%s", err.Error())
		}
		go func(nt string, height int64) {
			logger := log.GlobalLogger().WithFields(log.Fields{log.FieldKeyModule: nt})
			err = a.MonitorEvent(func(e contract.Event) {
				logger.Infof("%s: %+v", nt, e)
			}, efs, height)
		}(arg.networkType, arg.height)
	}
	<-time.After(1 * time.Minute)
}

func Test_HandlerMonitorEvent(t *testing.T) {
	networkTypes := []string{
		icon.NetworkTypeIcon,
		eth.NetworkTypeEth,
	}
	for _, nt := range networkTypes {
		config := configs[nt]
		a := adaptor(t, nt)
		h := handler(t, a, config)
		err := h.MonitorEvent(func(e contract.Event) {
			t.Logf("%+v", e)
		}, map[string][]contract.Params{"CallMessageSent": nil}, 798513)
		assert.NoError(t, err)
	}
}

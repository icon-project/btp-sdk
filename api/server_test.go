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
	"github.com/icon-project/btp-sdk/service/bmc"
	"github.com/icon-project/btp-sdk/service/xcall"
)

const (
	serviceTest       = "test"
	serverAddress     = "localhost:8080"
	networkIconTest   = "icon_test"
	networkEth2Test   = "eth2_test"
	transportLogLevel = log.DebugLevel
	serverLogLevel    = log.DebugLevel
	clientLogLevel    = log.DebugLevel
)

var (
	byteVal       = contract.Integer("0x7f")
	shortVal      = contract.Integer("0x7fff")
	intVal        = contract.Integer("0x7fffffff")
	longVal       = contract.Integer("0x7fffffffffffffff")
	bigIntegerVal = contract.Integer("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
	int24Val      = contract.Integer("0x7fffff")
	int40Val      = contract.Integer("0x7fffffffff")
	int72Val      = contract.Integer("0x7fffffffffffffffff")
	charVal       = contract.Integer("0xffff")
	booleanVal    = contract.Boolean(true)
	stringVal     = contract.String("string")
	bytesVal      = contract.Bytes("bytes")
	addressVal    = map[string]contract.Address{
		icon.NetworkTypeIcon: "hx0000000000000000000000000000000000000000",
		eth.NetworkTypeEth:   "0x0000000000000000000000000000000000000000",
		eth.NetworkTypeEth2:  "0x0000000000000000000000000000000000000000",
	}
	structVal = struct {
		BooleanVal contract.Boolean `json:"booleanVal"`
	}{booleanVal}
)

var (
	configs = map[string]TestConfig{
		networkIconTest: {
			Endpoint: "http://localhost:9080/api/v3/icon_dex",
			NetworkType: icon.NetworkTypeIcon,
			AdaptorOption: icon.AdaptorOption{
				NetworkID:         "0x3",
				TransportLogLevel: contract.LogLevel(transportLogLevel),
			},
			ServiceOptions: map[string]contract.Options{
				xcall.ServiceName: MustEncodeOptions(service.DefaultServiceOptions{
					ContractAddress: "cx784b438d386170c24a12d4de3df9809d066b6258",
				}),
				bmc.ServiceName: MustEncodeOptions(service.MultiContractServiceOption{
					service.MultiContractServiceOptionNameDefault: "cx2093dd8134f26df11aa928c86b6f5bac64a1cf83",
				}),
				serviceTest: MustEncodeOptions(service.DefaultServiceOptions{
					ContractAddress: "cxbdb2fac53eaf445f9f0d0c33306d6b2a1a25353d",
				}),
			},
		},
		networkEth2Test: {
			Endpoint: "http://localhost:8545",
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
				TransportLogLevel: contract.LogLevel(transportLogLevel),
			},
			ServiceOptions: map[string]contract.Options{
				xcall.ServiceName: MustEncodeOptions(service.DefaultServiceOptions{
					ContractAddress: "0x0DCd1Bf9A1b36cE34237eEaFef220932846BCD82",
				}),
				bmc.ServiceName: MustEncodeOptions(service.MultiContractServiceOption{
					service.MultiContractServiceOptionNameDefault: "0xDc64a140Aa3E981100a9becA4E685f962f0cF6C9",
					bmc.MultiContractServiceOptionNameBMCM:        "0x5FbDB2315678afecb367f032d93F642f64180aa3",
				}),
				serviceTest: MustEncodeOptions(service.DefaultServiceOptions{
					ContractAddress: "0x09635F643e140090A9A8Dcd712eD6285858ceBef",
				}),
			},
		},
	}
	signers = map[string]service.Signer{
		networkIconTest: service.NewDefaultSigner(
			MustLoadWallet("../example/javascore/src/test/resources/keystore.json", "../example/javascore/src/test/resources/keysecret"),
			icon.NetworkTypeIcon),
		networkEth2Test: service.NewDefaultSigner(
			MustLoadWallet("../example/solidity/test/keystore.json", "../example/solidity/test/keysecret"),
			eth.NetworkTypeEth2),
	}
	contracts = map[string]Contract{
		networkIconTest: {
			Spec:    MustReadFile("../example/javascore/build/generated/contractSpec.json"),
			Address: "cxbdb2fac53eaf445f9f0d0c33306d6b2a1a25353d",
		},
		networkEth2Test: {
			Spec:    MustReadFile("../example/solidity/artifacts/contracts/HelloWorld.sol/HelloWorld.abi.json"),
			Address: "0x09635F643e140090A9A8Dcd712eD6285858ceBef",
		},
	}
)

type TestConfig struct {
	NetworkType    string
	Endpoint       string
	AdaptorOption  interface{}
	ServiceOptions map[string]contract.Options
}

type Contract struct {
	Spec    []byte
	Address contract.Address
}

func MustLoadWallet(keyStoreFile, keyStoreSecret string) wallet.Wallet {
	w, err := wallet.DecryptKeyStore(MustReadFile(keyStoreFile), MustReadFile(keyStoreSecret))
	if err != nil {
		log.Panicf("keyStoreFile:%s, keyStoreSecret:%s, %+v",
			keyStoreFile, keyStoreSecret, err)
	}
	return w
}

func MustReadFile(f string) []byte {
	b, err := os.ReadFile(f)
	if err != nil {
		log.Panicf("fail to ReadFile err:%+v", err)
	}
	return b
}

func MustEncodeOptions(v interface{}) contract.Options {
	opt, err := contract.EncodeOptions(v)
	if err != nil {
		log.Panicf("%+v", err)
	}
	return opt
}

func server(t *testing.T, withSignerService bool) *Server {
	l := log.GlobalLogger()
	s := NewServer(serverAddress, serverLogLevel, l)
	svcToNetworks := make(map[string]map[string]service.Network)
	for network, config := range configs {
		opt, err := contract.EncodeOptions(config.AdaptorOption)
		if err != nil {
			assert.FailNow(t, "fail to EncodeOptions", err)
		}
		a, err := contract.NewAdaptor(config.NetworkType, config.Endpoint, opt, l)
		if err != nil {
			assert.FailNow(t, "fail to NewAdaptor", err)
		}
		s.AddAdaptor(network, a)
		for name, so := range config.ServiceOptions {
			networks, ok := svcToNetworks[name]
			if !ok {
				networks = make(map[string]service.Network)
				svcToNetworks[name] = networks
			}
			networks[network] = service.Network{
				NetworkType: config.NetworkType,
				Adaptor:     a,
				Options:     so,
			}
		}
	}
	for name, networks := range svcToNetworks {
		if name == serviceTest {
			typeToSpec := make(map[string][]byte)
			for network, n := range networks {
				typeToSpec[n.NetworkType] = contracts[network].Spec
			}
			service.RegisterFactory(name, func(networks map[string]service.Network, l log.Logger) (service.Service, error) {
				return service.NewDefaultService(name, networks, typeToSpec, l)
			})
		}
		svc, err := service.NewService(name, networks, l)
		if err != nil {
			assert.FailNow(t, "fail to NewService", err)
		}
		if withSignerService {
			if svc, err = service.NewSignerService(svc, signers, l); err != nil {
				assert.FailNow(t, "fail to NewSignerService", err)
			}
		}
		s.AddService(svc)
	}
	return s
}

func client() *Client {
	networkToType := make(map[string]string)
	for network, config := range configs {
		networkToType[network] = config.NetworkType
	}
	return NewClient(fmt.Sprintf("http://%s", serverAddress), networkToType, clientLogLevel, log.GlobalLogger())
}

func Test_ServerCall(t *testing.T) {
	s := server(t, false)
	go func() {
		err := s.Start()
		defer s.Stop()
		assert.FailNow(t, "fail to Server.Start", err)
	}()

	args := []struct {
		Networks []string
		Service  string
		Request  Request
		Response interface{}
	}{
		{
			Networks: []string{networkIconTest, networkEth2Test},
			Request: Request{
				Method: "callInteger",
				Params: contract.Params{
					"arg1": byteVal,
					"arg2": shortVal,
					"arg3": intVal,
					"arg4": longVal,
					"arg5": int24Val,
					"arg6": int40Val,
					"arg7": int72Val,
					"arg8": bigIntegerVal,
				},
			},
			Response: contract.Struct{},
		},
		{
			Networks: []string{networkIconTest},
			Request: Request{
				Method: "callPrimitive",
				Params: contract.Params{
					"arg1": bigIntegerVal,
					"arg2": booleanVal,
					"arg3": stringVal,
					"arg4": bytesVal,
					"arg5": addressVal[icon.NetworkTypeIcon],
				},
			},
			Response: contract.Struct{},
		},
		{
			Networks: []string{networkEth2Test},
			Request: Request{
				Method: "callPrimitive",
				Params: contract.Params{
					"arg1": bigIntegerVal,
					"arg2": booleanVal,
					"arg3": stringVal,
					"arg4": bytesVal,
					"arg5": addressVal[eth.NetworkTypeEth2],
				},
			},
			Response: contract.Struct{},
		},
		{
			Networks: []string{networkIconTest, networkEth2Test},
			Service:  serviceTest,
			Request: Request{
				Method: "callInteger",
				Params: contract.Params{
					"arg1": byteVal,
					"arg2": shortVal,
					"arg3": intVal,
					"arg4": longVal,
					"arg5": int24Val,
					"arg6": int40Val,
					"arg7": int72Val,
					"arg8": bigIntegerVal,
				},
			},
			Response: contract.Struct{},
		},
		{
			Networks: []string{networkIconTest, networkEth2Test},
			Service:  bmc.ServiceName,
			Request: Request{
				Method: "getLinks",
			},
			Response: []string{},
		},
	}
	c := client()
	for _, arg := range args {
		for _, n := range arg.Networks {
			var err error
			if len(arg.Service) > 0 {
				_, err = c.ServiceCall(n, arg.Service, &arg.Request, &arg.Response)
			} else {
				ctr := contracts[n]
				req := &ContractRequest{
					Request: arg.Request,
					Spec:    ctr.Spec,
				}
				_, err = c.Call(n, ctr.Address, req, &arg.Response)
			}
			assert.NoError(t, err)
			t.Logf("response:%v", arg.Response)
		}
	}
}

func Test_ServerInvokeWithoutSignerService(t *testing.T) {
	s := server(t, false)
	go func() {
		err := s.Start()
		defer s.Stop()
		assert.FailNow(t, "fail to Server.Start", err)
	}()

	args := []struct {
		Networks []string
		Service  string
		Request  Request
	}{
		{
			Networks: []string{networkIconTest, networkEth2Test},
			Service:  serviceTest,
			Request: Request{
				Method: "invokeInteger",
				Params: contract.Params{
					"arg1": byteVal,
					"arg2": shortVal,
					"arg3": intVal,
					"arg4": longVal,
					"arg5": int24Val,
					"arg6": int40Val,
					"arg7": int72Val,
					"arg8": bigIntegerVal,
				},
			},
		},
		{
			Networks: []string{networkIconTest},
			Service:  serviceTest,
			Request: Request{
				Method: "invokePrimitive",
				Params: contract.Params{
					"arg1": bigIntegerVal,
					"arg2": booleanVal,
					"arg3": stringVal,
					"arg4": bytesVal,
					"arg5": addressVal[icon.NetworkTypeIcon],
				},
			},
		},
		{
			Networks: []string{networkEth2Test},
			Service:  serviceTest,
			Request: Request{
				Method: "invokePrimitive",
				Params: contract.Params{
					"arg1": bigIntegerVal,
					"arg2": booleanVal,
					"arg3": stringVal,
					"arg4": bytesVal,
					"arg5": addressVal[eth.NetworkTypeEth2],
				},
			},
		},
		{
			Networks: []string{networkIconTest, networkEth2Test},
			Request: Request{
				Method: "invokeInteger",
				Params: contract.Params{
					"arg1": byteVal,
					"arg2": shortVal,
					"arg3": intVal,
					"arg4": longVal,
					"arg5": int24Val,
					"arg6": int40Val,
					"arg7": int72Val,
					"arg8": bigIntegerVal,
				},
			},
		},
	}
	c := client()
	for _, arg := range args {
		for _, n := range arg.Networks {
			var (
				txId contract.TxID
				err  error
			)
			if len(arg.Service) > 0 {
				txId, err = c.ServiceInvoke(n, arg.Service, &arg.Request, signers[n])
			} else {
				ctr := contracts[n]
				req := &ContractRequest{
					Request: arg.Request,
					Spec:    ctr.Spec,
				}
				txId, err = c.Invoke(n, ctr.Address, req, signers[n])
			}
			assert.NoError(t, err)
			t.Logf("txId:%v", txId)
			txr, err := c.GetResult(n, txId)
			assert.NoError(t, err)
			t.Logf("txr:%v", txr)
		}
	}
}

func Test_ServerInvokeWithSignerService(t *testing.T) {
	s := server(t, true)
	go func() {
		err := s.Start()
		defer s.Stop()
		assert.FailNow(t, "fail to Server.Start", err)
	}()

	args := []struct {
		Networks []string
		Service  string
		Request  Request
	}{
		{
			Networks: []string{networkIconTest, networkEth2Test},
			Service:  serviceTest,
			Request: Request{
				Method: "invokeInteger",
				Params: contract.Params{
					"arg1": byteVal,
					"arg2": shortVal,
					"arg3": intVal,
					"arg4": longVal,
					"arg5": int24Val,
					"arg6": int40Val,
					"arg7": int72Val,
					"arg8": bigIntegerVal,
				},
			},
		},
		{
			Networks: []string{networkIconTest},
			Service:  serviceTest,
			Request: Request{
				Method: "invokePrimitive",
				Params: contract.Params{
					"arg1": bigIntegerVal,
					"arg2": booleanVal,
					"arg3": stringVal,
					"arg4": bytesVal,
					"arg5": addressVal[icon.NetworkTypeIcon],
				},
			},
		},
		{
			Networks: []string{networkEth2Test},
			Service:  serviceTest,
			Request: Request{
				Method: "invokePrimitive",
				Params: contract.Params{
					"arg1": bigIntegerVal,
					"arg2": booleanVal,
					"arg3": stringVal,
					"arg4": bytesVal,
					"arg5": addressVal[eth.NetworkTypeEth2],
				},
			},
		},
		{
			Networks: []string{networkIconTest, networkEth2Test},
			Request: Request{
				Method: "invokeInteger",
				Params: contract.Params{
					"arg1": byteVal,
					"arg2": shortVal,
					"arg3": intVal,
					"arg4": longVal,
					"arg5": int24Val,
					"arg6": int40Val,
					"arg7": int72Val,
					"arg8": bigIntegerVal,
				},
			},
		},
	}
	c := client()
	for _, arg := range args {
		for _, n := range arg.Networks {
			var (
				txId contract.TxID
				err  error
			)
			if len(arg.Service) > 0 {
				txId, err = c.ServiceInvoke(n, arg.Service, &arg.Request, nil)
			} else {
				ctr := contracts[n]
				req := &ContractRequest{
					Request: arg.Request,
					Spec:    ctr.Spec,
				}
				txId, err = c.Invoke(n, ctr.Address, req, signers[n])
			}
			assert.NoError(t, err)
			t.Logf("txId:%v", txId)
			txr, err := c.GetResult(n, txId)
			assert.NoError(t, err)
			t.Logf("txr:%v", txr)
		}
	}
}

func Test_ServerMonitorEvent(t *testing.T) {
	s := server(t, true)
	go func() {
		err := s.Start()
		defer s.Stop()
		assert.FailNow(t, "fail to Server.Start", err)
	}()
	args := []struct {
		Networks       []string
		Service        string
		Request        Request
		MonitorRequest MonitorRequest
	}{
		{
			Networks: []string{networkIconTest, networkEth2Test},
			Service:  serviceTest,
			Request: Request{
				Method: "setName",
				Params: contract.Params{
					"name": "testName",
				},
			},
			MonitorRequest: MonitorRequest{
				NameToParams: map[string][]contract.Params{
					"HelloEvent": nil,
				},
				Height: 0,
			},
		},
	}
	c := client()
	for _, arg := range args {
		for _, n := range arg.Networks {
			var (
				txId contract.TxID
				err  error
			)
			if len(arg.Service) > 0 {
				txId, err = c.ServiceInvoke(n, arg.Service, &arg.Request, nil)
			} else {
				ctr := contracts[n]
				req := &ContractRequest{
					Request: arg.Request,
					Spec:    ctr.Spec,
				}
				txId, err = c.Invoke(n, ctr.Address, req, signers[n])
			}
			assert.NoError(t, err)
			t.Logf("txId:%v", txId)
			txr, err := c.GetResult(n, txId)
			assert.NoError(t, err)
			t.Logf("txr:%+v", txr)
			r, ok := txr.(contract.TxResult)
			if !ok {
				assert.FailNow(t, "fail to txr.(contract.TxResult)")
			}
			if len(r.Events()) == 0 {
				assert.FailNow(t, "not found event in TxResult")
			}
			expected := r.Events()[0]
			arg.MonitorRequest.Height = expected.BlockHeight()
			ch := make(chan contract.Event, 0)
			onEvent := func(e contract.Event) error {
				t.Logf("%+v", e)
				ch <- e
				return nil
			}
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				if len(arg.Service) > 0 {
					assert.NoError(t, c.ServiceMonitorEvent(ctx, n, arg.Service, &arg.MonitorRequest, onEvent))
				} else {
					assert.NoError(t, c.MonitorEvent(ctx, n, contracts[n].Address, &arg.MonitorRequest, onEvent))
				}
			}()
			select {
			case e := <-ch:
				var actual contract.BaseEvent
				switch evt := e.(type) {
				case *icon.Event:
					actual = evt.BaseEvent
				case *eth.Event:
					actual = evt.BaseEvent
				default:
					assert.FailNow(t, "invalid event type %T", e)
				}
				assert.Equal(t, expected, actual)
			case <-time.After(10 * time.Second):
				assert.FailNow(t, "timeout assert Event")
			}
			cancel()
		}
	}
}

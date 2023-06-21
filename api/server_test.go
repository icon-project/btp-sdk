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
	"os"
	"testing"

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
	serviceGeneral  = "general"
	serviceTest     = "test"
	serverAddress   = "localhost:8080"
	networkIconTest = "icon_test"
	networkEth2Test = "eth2_test"
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
			Wallet: MustLoadWallet(
				"../example/javascore/src/test/resources/keystore.json",
				"../example/javascore/src/test/resources/keysecret"),
			NetworkType: icon.NetworkTypeIcon,
			AdaptorOption: icon.AdaptorOption{
				NetworkID: "0x3",
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
			Wallet: MustLoadWallet(
				"../example/solidity/test/keystore.json",
				"../example/solidity/test/keysecret"),
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
	Wallet         wallet.Wallet
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

func server(t *testing.T) *Server {
	l := log.GlobalLogger()
	s := NewServer(serverAddress, log.TraceLevel, l)
	svcToNetworks := make(map[string]map[string]service.Network)
	signers := make(map[string]service.Signer)
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
		signers[network] = service.Signer{
			Wallet:      config.Wallet,
			NetworkType: config.NetworkType,
		}
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
		if svc, err = service.NewSignerService(svc, signers, l); err != nil {
			assert.FailNow(t, "fail to NewSignerService", err)
		}
		s.AddService(svc)
	}
	return s
}

func client() *Client {
	return NewClient(fmt.Sprintf("http://%s/api", serverAddress), log.DebugLevel, log.GlobalLogger())
}

func Test_ServerCall(t *testing.T) {
	s := server(t)
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
			Service:  serviceGeneral,
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
			Service:  serviceGeneral,
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
			Service:  serviceGeneral,
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
			if arg.Service != serviceGeneral {
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

func Test_ServerInvoke(t *testing.T) {
	s := server(t)
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
	}
	c := client()
	for _, arg := range args {
		for _, n := range arg.Networks {
			var (
				txId contract.TxID
				err  error
			)
			if arg.Service != serviceGeneral {
				txId, err = c.ServiceInvoke(n, arg.Service, &arg.Request)
			} else {
				ctr := contracts[n]
				req := &ContractRequest{
					Request: arg.Request,
					Spec:    ctr.Spec,
				}
				txId, err = c.Invoke(n, ctr.Address, req)
			}
			assert.NoError(t, err)
			t.Logf("txId:%v", txId)
			txr, err := c.GetResult(n, txId)
			assert.NoError(t, err)
			t.Logf("txr:%v", txr)
		}
	}
}

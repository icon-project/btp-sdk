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

package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/icon-project/btp2/common/codec"
	"github.com/icon-project/btp2/common/log"
	"github.com/icon-project/btp2/common/wallet"
	"github.com/stretchr/testify/assert"

	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/contract/eth"
	"github.com/icon-project/btp-sdk/contract/icon"
)

const (
	networkIconTest = "icon_test"
	networkEthTest  = "eth_test"
)

var (
	configs = map[string]TestConfig{
		networkIconTest: {
			Endpoint: "http://localhost:9080/api/v3/icon_dex",
			NetworkType: icon.NetworkTypeIcon,
			AdaptorOption: icon.AdaptorOption{
				NetworkID: "0x3",
				//TransportLogLevel: contract.LogLevel(log.DebugLevel),
			},
			ServiceOption: DefaultServiceOptions{
				ContractAddress: "cxbdb2fac53eaf445f9f0d0c33306d6b2a1a25353d",
			},
		},
		networkEthTest: {
			Endpoint: "http://localhost:8545",
			NetworkType: eth.NetworkTypeEth,
			AdaptorOption: eth.AdaptorOption{
				BlockMonitor: MustEncodeOptions(eth.BlockMonitorOptions{
					FinalizeBlockCount: 3,
				}),
			},
			//NetworkType: eth.NetworkTypeEth2,
			//AdaptorOption: eth.AdaptorOption{
			//	BlockMonitor: MustEncodeOptions(eth.Eth2BlockMonitorOptions{
			//		Endpoint: "http://localhost:9596",
			//	}),
			//},
			ServiceOption: DefaultServiceOptions{
				ContractAddress: "0x09635F643e140090A9A8Dcd712eD6285858ceBef",
			},
		},
	}
	signers = map[string]Signer{
		networkIconTest: NewDefaultSigner(
			MustLoadWallet("../example/javascore/src/test/resources/keystore.json", "../example/javascore/src/test/resources/keysecret"),
			icon.NetworkTypeIcon),
		networkEthTest: NewDefaultSigner(
			MustLoadWallet("../example/solidity/test/keystore.json", "../example/solidity/test/keysecret"),
			eth.NetworkTypeEth),
	}
)

type TestConfig struct {
	NetworkType   string
	Endpoint      string
	AdaptorOption interface{}
	ServiceOption DefaultServiceOptions
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

const (
	serviceName  = "test"
	iconSpecFile = "../example/javascore/build/generated/contractSpec.json"
	ethSpecFile  = "../example/solidity/artifacts/contracts/HelloWorld.sol/HelloWorld.abi.json"
)

var (
	typeToSpec = map[string][]byte{
		icon.NetworkTypeIcon: MustReadFile(iconSpecFile),
		eth.NetworkTypeEth:   MustReadFile(ethSpecFile),
		eth.NetworkTypeEth2:  MustReadFile(ethSpecFile),
	}

	iconSpec = contract.MustNewSpec(icon.NetworkTypeIcon, typeToSpec[icon.NetworkTypeIcon])
	ethSpec  = contract.MustNewSpec(eth.NetworkTypeEth, typeToSpec[eth.NetworkTypeEth])
)

type TestService struct {
	*DefaultService
	spec Spec
	l    log.Logger
}

func (s *TestService) Spec() Spec {
	return s.spec
}

func (s *TestService) EventFilters(network string, nameToParams map[string][]contract.Params) ([]contract.EventFilter, error) {
	n, err := s.network(network)
	if err != nil {
		return nil, err
	}
	if n.NetworkType == icon.NetworkTypeIcon {
		m := make(map[string][]contract.Params)
		for name, l := range nameToParams {
			switch name {
			case "StructEvent", "ArrayEvent":
				se := s.spec.Events[name]
				tl := make([]contract.Params, len(l))
				m[name] = tl
				for i, p := range l {
					if p == nil {
						continue
					}
					tp := make(contract.Params)
					tl[i] = tp
					for k, v := range p {
						b, err := MarshalRLP(v, se.Inputs[k].Type)
						if err != nil {
							s.l.Infof("fail to MarshalRLP event:%s param:%s err:%+v", name, k, err)
							return nil, err
						}
						tp[k] = contract.Bytes(b)
						s.l.Infof("EventFilter k:%s v:%s", k, hex.EncodeToString(b))
					}
				}
			default:
				m[name] = l
			}
		}
		nameToParams = m
	}
	return s.DefaultService.EventFilters(network, nameToParams)
}

func (s *TestService) MonitorEvent(ctx context.Context, network string, cb contract.EventCallback, efs []contract.EventFilter, height int64) error {
	n, err := s.network(network)
	if err != nil {
		return err
	}
	ecb := cb
	if n.NetworkType == icon.NetworkTypeIcon {
		ecb = func(e contract.Event) error {
			name := e.Name()
			switch name {
			case "StructEvent", "ArrayEvent":
				se := s.spec.Events[name]
				p := e.Params()
				for k, v := range p {
					if eivp, ok := v.(contract.EventIndexedValueWithParam); ok {
						v = eivp.Param()
					}
					if b, ok := v.(contract.Bytes); ok {
						cv, err := UnmarshalRLP(b, se.Inputs[k].Type)
						if err != nil {
							s.l.Infof("fail to UnmarshalRLP event:%s param:%s err:%+v", name, k, err)
							return err
						}
						p[k] = cv
					}
				}
			}
			return cb(e)
		}
	}
	return s.DefaultService.MonitorEvent(ctx, network, ecb, efs, height)
}

func NewTestService(networks map[string]Network, l log.Logger) (Service, error) {
	s, err := NewDefaultService(serviceName, networks, typeToSpec, l)
	if err != nil {
		return nil, err
	}
	ss := CopySpec(s.Spec())
	if err = ss.MergeOverloads(map[string]MergeInfo{
		"StructEvent": {
			Flag: EventOverloadInputs,
			Spec: ethSpec,
		},
		"ArrayEvent": {
			Flag: EventOverloadInputs,
			Spec: ethSpec,
		},
	}); err != nil {
		return nil, err
	}
	return &TestService{
		DefaultService: s,
		spec:           ss,
		l:              l,
	}, nil
}

func service(t *testing.T, withSignerService bool) (Service, map[string]Network) {
	networks := make(map[string]Network)
	for network, config := range configs {
		networks[network] = Network{
			NetworkType: config.NetworkType,
			Adaptor:     adaptor(t, network),
			Options:     MustEncodeOptions(config.ServiceOption),
		}
	}
	RegisterFactory(serviceName, NewTestService)
	l := log.GlobalLogger()
	s, err := NewService(serviceName, networks, l)
	if err != nil {
		assert.FailNow(t, "fail to NewService", err)
	}
	if withSignerService {
		if s, err = NewSignerService(s, signers, l); err != nil {
			assert.FailNow(t, "fail to NewSignerService", err)
		}
	}
	return s, networks
}

var (
	integerVal = "0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	booleanVal = true
	stringVal  = "string"
	bytesVal   = "0x" + hex.EncodeToString([]byte("bytes"))
	structVal  = contract.Params{
		"booleanVal": booleanVal,
	}
)

func Test_Service(t *testing.T) {
	s, networks := service(t, true)

	args := []struct {
		Networks     []string
		Method       string
		MethodParams contract.Params
		Event        string
		EventParams  contract.Params
	}{
		{
			Networks: []string{
				networkIconTest,
				networkEthTest,
			},
			Method: "invokeStruct",
			MethodParams: contract.Params{
				"arg1": structVal,
			},
			Event: "StructEvent",
			EventParams: contract.Params{
				"arg1": structVal,
			},
		},
		{
			Networks: []string{
				networkIconTest,
				networkEthTest,
			},
			Method: "invokeArray",
			MethodParams: contract.Params{
				"arg1": []interface{}{integerVal},
				"arg2": []interface{}{booleanVal},
				"arg3": []interface{}{stringVal},
				"arg4": []interface{}{bytesVal},
				"arg5": []interface{}{},
				"arg6": []interface{}{structVal},
			},
			Event: "ArrayEvent",
			EventParams: contract.Params{
				"arg1": []interface{}{integerVal},
				"arg2": []interface{}{booleanVal},
				"arg3": []interface{}{stringVal},
				"arg4": []interface{}{bytesVal},
				"arg5": []interface{}{},
				"arg6": []interface{}{structVal},
			},
		},
	}
	for _, arg := range args {
		for _, n := range arg.Networks {
			txId, err := s.Invoke(n, arg.Method, arg.MethodParams, nil)
			if err != nil {
				assert.FailNow(t, "fail to Invoke", err)
			}
			t.Logf("txId:%v", txId)
			txr, err := networks[n].Adaptor.GetResult(txId)
			assert.NoError(t, err)
			t.Logf("txr:%+v", txr)

			efs, err := s.EventFilters(n, map[string][]contract.Params{arg.Event: {arg.EventParams}})
			if err != nil {
				assert.FailNow(t, "fail to EventFilters", err)
			}
			if len(txr.Events()) == 0 {
				assert.FailNow(t, "not found event in TxResult")
			}
			var expected contract.Event
			for _, be := range txr.Events() {
				e, _ := efs[0].Filter(be)
				if e != nil {
					expected = e
				}
			}
			if expected == nil {
				assert.FailNow(t, "not found event by EventFilter")
			}

			ch := make(chan contract.Event, 0)
			onEvent := func(e contract.Event) error {
				LogJson(t, e)
				ch <- e
				return nil
			}
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				err := s.MonitorEvent(ctx, n, onEvent, efs, txr.BlockHeight())
				assert.Equal(t, ctx.Err(), err)
			}()
			select {
			case e := <-ch:
				//Replace if EventIndexedValueWithParam
				params := e.Params()
				for k, p := range params {
					if eivp, ok := p.(contract.EventIndexedValueWithParam); ok {
						t.Logf("EventIndexedValueWithParam k:%s v:%v", k, eivp.Param())
						params[k] = eivp.Param()
					}
				}
				assert.NoError(t, contract.ParamsTypeCheck(s.Spec().Events[arg.Event].Inputs, params))
				t.Logf("%+v", e)
			case <-time.After(10 * time.Second):
				assert.FailNow(t, "timeout assert Event")
			}
			cancel()
		}
	}
}

func LogJson(t *testing.T, v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		assert.FailNow(t, "fail to MarshalIndent", err)
	}
	t.Log(string(b))
}

func LogJsonPretty(t *testing.T, v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		assert.FailNow(t, "fail to MarshalIndent", err)
	}
	t.Log(string(b))
}

func MarshalRLP(v interface{}, spec contract.TypeSpec) ([]byte, error) {
	p, err := contract.ParamToGoType(spec, v)
	if err != nil {
		return nil, err
	}
	log.Infof("MarshalRLP %+v", p)
	return codec.RLP.MarshalToBytes(p)
}

func UnmarshalRLP(b []byte, spec contract.TypeSpec) (interface{}, error) {
	t := spec.Type
	if spec.TypeID == contract.TStruct {
		t = spec.ResolvedType
	}
	ptr := reflect.New(t)
	if _, err := codec.RLP.UnmarshalFromBytes(b, ptr.Interface()); err != nil {
		return nil, err
	}
	b, err := json.Marshal(ptr.Interface())
	if err != nil {
		return nil, err
	}
	var i interface{}
	if err = json.Unmarshal(b, &i); err != nil {
		return nil, err
	}
	return contract.ParamOfWithSpec(spec, i)
}

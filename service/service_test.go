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
	"encoding/json"
	"os"
	"reflect"
	"strings"
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
			Wallet: MustLoadWallet(
				"../example/javascore/src/test/resources/keystore.json",
				"../example/javascore/src/test/resources/keysecret"),
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
			Wallet: MustLoadWallet(
				"../example/solidity/test/keystore.json",
				"../example/solidity/test/keysecret"),
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
)

type TestConfig struct {
	NetworkType   string
	Endpoint      string
	Wallet        wallet.Wallet
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

func (s *TestService) MonitorEvent(ctx context.Context, network string, cb contract.EventCallback, efs []contract.EventFilter, height int64) error {
	m := UnmarshalRLPEventCallbackMap(s.spec, "StructEvent", "ArrayEvent")
	return s.DefaultService.MonitorEvent(ctx, network, func(e contract.Event) error {
		s.l.Infof("%+v", e)
		if ecb, ok := m[e.Name()]; ok {
			if err := ecb(e); err != nil {
				s.l.Infof("fail to ecb err:%+v", err)
				return err
			}
		}
		return cb(e)
	}, efs, height)
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

func Test_Service(t *testing.T) {
	networks := make(map[string]Network)
	signers := make(map[string]Signer)
	for network, config := range configs {
		networks[network] = Network{
			NetworkType: config.NetworkType,
			Adaptor:     adaptor(t, network),
			Options:     MustEncodeOptions(config.ServiceOption),
		}
		signers[network] = NewDefaultSigner(config.Wallet, config.NetworkType)
	}
	RegisterFactory(serviceName, NewTestService)
	l := log.GlobalLogger()
	s, err := NewService(serviceName, networks, l)
	if err != nil {
		assert.FailNow(t, "fail to NewService", err)
	}
	if s, err = NewSignerService(s, signers, l); err != nil {
		assert.FailNow(t, "fail to NewSignerService", err)
	}
	assert.Equal(t, serviceName, s.Name())
	//LogJsonPretty(t, s.Spec())

	network := networkIconTest
	txId, err := s.Invoke(network, "invokeStruct", contract.Params{"arg1": contract.Params{"booleanVal": true}}, nil)
	if err != nil {
		assert.FailNow(t, "fail to Invoke", err)
	}
	t.Logf("txId:%v", txId)
	txr, err := networks[network].Adaptor.GetResult(txId)
	assert.NoError(t, err)
	t.Logf("txr:%+v", txr)

	efs, err := s.EventFilters(network, map[string][]contract.Params{"StructEvent": nil})
	if err != nil {
		assert.FailNow(t, "fail to EventFilters", err)
	}
	ch := make(chan contract.Event, 0)
	onEvent := func(e contract.Event) error {
		LogJson(t, e)
		ch <- e
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		assert.NoError(t, s.MonitorEvent(ctx, network, onEvent, efs, txr.BlockHeight()))
	}()
	select {
	case e := <-ch:
		t.Logf("%+v", e)
	case <-time.After(10 * time.Second):
		assert.FailNow(t, "timeout assert Event")
	}
	cancel()
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

func UnmarshalRLPEventCallbackMap(s Spec, names ...string) map[string]contract.EventCallback {
	m := make(map[string]contract.EventCallback)
	for _, name := range names {
		m[name] = UnmarshalRLPEventCallback(s.Events[name])
	}
	return m
}

func UnmarshalRLPEventCallback(s *EventSpec) contract.EventCallback {
	rtMap := make(map[string]reflect.Type)
	for k, i := range s.Inputs {
		rtMap[k] = ReflectType(i.Type)
	}
	return func(e contract.Event) error {
		p := e.Params()
		for k, v := range p {
			if b, ok := v.(contract.Bytes); ok {
				cv, err := UnmarshalRLP(b, s.Inputs[k].Type, rtMap[k])
				if err != nil {
					return err
				}
				p[k] = cv
			}
		}
		return nil
	}
}

func ReflectType(s contract.TypeSpec) reflect.Type {
	switch s.TypeID {
	case contract.TStruct:
		sfs := make([]reflect.StructField, 0)
		for _, f := range s.Resolved.Fields {
			sf := reflect.StructField{
				Name: strings.ToUpper(f.Name[:1]) + f.Name[1:],
				Type: ReflectType(f.Type),
				Tag:  reflect.StructTag("json:\"" + f.Name + "\""),
			}
			sfs = append(sfs, sf)
		}
		t := reflect.StructOf(sfs)
		for i := 0; i < s.Dimension; i++ {
			t = reflect.SliceOf(t)
		}
		return t
	default:
		return s.Type
	}
}

func UnmarshalRLP(b []byte, spec contract.TypeSpec, rt reflect.Type) (interface{}, error) {
	ptr := reflect.New(rt)
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

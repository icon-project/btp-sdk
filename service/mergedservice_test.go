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
	"testing"
	"time"

	"github.com/icon-project/btp2/common/log"
	"github.com/stretchr/testify/assert"

	"github.com/icon-project/btp-sdk/contract"
)

type TestMergedService struct {
	*MergedService
	l log.Logger
}

func NewTestMergedService(networks map[string]Network, l log.Logger) (Service, error) {
	s, err := NewDefaultService(serviceName, networks, typeToSpec, l)
	if err != nil {
		return nil, err
	}
	ms, err := NewMergedService(s, &TestMergeHandler{}, l)
	if err != nil {
		return nil, err
	}
	return &TestMergedService{
		MergedService: ms,
		l:             l,
	}, nil
}

type TestMergeHandler struct {
}

func (t *TestMergeHandler) MergeMethodOverloads(s *MethodSpec) (bool, error) {
	return false, nil
}

func (t *TestMergeHandler) MergeEventOverloads(s *EventSpec) (bool, error) {
	switch s.Name {
	case "StructEvent", "ArrayEvent":
		return true, s.MergeOverloads(ethSpec.EventMap[s.Name], EventOverloadInputs)
	}
	return false, nil
}

func (t *TestMergeHandler) HandleMethodInputs(name string, org, merged map[string]*contract.NameAndTypeSpec, params contract.Params) (contract.Params, error) {
	return params, nil
}

func (t *TestMergeHandler) HandleMethodOutput(name string, org, merged contract.TypeSpec, r contract.ReturnValue) (contract.ReturnValue, error) {
	return r, nil
}

func (t *TestMergeHandler) HandleEventFilterParams(s *EventSpec, o *EventOverload, l []contract.Params) ([]contract.Params, error) {
	switch s.Name {
	case "StructEvent", "ArrayEvent":
		tl := make([]contract.Params, len(l))
		for i, p := range l {
			if p == nil {
				continue
			}
			tp := make(contract.Params)
			tl[i] = tp
			for k, v := range p {
				b, err := MarshalRLP(v, s.Inputs[k].Type)
				if err != nil {
					return nil, err
				}
				tp[k] = contract.Bytes(b)
			}
		}
		return tl, nil
	}
	return l, nil
}

func (t *TestMergeHandler) HandleEvent(s *EventSpec, o *EventOverload, e contract.Event) error {
	switch e.Name() {
	case "StructEvent", "ArrayEvent":
		p := e.Params()
		for k, v := range p {
			if eivp, ok := v.(contract.EventIndexedValueWithParam); ok {
				v = eivp.Param()
			}
			if b, ok := v.(contract.Bytes); ok {
				cv, err := UnmarshalRLP(b, s.Inputs[k].Type)
				if err != nil {
					return err
				}
				p[k] = cv
			}
		}
	}
	return nil
}

func mergedService(t *testing.T, withSignerService bool) (Service, map[string]Network) {
	networks := make(map[string]Network)
	for network, config := range configs {
		networks[network] = Network{
			NetworkType: config.NetworkType,
			Adaptor:     adaptor(t, network),
			Options:     MustEncodeOptions(config.ServiceOption),
		}
	}
	RegisterFactory(serviceName, NewTestMergedService)
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

func Test_MergedService(t *testing.T) {
	s, networks := mergedService(t, true)

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
				if !assertParamsType(t, s.Spec().Events[arg.Event].Inputs, params) {
					assert.FailNow(t, "fail assertParamsType")
				}
				t.Logf("%+v", e)
			case <-time.After(10 * time.Second):
				assert.FailNow(t, "timeout assert Event")
			}
			cancel()
		}
	}
}

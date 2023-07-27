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

	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
)

type MergeHandler interface {
	MergeMethodOverloads(s *MethodSpec) (bool, error)
	MergeEventOverloads(s *EventSpec) (bool, error)
	HandleMethodInputs(name string, org, merged map[string]*contract.NameAndTypeSpec, params contract.Params) (contract.Params, error)
	HandleMethodOutput(name string, org, merged contract.TypeSpec, r contract.ReturnValue) (contract.ReturnValue, error)
	HandleEventFilterParams(s *EventSpec, o *EventOverload, l []contract.Params) ([]contract.Params, error)
	HandleEvent(s *EventSpec, o *EventOverload, e contract.Event) error
}

type MergedService struct {
	Service
	nMap    map[string]string
	methods map[string]*MethodSpec
	events  map[string]*EventSpec
	spec    Spec
	mh      MergeHandler
	l       log.Logger
}

func compareMethod(a, b *MethodSpec) MethodOverloadFlag {
	f := MethodOverloadNone
	if !contract.EqualsNameAndTypes(a.Inputs, b.Inputs) {
		f |= MethodOverloadInputs
	}
	if !contract.EqualsTypeSpec(a.Output, b.Output) {
		f |= MethodOverloadOutput
	}
	if a.Payable != b.Payable {
		f |= MethodOverloadPayable
	}
	if a.Readonly != b.Readonly {
		f |= MethodOverloadReadonly
	}
	return f
}

func newMethodOverload(f MethodOverloadFlag, m *MethodSpec, networkTypes ...string) *MethodOverload {
	o := &MethodOverload{}
	o.NetworkTypes = append(o.NetworkTypes, networkTypes...)
	if f.Inputs() {
		o.Inputs = &m.Inputs
	}
	if f.Output() {
		o.Output = &m.Output
	}
	if f.Payable() {
		o.Payable = &m.Payable
	}
	if f.Readonly() {
		o.Readonly = &m.Readonly
	}
	return o
}

func newMethodSpec(merged, origin *MethodSpec) *MethodSpec {
	f := compareMethod(merged, origin)
	method := CopyMethodSpec(merged)
	method.Overloads = []*MethodOverload{newMethodOverload(f, origin, origin.NetworkTypes...)}
	for _, oo := range origin.Overloads {
		o := &MethodOverload{
			NetworkTypes: oo.NetworkTypes,
		}
		method.Overloads = append(method.Overloads, o)
		if oo.Inputs != nil {
			if !f.Inputs() || !contract.EqualsNameAndTypes(merged.Inputs, *oo.Inputs) {
				o.Inputs = oo.Inputs
			}
		} else {
			if f.Inputs() {
				o.Inputs = &origin.Inputs
			}
		}
		if oo.Output != nil {
			if !f.Output() || !contract.EqualsTypeSpec(merged.Output, *oo.Output) {
				o.Output = oo.Output
			}
		} else {
			if f.Output() {
				o.Output = &origin.Output
			}
		}
	}
	return method
}

func compareEventSpec(a, b *EventSpec) EventOverloadFlag {
	f := EventOverloadNone
	if !contract.EqualsNameAndTypes(a.Inputs, b.Inputs) {
		f |= EventOverloadInputs
	}
	if !EqualsEventIndexed(a.Indexed, b.Indexed) {
		f |= EventOverloadIndexed
	}
	return f
}

func newEventOverload(f EventOverloadFlag, m *EventSpec, networkTypes ...string) *EventOverload {
	o := &EventOverload{}
	o.NetworkTypes = append(o.NetworkTypes, networkTypes...)
	if f.Inputs() {
		o.Inputs = &m.Inputs
	}
	if f.Indexed() {
		o.Indexed = &m.Indexed
	}
	return o
}

func newEventSpec(merged, origin *EventSpec) *EventSpec {
	f := compareEventSpec(merged, origin)
	event := CopyEventSpec(merged)
	event.Overloads = []*EventOverload{newEventOverload(f, origin, origin.NetworkTypes...)}
	for _, oo := range origin.Overloads {
		o := &EventOverload{
			NetworkTypes: oo.NetworkTypes,
		}
		event.Overloads = append(event.Overloads, o)
		if oo.Inputs != nil {
			if !f.Inputs() || !contract.EqualsNameAndTypes(merged.Inputs, *oo.Inputs) {
				o.Inputs = oo.Inputs
			}
		} else {
			if f.Inputs() {
				o.Inputs = &origin.Inputs
			}
		}
		if oo.Indexed != nil {
			if !f.Indexed() || !EqualsEventIndexed(merged.Indexed, *oo.Indexed) {
				o.Indexed = oo.Indexed
			}
		} else {
			if f.Indexed() {
				o.Indexed = &origin.Indexed
			}
		}
	}
	return event
}

func NewMergedService(s Service, mh MergeHandler, l log.Logger) (*MergedService, error) {
	methods := make(map[string]*MethodSpec)
	events := make(map[string]*EventSpec)
	origin := CopySpec(s.Spec())
	merged := CopySpec(s.Spec())
	for name, m := range merged.Methods {
		ok, err := mh.MergeMethodOverloads(m)
		if err != nil {
			return nil, err
		}
		if len(m.Overloads) > 0 {
			return nil, errors.Errorf("method:%s not merged", name)
		}
		if ok {
			methods[name] = newMethodSpec(m, origin.Methods[name])
		}
	}
	for name, e := range merged.Events {
		ok, err := mh.MergeEventOverloads(e)
		if err != nil {
			return nil, err
		}
		if len(e.Overloads) > 0 {
			return nil, errors.Errorf("event:%s not merged", name)
		}
		if ok {
			events[name] = newEventSpec(e, origin.Events[name])
		}
	}
	return &MergedService{
		Service: s,
		nMap:    s.Networks(),
		spec:    merged,
		methods: methods,
		events:  events,
		mh:      mh,
		l:       l,
	}, nil
}

func (s *MergedService) Spec() Spec {
	return s.spec
}

func (s *MergedService) networkType(network string) (string, error) {
	nt, ok := s.nMap[network]
	if !ok {
		return nt, errors.Errorf("network:%s not found", network)
	}
	return nt, nil
}

func (s *MergedService) methodOverload(network string, name string) (*MethodSpec, *MethodOverload, error) {
	nt, err := s.networkType(network)
	if err != nil {
		return nil, nil, err
	}
	ms, ok := s.methods[name]
	if !ok {
		if _, ok = s.spec.Methods[name]; !ok {
			return nil, nil, errors.Errorf("method:%s not found", name)
		} else {
			return nil, nil, nil
		}
	}
	mo := ms.Overload(nt)
	if mo == nil {
		return nil, nil, errors.Errorf("method:%s not supported, networkType:%s", name, nt)
	}
	return ms, mo, nil
}

func (s *MergedService) Invoke(network, method string, params contract.Params, options contract.Options) (contract.TxID, error) {
	ms, mo, err := s.methodOverload(network, method)
	if err != nil {
		return nil, err
	}
	if mo != nil && mo.Inputs != nil {
		if params, err = s.mh.HandleMethodInputs(ms.Name, *mo.Inputs, ms.Inputs, params); err != nil {
			return nil, err
		}
	}
	return s.Service.Invoke(network, method, params, options)
}

func (s *MergedService) Call(network, method string, params contract.Params, options contract.Options) (contract.ReturnValue, error) {
	ms, mo, err := s.methodOverload(network, method)
	if err != nil {
		return nil, err
	}
	if mo != nil && mo.Inputs != nil {
		if params, err = s.mh.HandleMethodInputs(ms.Name, *mo.Inputs, ms.Inputs, params); err != nil {
			return nil, err
		}
	}
	r, err := s.Service.Call(network, method, params, options)
	if err != nil {
		return nil, err
	}
	if mo != nil && mo.Output != nil {
		if r, err = s.mh.HandleMethodOutput(ms.Name, *mo.Output, ms.Output, r); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (s *MergedService) eventOverload(networkType string, name string) (*EventSpec, *EventOverload, error) {
	es, ok := s.events[name]
	if !ok {
		if _, ok = s.spec.Events[name]; !ok {
			return nil, nil, errors.Errorf("event:%s not found", name)
		} else {
			return nil, nil, nil
		}
	}
	eo := es.Overload(networkType)
	if eo == nil {
		return nil, nil, errors.Errorf("event:%s not supported, networkType:%s", name, networkType)
	}
	return es, eo, nil
}

func (s *MergedService) EventFilters(network string, nameToParams map[string][]contract.Params) ([]contract.EventFilter, error) {
	nt, err := s.networkType(network)
	if err != nil {
		return nil, err
	}
	for name, l := range nameToParams {
		es, eo, err := s.eventOverload(nt, name)
		if err != nil {
			return nil, err
		}
		if eo != nil && eo.Inputs != nil {
			if l, err = s.mh.HandleEventFilterParams(es, eo, l); err != nil {
				return nil, err
			}
			nameToParams[name] = l
		}
	}
	return s.Service.EventFilters(network, nameToParams)
}

func (s *MergedService) MonitorEvent(ctx context.Context, network string, cb contract.EventCallback, efs []contract.EventFilter, height int64) error {
	nt, err := s.networkType(network)
	if err != nil {
		return err
	}
	ecb := func(e contract.Event) error {
		es, eo, err := s.eventOverload(nt, e.Name())
		if err != nil {
			return err
		}
		if eo != nil && eo.Inputs != nil {
			if err = s.mh.HandleEvent(es, eo, e); err != nil {
				return err
			}
		}
		return cb(e)
	}
	return s.Service.MonitorEvent(ctx, network, ecb, efs, height)
}

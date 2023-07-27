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

package eth

import (
	"bytes"
	"encoding/json"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/icon-project/btp2/common/errors"

	"github.com/icon-project/btp-sdk/contract"
)

func init() {
	contract.RegisterSpecFactory(NewSpec, NetworkTypes...)
}

func NewSpec(b []byte) (*contract.Spec, error) {
	out, err := abi.JSON(bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	return SpecFromABI(out)
}

func SpecFromABI(out abi.ABI) (*contract.Spec, error) {
	spec := &contract.Spec{
		Methods:   make([]*contract.MethodSpec, 0),
		Events:    make([]*contract.EventSpec, 0),
		Structs:   make([]*contract.StructSpec, 0),
		MethodMap: make(map[string]*contract.MethodSpec),
		EventMap:  make(map[string]*contract.EventSpec),
		StructMap: make(map[string]*contract.StructSpec),
	}
	for _, m := range out.Methods {
		method := &contract.MethodSpec{
			Name:   m.RawName,
			Inputs: make([]*contract.NameAndTypeSpec, 0),
			Output: contract.TypeSpec{
				Name:   contract.TVoid.String(),
				TypeID: contract.TVoid,
			},
			ReadOnly: m.IsConstant(),
			Payable:  m.IsPayable(),
			InputMap: make(map[string]*contract.NameAndTypeSpec),
		}
		for _, i := range m.Inputs {
			it, err := newTypeSpec(i.Type, spec)
			if err != nil {
				return nil, errors.Wrapf(err, "method:%s input:%s err:%s", method.Name, i.Name, err.Error())
			}
			input := &contract.NameAndTypeSpec{
				Name: i.Name,
				Type: it,
			}
			method.Inputs = append(method.Inputs, input)
			method.InputMap[input.Name] = input
		}
		if len(m.Outputs) > 1 {
			return nil, errors.Errorf("method:%s output err:not support multiple", method.Name)
		}
		if len(m.Outputs) == 1 {
			ot, err := newTypeSpec(m.Outputs[0].Type, spec)
			if err != nil {
				return nil, errors.Wrapf(err, "method:%s output err:%s", method.Name, err.Error())
			}
			method.Output = ot
		}

		if old, exists := spec.MethodMap[method.Name]; exists {
			if !contract.EqualsTypeSpec(old.Output, method.Output) {
				return nil, errors.Errorf("method:%s overload err:not support output overload", method.Name)
			}
			optionals, err := contract.OptionalInputs(old.InputMap, method.InputMap)
			if err != nil {
				return nil, errors.Wrapf(err, "method:%s overload err:%s", method.Name, err.Error())
			}
			for k, v := range optionals {
				input, ok := old.InputMap[k]
				if !ok {
					input = v
					old.Inputs = append(old.Inputs, input)
					old.InputMap[k] = input
				}
				input.Optional = true
			}
			continue
		}
		spec.Methods = append(spec.Methods, method)
		spec.MethodMap[method.Name] = method
	}
	for _, e := range out.Events {
		event := &contract.EventSpec{
			Name:        e.RawName,
			Indexed:     0,
			Inputs:      make([]*contract.NameAndTypeSpec, 0),
			InputMap:    make(map[string]*contract.NameAndTypeSpec),
			Signature:   "",
			NameToIndex: make(map[string]int),
		}

		for idx, i := range e.Inputs {
			it, err := newTypeSpec(i.Type, spec)
			if err != nil {
				return nil, errors.Wrapf(err, "event:%s input:%s err:%s", event.Name, i.Name, err.Error())
			}
			input := &contract.NameAndTypeSpec{
				Name: i.Name,
				Type: it,
			}
			if i.Indexed {
				event.Indexed++
				event.NameToIndex[input.Name] = idx
			}
			event.Inputs = append(event.Inputs, input)
			event.InputMap[input.Name] = input
		}

		if old, exists := spec.EventMap[event.Name]; exists {
			optionals, err := contract.OptionalInputs(old.InputMap, event.InputMap)
			if err != nil {
				return nil, errors.Wrapf(err, "event:%s overload err:%s", event.Name, err.Error())
			}
			for k, v := range optionals {
				input, ok := old.InputMap[k]
				if !ok {
					input = v
					old.Inputs = append(old.Inputs, input)
					old.InputMap[k] = input
				}
				input.Optional = true
			}
			continue
		}
		spec.Events = append(spec.Events, event)
		spec.EventMap[event.Name] = event
	}
	b, err := json.Marshal(spec)
	if err != nil {
		return nil, errors.Wrapf(err, "fail to Marshal err:%v", err.Error())
	}
	ret := &contract.Spec{}
	err = json.Unmarshal(b, ret)
	if err != nil {
		return nil, errors.Wrapf(err, "fail to Unmarshal err:%v", err.Error())
	}
	return ret, nil
}

func newTypeSpec(t abi.Type, spec *contract.Spec) (contract.TypeSpec, error) {
	switch t.T {
	case abi.ArrayTy, abi.SliceTy:
		return newArrayTypeSpec(t, spec)
	case abi.TupleTy:
		return newStructTypeSpec(t, spec)
	default:
		return newPrimitiveTypeSpec(t)
	}
}

func newPrimitiveTypeSpec(t abi.Type) (contract.TypeSpec, error) {
	var s contract.TypeTag
	switch t.T {
	case abi.IntTy, abi.UintTy:
		s = contract.TInteger
	case abi.StringTy:
		s = contract.TString
	case abi.AddressTy:
		s = contract.TAddress
	case abi.BytesTy:
		s = contract.TBytes
	case abi.BoolTy:
		s = contract.TBoolean
	default:
		return contract.TypeSpec{}, errors.Errorf("not supported type:%s", t.String())
	}
	return contract.TypeSpec{
		Name:   s.String(),
		TypeID: s,
	}, nil
}

func newStructTypeSpec(t abi.Type, spec *contract.Spec) (contract.TypeSpec, error) {
	s := &contract.StructSpec{
		Name:     t.TupleRawName,
		Fields:   make([]*contract.NameAndTypeSpec, 0),
		FieldMap: make(map[string]*contract.NameAndTypeSpec),
	}
	for i, n := range t.TupleRawNames {
		ft, err := newTypeSpec(*t.TupleElems[i], spec)
		if err != nil {
			return contract.TypeSpec{}, err
		}
		field := &contract.NameAndTypeSpec{
			Name: n,
			Type: ft,
		}
		s.Fields = append(s.Fields, field)
		s.FieldMap[n] = field
	}
	if old, ok := spec.StructMap[s.Name]; ok {
		if !contract.EqualsNameAndTypes(old.FieldMap, s.FieldMap) {
			return contract.TypeSpec{}, errors.Errorf("duplicated struct type:%s", s.Name)
		}
		return contract.TypeSpec{
			Name:     s.Name,
			Resolved: old,
		}, nil
	}
	spec.Structs = append(spec.Structs, s)
	spec.StructMap[s.Name] = s
	return contract.TypeSpec{
		Name:     s.Name,
		Resolved: s,
	}, nil
}

func newArrayTypeSpec(t abi.Type, spec *contract.Spec) (contract.TypeSpec, error) {
	if t.T != abi.ArrayTy && t.T != abi.SliceTy {
		return contract.TypeSpec{}, errors.Errorf("invalid array type:%s", t.String())
	}
	s, err := newTypeSpec(*t.Elem, spec)
	if err != nil {
		return contract.TypeSpec{}, err
	}
	return contract.TypeSpec{
		Name:      s.Name,
		TypeID:    s.TypeID,
		Dimension: s.Dimension + 1,
	}, nil
}

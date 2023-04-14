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
	"encoding/json"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
)

func NewSpec(out abi.ABI) contract.Spec {
	spec := contract.Spec{
		Methods: make([]contract.MethodSpec, 0),
		Events:  make([]contract.EventSpec, 0),
		Structs: make([]contract.StructSpec, 0),
	}
	structMap := make(map[string]contract.StructSpec)
	for _, m := range out.Methods {
		inputs := make([]contract.NameAndTypeSpec, 0)
		for _, i := range m.Inputs {
			input := contract.NameAndTypeSpec{
				Name: i.Name,
				Type: NewTypeSpec(i.Type, structMap),
			}
			inputs = append(inputs, input)
		}
		var output contract.TypeSpec
		if len(m.Outputs) == 0 {
			output.Name = contract.TVoid.String()
		} else if len(m.Outputs) == 1 {
			o := m.Outputs[0]
			output = NewTypeSpec(o.Type, structMap)
		} else {
			log.Panicln("not support multiple Outputs method:", m.Name)
		}

		method := contract.MethodSpec{
			Name:     m.RawName,
			Inputs:   inputs,
			Output:   output,
			ReadOnly: m.IsConstant(),
			Payable:  m.IsPayable(),
		}
		spec.Methods = append(spec.Methods, method)
	}
	for _, e := range out.Events {
		indexed := 0
		inputs := make([]contract.NameAndTypeSpec, 0)
		for _, i := range e.Inputs {
			input := contract.NameAndTypeSpec{
				Name: i.Name,
				Type: NewTypeSpec(i.Type, structMap),
			}
			if i.Indexed {
				indexed++
			}
			inputs = append(inputs, input)
		}
		event := contract.EventSpec{
			Name:    e.RawName,
			Indexed: indexed,
			Inputs:  inputs,
		}
		spec.Events = append(spec.Events, event)
	}
	for _, v := range structMap {
		spec.Structs = append(spec.Structs, v)
	}
	b, err := json.Marshal(spec)
	if err != nil {
		log.Panicf("fail to Marshal err:%v", err.Error())
	}
	ret := contract.Spec{}
	err = json.Unmarshal(b, &ret)
	if err != nil {
		log.Panicf("fail to Unmarshal err:%v", err.Error())
	}
	return ret
}

func NewTypeSpec(t abi.Type, structMap map[string]contract.StructSpec) contract.TypeSpec {
	switch t.T {
	case abi.ArrayTy, abi.SliceTy:
		return NewArrayTypeSpec(t, structMap)
	case abi.TupleTy:
		return NewStructTypeSpec(t, structMap)
	default:
		return NewPrimitiveTypeSpec(t)
	}
}

func NewPrimitiveTypeSpec(t abi.Type) contract.TypeSpec {
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
		log.Panicln("not supported type:", t)
	}
	return contract.TypeSpec{
		Name: s.String(),
	}
}

func NewStructTypeSpec(t abi.Type, structMap map[string]contract.StructSpec) contract.TypeSpec {
	s := contract.StructSpec{
		Name:   t.TupleRawName,
		Fields: make([]contract.NameAndTypeSpec, 0),
	}
	for i, n := range t.TupleRawNames {
		field := contract.NameAndTypeSpec{
			Name: n,
			Type: NewTypeSpec(*t.TupleElems[i], structMap),
		}
		s.Fields = append(s.Fields, field)
	}
	structMap[s.Name] = s
	return contract.TypeSpec{
		Name: s.Name,
	}
}

func NewArrayTypeSpec(t abi.Type, structMap map[string]contract.StructSpec) contract.TypeSpec {
	if t.T != abi.ArrayTy && t.T != abi.SliceTy {
		log.Panicln("invalid type:", t)
	}
	s := NewTypeSpec(*t.Elem, structMap)
	return contract.TypeSpec{
		Name:      s.Name,
		Dimension: s.Dimension + 1,
	}
}

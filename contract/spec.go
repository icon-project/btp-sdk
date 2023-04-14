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

package contract

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"

	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"
)

type TypeTag int64

const (
	TUnknown TypeTag = iota
	TVoid
	TInteger
	TBoolean
	TString
	TBytes
	TStruct
	TAddress
)

var (
	specLogger = log.New()
)

func init() {
	specLogger.SetLevel(log.DebugLevel)
}

func (t TypeTag) String() string {
	return typeIdToNames[t]
}

func (t TypeTag) Type() reflect.Type {
	return typeIdToType[t]
}

var (
	typeIdToNames = []string{"Unknown", "Void", "Integer", "Boolean", "String", "Bytes", "Struct", "Address"}
	nilInterface  interface{}
	typeIdToType  = []reflect.Type{
		nil,
		nil,
		reflect.TypeOf(Integer("")),
		reflect.TypeOf(Boolean(true)),
		reflect.TypeOf(String("")),
		reflect.TypeOf(Bytes("")),
		reflect.MapOf(reflect.TypeOf(""), reflect.TypeOf(&nilInterface)),
		reflect.TypeOf(Address("")),
	}
	signatureTypeNames = map[TypeTag]string{
		TInteger: "int",
		TBoolean: "bool",
		TString:  "str",
		TBytes:   "bytes",
		TStruct:  "struct",
		TAddress: "Address",
	}
	nameToTypeIds = map[string]TypeTag{
		"Void":    TVoid,
		"Integer": TInteger,
		"Boolean": TBoolean,
		"String":  TString,
		"Bytes":   TBytes,
		"Address": TAddress,
	}
)

func TypeIDByName(name string) TypeTag {
	if t, ok := nameToTypeIds[name]; ok {
		return t
	}
	return TUnknown
}

type Integer string

func (i Integer) clearPrefix() string {
	s := string(i)
	if strings.HasPrefix(s, "0x") {
		s = s[2:]
	}
	return s
}
func (i Integer) AsUint64() (uint64, error) {
	return strconv.ParseUint(i.clearPrefix(), 16, 64)
}

func (i Integer) AsInt64() (int64, error) {
	return strconv.ParseInt(i.clearPrefix(), 16, 64)
}

func (i Integer) AsBigInt() (*big.Int, error) {
	r, ok := new(big.Int), false
	if r, ok = r.SetString(i.clearPrefix(), 16); !ok {
		return nil, errors.New("fail to convert big.Int")
	}
	return r, nil
}
func FromUint64(i uint64) Integer {
	return Integer("0x" + strconv.FormatUint(i, 16))
}
func FromInt64(i int64) Integer {
	return Integer("0x" + strconv.FormatInt(i, 16))
}
func FromBigInt(i *big.Int) Integer {
	return Integer("0x" + hex.EncodeToString(i.Bytes()))
}

type Boolean bool
type String string
type Bytes []byte
type Address string

type HashValue interface {
	Match(v interface{}) bool
	Bytes() []byte
}

const (
	listDepthOffset = 4
	listDepthBits   = 4
	listDepthMask   = (1 << listDepthBits) - 1

	valueTagBits = listDepthOffset
	valueTagMask = (1 << valueTagBits) - 1
)

type DataType int64

func (dt DataType) Tag() TypeTag {
	return TypeTag(dt & valueTagMask)
}

type TypeSpec struct {
	Name      string `json:"name"`
	Dimension int    `json:"dimension,omitempty"`

	Type     reflect.Type `json:"-"`
	TypeID   TypeTag      `json:"-"`
	Resolved *StructSpec  `json:"-"`
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (s *TypeSpec) UnmarshalJSON(data []byte) error {
	type tSpec TypeSpec
	if err := json.Unmarshal(data, (*tSpec)(s)); err != nil {
		return err
	}
	s.TypeID = TypeIDByName(s.Name)
	return nil
}

func (s *TypeSpec) resolveType(structMap map[string]*StructSpec) error {
	if s.TypeID == TUnknown {
		v, ok := structMap[s.Name]
		if ok {
			s.TypeID = TStruct
			s.Resolved = v
		}
	}
	t := s.TypeID.Type()
	for i := 0; i < s.Dimension; i++ {
		t = reflect.SliceOf(t)
	}
	s.Type = t
	specLogger.Tracef("TypeSpec resolve name:%s type:%s dimension:%d resolved:%p goType:%v\n",
		s.Name, s.TypeID.String(), s.Dimension, s.Resolved, s.Type)
	return nil
}

type NameAndTypeSpec struct {
	Name     string   `json:"name"`
	Type     TypeSpec `json:"type"`
	Optional bool     `json:"optional,omitempty"`
}

type MethodSpec struct {
	Name     string            `json:"name"`
	Inputs   []NameAndTypeSpec `json:"inputs"`
	Output   TypeSpec          `json:"output"`
	Payable  bool              `json:"payable,omitempty"`
	ReadOnly bool              `json:"readOnly,omitempty"`

	InputMap map[string]*NameAndTypeSpec `json:"-"`
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (s *MethodSpec) UnmarshalJSON(data []byte) error {
	type tSpec MethodSpec
	if err := json.Unmarshal(data, (*tSpec)(s)); err != nil {
		return err
	}
	s.InputMap = make(map[string]*NameAndTypeSpec)
	for i := 0; i < len(s.Inputs); i++ {
		v := &s.Inputs[i]
		s.InputMap[v.Name] = v
	}
	return nil
}

func (s *MethodSpec) resolveType(structMap map[string]*StructSpec) error {
	for _, v := range s.InputMap {
		specLogger.Traceln("MethodSpec resolve input:", v.Name)
		if err := v.Type.resolveType(structMap); err != nil {
			return err
		}
	}
	specLogger.Traceln("MethodSpec resolve output:", s.Output.Name)
	return s.Output.resolveType(structMap)
}

type EventSpec struct {
	Name    string            `json:"name"`
	Indexed int               `json:"indexed,omitempty"`
	Inputs  []NameAndTypeSpec `json:"inputs"`

	InputMap  map[string]*NameAndTypeSpec `json:"-"`
	Signature string                      `json:"-"`
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (s *EventSpec) UnmarshalJSON(data []byte) error {
	type tSpec EventSpec
	if err := json.Unmarshal(data, (*tSpec)(s)); err != nil {
		return err
	}
	s.InputMap = make(map[string]*NameAndTypeSpec)
	for i := 0; i < len(s.Inputs); i++ {
		v := &s.Inputs[i]
		s.InputMap[v.Name] = v
	}
	return nil
}

func (s *EventSpec) resolveType(structMap map[string]*StructSpec) error {
	for _, v := range s.InputMap {
		specLogger.Traceln("EventSpec resolve input:", v.Name)
		if err := v.Type.resolveType(structMap); err != nil {
			return err
		}
	}

	inputTypes := make([]string, len(s.Inputs))
	for i, v := range s.Inputs {
		if inputType, ok := signatureTypeNames[v.Type.TypeID]; !ok {
			return errors.Errorf("invalid event type name:%s id:%s",
				v.Type.Name, v.Type.TypeID)
		} else {
			inputTypes[i] = inputType
		}
	}
	//FIXME event signature
	s.Signature = fmt.Sprintf("%s(%s)",
		s.Name, strings.Join(inputTypes, ","))
	return nil
}

type StructSpec struct {
	Name   string            `json:"name"`
	Fields []NameAndTypeSpec `json:"fields"`

	FieldMap map[string]*NameAndTypeSpec `json:"-"`
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (s *StructSpec) UnmarshalJSON(data []byte) error {
	type tSpec StructSpec
	if err := json.Unmarshal(data, (*tSpec)(s)); err != nil {
		return err
	}
	s.FieldMap = make(map[string]*NameAndTypeSpec)
	for i := 0; i < len(s.Fields); i++ {
		v := &s.Fields[i]
		s.FieldMap[v.Name] = v
	}
	return nil
}

func (s *StructSpec) resolveType(structMap map[string]*StructSpec) error {
	for _, v := range s.FieldMap {
		specLogger.Traceln("StructSpec resolve field:", v.Name)
		if err := v.Type.resolveType(structMap); err != nil {
			return err
		}
	}
	return nil
}

type Spec struct {
	SpecVersion string       `json:"specVersion"`
	Name        string       `json:"name"`
	Methods     []MethodSpec `json:"methods"`
	Events      []EventSpec  `json:"events"`
	Structs     []StructSpec `json:"structs"`

	MethodMap map[string]*MethodSpec `json:"-"`
	EventMap  map[string]*EventSpec  `json:"-"`
	StructMap map[string]*StructSpec `json:"-"`
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (s *Spec) UnmarshalJSON(data []byte) error {
	type tSpec Spec
	if err := json.Unmarshal(data, (*tSpec)(s)); err != nil {
		return err
	}

	s.StructMap = make(map[string]*StructSpec)
	for i := 0; i < len(s.Structs); i++ {
		s.StructMap[s.Structs[i].Name] = &s.Structs[i]
	}
	for _, v := range s.StructMap {
		specLogger.Tracef("StructSpec resolve name:%s ptr:%p\n", v.Name, v)
		if err := v.resolveType(s.StructMap); err != nil {
			return err
		}
	}
	s.MethodMap = make(map[string]*MethodSpec)
	for i := 0; i < len(s.Methods); i++ {
		v := &s.Methods[i]
		s.MethodMap[v.Name] = v
		specLogger.Tracef("MethodSpec resolve name:%s readonly:%v\n", v.Name, v.ReadOnly)
		if err := v.resolveType(s.StructMap); err != nil {
			return err
		}
	}
	s.EventMap = make(map[string]*EventSpec)
	for i := 0; i < len(s.Events); i++ {
		v := &s.Events[i]
		s.EventMap[v.Name] = v
		specLogger.Tracef("EventSpec resolve name:%s indexed:%v\n", v.Name, v.Indexed)
		if err := v.resolveType(s.StructMap); err != nil {
			return err
		}
	}
	return nil
}

func ParamsTypeCheck(s *EventSpec, params Params) error {
	if len(params) != len(s.InputMap) {
		return errors.Errorf("invalid length params")
	}
	for k, v := range params {
		spec, ok := s.InputMap[k]
		if !ok {
			return errors.Errorf("not found param name:%s", k)
		}
		return typeCheck(spec, v)
	}
	return nil
}

func typeCheck(s *NameAndTypeSpec, value interface{}) error {
	specLogger.Traceln("typeCheck name:", s.Name, "typeName:", s.Type.Name, "type:", s.Type.TypeID.String(),
		"reflect:", s.Type.Type)
	if s.Type.Dimension > 0 {
		return errors.New("not implemented")
	} else {
		switch s.Type.TypeID {
		case TStruct:
			return structTypeCheck(s, value)
		default:
			return primitiveTypeCheck(s, value)
		}
	}
}

func primitiveTypeCheck(s *NameAndTypeSpec, value interface{}) error {
	specLogger.Traceln("primitiveTypeCheck name:", s.Name, "typeName:", s.Type.Name,
		"type:", s.Type.TypeID.String(), "reflect:", s.Type.TypeID.Type(), value)
	ok := false
	switch s.Type.TypeID {
	case TInteger:
		_, ok = value.(Integer)
	case TString:
		_, ok = value.(String)
	case TAddress:
		_, ok = value.(Address)
	case TBytes:
		_, ok = value.(Bytes)
	case TBoolean:
		_, ok = value.(Boolean)
	default:
		return errors.Errorf("not supported param type name:%s typeID:%v", s.Name, s.Type.TypeID.String())
	}
	if !ok {
		return errors.Errorf("invalid param type name:%s expected:%s actual:%T",
			s.Name, s.Type.TypeID.String(), value)
	}
	return nil
}

func structTypeCheck(s *NameAndTypeSpec, value interface{}) error {
	specLogger.Traceln("structTypeCheck name:", s.Name, "typeName:", s.Type.Name, value)
	m, ok := value.(map[string]interface{})
	if !ok {
		return errors.Errorf("invalid param type name:%s, expected:%s actual:%T",
			s.Name, s.Type.TypeID.String(), value)
	}
	for k, v := range m {
		spec, ok := s.Type.Resolved.FieldMap[k]
		if !ok {
			return errors.Errorf("not found param name:%s", k)
		}
		if err := typeCheck(spec, v); err != nil {
			return err
		}
	}
	return nil
}

func StructToMap(obj interface{}) map[string]interface{} {
	v := reflect.ValueOf(obj)
	t := v.Type()
	if t.Kind() != reflect.Struct {
		panic("invalid type")
	}
	ret := make(map[string]interface{})
	for i := 0; i < v.NumField(); i++ {
		ft := t.Field(i)
		name := strings.ToLower(ft.Name[:1]) + ft.Name[1:]
		fv := v.Field(i).Interface()
		if ft.Type.Kind() == reflect.Struct {
			ret[name] = StructToMap(fv)
		} else {
			ret[name] = fv
		}
	}
	return ret
}

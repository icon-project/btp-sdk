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
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/intconv"
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
	typeIdToType  = []reflect.Type{
		nil,
		nil,
		reflect.TypeOf(Integer("")),
		reflect.TypeOf(Boolean(true)),
		reflect.TypeOf(String("")),
		reflect.TypeOf(Bytes([]byte{})),
		reflect.TypeOf(Struct{}),
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

func TypeIDByType(v reflect.Type) TypeTag {
	for i, t := range typeIdToType[TInteger:] {
		if t == v {
			return TypeTag(int(TInteger) + i)
		}
	}
	return TUnknown
}

func TypeIDOf(value interface{}) TypeTag {
	if value == nil {
		return TVoid
	}
	return TypeIDByType(reflect.TypeOf(value))
}

type Integer string

func (i Integer) AsUint64() (uint64, error) {
	bi, err := i.AsBigInt()
	if err != nil {
		return 0, err
	}
	if !bi.IsUint64() {
		return 0, errors.New("cannot convert to uint64")
	}
	return bi.Uint64(), nil
}

func (i Integer) AsInt64() (int64, error) {
	bi, err := i.AsBigInt()
	if err != nil {
		return 0, err
	}
	if !bi.IsInt64() {
		return 0, errors.New("cannot convert to uint64")
	}
	return bi.Int64(), nil
}

func (i Integer) AsBigInt() (*big.Int, error) {
	bi := new(big.Int)
	if err := intconv.ParseBigInt(bi, string(i)); err != nil {
		return nil, err
	}
	return bi, nil
}

func (i Integer) AsBytes() ([]byte, error) {
	bi, err := i.AsBigInt()
	if err != nil {
		return nil, err
	}
	return intconv.BigIntToBytes(bi), nil
}

func (i Integer) MarshalBinary() (data []byte, err error) {
	return i.AsBytes()
}

func (i *Integer) UnmarshalBinary(data []byte) error {
	ci, err := IntegerOf(data)
	if err != nil {
		return err
	}
	*i = ci
	return nil
}

type Boolean bool
type String string
type Bytes []byte
type Address string
type Struct struct {
	Name   string
	Fields []KeyValue
}

func (s Struct) Params() Params {
	ret := make(Params)
	for _, f := range s.Fields {
		ret[f.Key] = f.Value
	}
	return ret
}

type KeyValue struct {
	Key   string
	Value interface{}
}
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

	Type         reflect.Type `json:"-"`
	TypeID       TypeTag      `json:"-"`
	Resolved     *StructSpec  `json:"-"`
	ResolvedType reflect.Type `json:"-"`
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
			if v.Type == nil {
				if err := v.resolveType(structMap); err != nil {
					return err
				}
			}
			s.ResolvedType = v.Type
			for i := 0; i < s.Dimension; i++ {
				s.ResolvedType = reflect.SliceOf(s.ResolvedType)
			}
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
	Name     string             `json:"name"`
	Inputs   []*NameAndTypeSpec `json:"inputs"`
	Output   TypeSpec           `json:"output"`
	Payable  bool               `json:"payable,omitempty"`
	ReadOnly bool               `json:"readOnly,omitempty"`

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
		v := s.Inputs[i]
		if old, exists := s.InputMap[v.Name]; exists {
			specLogger.Warnf("MethodSpec overwrite name:%s input:%+v", s.Name, old)
		}
		s.InputMap[v.Name] = v
	}
	return nil
}

func (s *MethodSpec) resolveType(structMap map[string]*StructSpec) error {
	for _, v := range s.InputMap {
		specLogger.Tracef("MethodSpec resolve name:%s input:%s", s.Name, v.Name)
		if err := v.Type.resolveType(structMap); err != nil {
			return err
		}
	}
	specLogger.Tracef("MethodSpec resolve name:%s output:%s", s.Name, s.Output.Name)
	return s.Output.resolveType(structMap)
}

type EventSpec struct {
	Name    string             `json:"name"`
	Indexed int                `json:"indexed,omitempty"`
	Inputs  []*NameAndTypeSpec `json:"inputs"`

	InputMap    map[string]*NameAndTypeSpec `json:"-"`
	Signature   string                      `json:"-"`
	NameToIndex map[string]int              `json:"-"`
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (s *EventSpec) UnmarshalJSON(data []byte) error {
	type tSpec EventSpec
	if err := json.Unmarshal(data, (*tSpec)(s)); err != nil {
		return err
	}
	s.InputMap = make(map[string]*NameAndTypeSpec)
	s.NameToIndex = make(map[string]int)
	for i := 0; i < len(s.Inputs); i++ {
		v := s.Inputs[i]
		if old, exists := s.InputMap[v.Name]; exists {
			specLogger.Warnf("EventSpec overwrite name:%s input:%+v", s.Name, old)
		}
		s.InputMap[v.Name] = v
		s.NameToIndex[v.Name] = i
	}
	return nil
}

func (s *EventSpec) resolveType(structMap map[string]*StructSpec) error {
	for _, v := range s.InputMap {
		specLogger.Tracef("EventSpec resolve name:%s input:%s", s.Name, v.Name)
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
			inputTypes[i] = inputType + strings.Repeat("[]", v.Type.Dimension)
		}
	}
	s.Signature = fmt.Sprintf("%s(%s)",
		s.Name, strings.Join(inputTypes, ","))
	return nil
}

type StructSpec struct {
	Name   string             `json:"name"`
	Fields []*NameAndTypeSpec `json:"fields"`

	FieldMap map[string]*NameAndTypeSpec `json:"-"`
	Type     reflect.Type                `json:"-"`
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (s *StructSpec) UnmarshalJSON(data []byte) error {
	type tSpec StructSpec
	if err := json.Unmarshal(data, (*tSpec)(s)); err != nil {
		return err
	}
	s.FieldMap = make(map[string]*NameAndTypeSpec)
	for i := 0; i < len(s.Fields); i++ {
		v := s.Fields[i]
		if old, exists := s.FieldMap[v.Name]; exists {
			specLogger.Warnf("StructSpec overwrite name:%s input:%+v", s.Name, old)
		}
		s.FieldMap[v.Name] = v
	}
	return nil
}

func (s *StructSpec) resolveType(structMap map[string]*StructSpec) error {
	for _, v := range s.FieldMap {
		specLogger.Tracef("StructSpec resolve name:%s field:%s", s.Name, v.Name)
		if err := v.Type.resolveType(structMap); err != nil {
			return err
		}
	}
	fields := make([]reflect.StructField, 0)
	for _, v := range s.Fields {
		f := reflect.StructField{
			Name: strings.ToUpper(v.Name[:1]) + v.Name[1:],
			Type: v.Type.Type,
			Tag:  reflect.StructTag("json:\"" + v.Name + "\""),
		}
		fields = append(fields, f)
	}
	s.Type = reflect.StructOf(fields)
	return nil
}

type Spec struct {
	SpecVersion string        `json:"specVersion"`
	Name        string        `json:"name"`
	Methods     []*MethodSpec `json:"methods"`
	Events      []*EventSpec  `json:"events"`
	Structs     []*StructSpec `json:"structs"`

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
		v := s.Structs[i]
		if old, exists := s.StructMap[v.Name]; exists {
			specLogger.Warnf("StructSpec overwrite %d name:%s fields:%v", i, old.Name, old.Fields)
		}
		s.StructMap[v.Name] = v
	}
	for _, v := range s.StructMap {
		if err := v.resolveType(s.StructMap); err != nil {
			return err
		}
	}
	s.MethodMap = make(map[string]*MethodSpec)
	for i := 0; i < len(s.Methods); i++ {
		v := s.Methods[i]
		if old, exists := s.MethodMap[v.Name]; exists {
			specLogger.Warnf("MethodSpec overwrite name:%s inputs:%v output:%v readonly:%v payable:%v",
				old.Name, old.Inputs, old.Output, old.ReadOnly, old.Payable)
		}
		s.MethodMap[v.Name] = v
		if err := v.resolveType(s.StructMap); err != nil {
			return err
		}
	}
	s.EventMap = make(map[string]*EventSpec)
	for i := 0; i < len(s.Events); i++ {
		v := s.Events[i]
		if old, exists := s.EventMap[v.Name]; exists {
			specLogger.Warnf("EventSpec overwrite name:%s inputs:%v indexed:%v",
				old.Name, old.Inputs, old.Indexed)
		}
		s.EventMap[v.Name] = v
		if err := v.resolveType(s.StructMap); err != nil {
			return err
		}
	}
	return nil
}

func EqualsTypeSpec(a, b TypeSpec) bool {
	if a.Dimension != b.Dimension {
		return false
	}
	if a.TypeID != b.TypeID {
		return false
	}
	switch a.TypeID {
	case TUnknown:
		return a.Name == b.Name
	case TStruct:
		//ignore StructSpec.Name
		return EqualsNameAndTypes(a.Resolved.FieldMap, b.Resolved.FieldMap)
	default:
		return true
	}
}

func EqualsNameAndTypes(a, b map[string]*NameAndTypeSpec) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		if !EqualsTypeSpec(av.Type, bv.Type) {
			return false
		}
		//TODO NameAndTypeSpec.Optional
	}
	return true
}

func OptionalInputs(a, b map[string]*NameAndTypeSpec) (map[string]*NameAndTypeSpec, error) {
	r := make(map[string]*NameAndTypeSpec)
	for k, v := range a {
		bv, ok := b[k]
		if ok {
			if !EqualsTypeSpec(v.Type, bv.Type) {
				return nil, errors.Errorf("mismatch input name:%s", k)
			}
		} else {
			r[k] = v
		}
	}
	for k, v := range b {
		if _, ok := a[k]; !ok {
			r[k] = v
		}
	}
	return r, nil
}

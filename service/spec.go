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
	"strings"

	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
)

var (
	specLogger = log.New()
)

func init() {
	specLogger.SetLevel(log.DebugLevel)
}

type Spec struct {
	SpecVersion  string                 `json:"specVersion"`
	Name         string                 `json:"name"`
	NetworkTypes []string               `json:"networkTypes"`
	Methods      map[string]*MethodSpec `json:"methods"`
	Events       map[string]*EventSpec  `json:"events"`
	Structs      map[string]*StructSpec `json:"structs"`
}

func (s *Spec) Merge(cs contract.Spec, networkTypes ...string) {
	for name, cm := range cs.MethodMap {
		if sm, ok := s.Methods[name]; ok {
			f := sm.merge(cm, networkTypes...)
			if f.Inputs() {
				s.mergeStructs(cm.InputMap, networkTypes...)
			}
			if f.Output() {
				s.mergeStruct(cm.Output, networkTypes...)
			}
		} else {
			s.Methods[name] = NewMethodSpec(cm, networkTypes...)
		}
	}
	for name, ce := range cs.EventMap {
		if se, ok := s.Events[name]; ok {
			f := se.merge(ce, networkTypes...)
			if f.Inputs() {
				s.mergeStructs(ce.InputMap, networkTypes...)
			}
		} else {
			s.Events[name] = NewEventSpec(ce, networkTypes...)
		}
	}
	StringSetMerge(s.NetworkTypes, networkTypes)
}

type MergeInfo struct {
	Spec *contract.Spec
	Flag interface{}
}

func (s *Spec) MergeOverloads(miMap map[string]MergeInfo) error {
	for name, mi := range miMap {
		switch f := mi.Flag.(type) {
		case MethodOverloadFlag:
			sm := s.Methods[name]
			if sm == nil {
				return errors.Errorf("not found method:%s in service spec", name)
			}
			cm := mi.Spec.MethodMap[name]
			if cm == nil {
				return errors.Errorf("not found method:%s in contract spec", name)
			}
			if err := sm.mergeOverloads(cm, f); err != nil {
				return err
			}
		case EventOverloadFlag:
			se := s.Events[name]
			if se == nil {
				return errors.Errorf("not found event:%s in service spec", name)
			}
			ce := mi.Spec.EventMap[name]
			if ce == nil {
				return errors.Errorf("not found event:%s in contract spec", name)
			}
			if err := se.mergeOverloads(ce, f); err != nil {
				return err
			}
		default:
			return errors.Errorf("not support flag type %T", mi.Flag)
		}
	}
	return nil
}

func (s *Spec) mergeStructs(m map[string]*contract.NameAndTypeSpec, networkTypes ...string) {
	for _, v := range m {
		s.mergeStruct(v.Type, networkTypes...)
	}
}

func (s *Spec) mergeStruct(cs contract.TypeSpec, networkTypes ...string) {
	if cs.TypeID == contract.TStruct && cs.Resolved != nil {
		rcs := cs.Resolved
		equals := false
		if ss, ok := s.Structs[rcs.Name]; ok {
			equals = ss.merge(rcs, networkTypes...)
		} else {
			s.Structs[cs.Name] = NewStructSpec(rcs, networkTypes...)
		}
		if !equals {
			s.mergeStructs(rcs.FieldMap, networkTypes...)
		}
	}
}

func (s *Spec) ResetStructs() {
	s.Structs = make(map[string]*StructSpec)
	for _, sm := range s.Methods {
		s.mergeStructs(sm.Inputs, sm.NetworkTypes...)
		s.mergeStruct(sm.Output, sm.NetworkTypes...)
		for _, o := range sm.Overloads {
			if o.Inputs != nil {
				s.mergeStructs(*o.Inputs, o.NetworkTypes...)
			}
			if o.Output != nil {
				s.mergeStruct(*o.Output, o.NetworkTypes...)
			}
		}
	}
	for _, se := range s.Events {
		s.mergeStructs(se.Inputs, se.NetworkTypes...)
		for _, o := range se.Overloads {
			if o.Inputs != nil {
				s.mergeStructs(*o.Inputs, o.NetworkTypes...)
			}
		}
	}
}

type MethodSpec struct {
	NetworkTypes []string                             `json:"networkTypes"`
	Name         string                               `json:"name"`
	Inputs       map[string]*contract.NameAndTypeSpec `json:"inputs"`
	Output       contract.TypeSpec                    `json:"output"`
	Payable      bool                                 `json:"payable"`
	Readonly     bool                                 `json:"readonly"`
	Overloads    []*MethodOverload                    `json:"overloads,omitempty"`
}

func (s *MethodSpec) IsSupport(networkType string) bool {
	return StringSetContains(s.NetworkTypes, networkType)
}

func (s *MethodSpec) Overload(networkType string) *MethodOverload {
	for _, o := range s.Overloads {
		if o.IsSupport(networkType) {
			return o
		}
	}
	return nil
}

func (s *MethodSpec) merge(cs *contract.MethodSpec, networkTypes ...string) MethodOverloadFlag {
	f := CompareMethodSpec(s, cs)
	if f.None() {
		for _, networkType := range networkTypes {
			if !s.IsSupport(networkType) {
				if o := s.Overload(networkType); o != nil {
					o.NetworkTypes, _ = StringSetRemove(o.NetworkTypes, networkType)
					specLogger.Tracef("MethodSpec:%s remove overload networkType:%s", s.Name, networkType)
				}
				s.NetworkTypes = append(s.NetworkTypes, networkType)
				specLogger.Tracef("MethodSpec:%s append networkType:%s", s.Name, networkType)
			}
		}
	} else {
		for _, networkType := range networkTypes {
			if s.IsSupport(networkType) {
				s.NetworkTypes, _ = StringSetRemove(s.NetworkTypes, networkType)
				specLogger.Tracef("MethodSpec:%s remove networkType:%s", s.Name, networkType)
			}
		}
		var to *MethodOverload
		for _, o := range s.Overloads {
			if CompareMethodOverload(o, cs) {
				to = o
				break
			}
		}
		if to != nil {
			to.NetworkTypes = StringSetMerge(to.NetworkTypes, networkTypes)
			specLogger.Tracef("MethodSpec:%s add overload networkTypes:%s", s.Name, strings.Join(networkTypes, ","))
		} else {
			to = NewMethodOverload(f, cs, networkTypes...)
			s.Overloads = append(s.Overloads, to)
			specLogger.Tracef("MethodSpec:%s new overload networkTypes:%s", s.Name, strings.Join(networkTypes, ","))
		}
	}
	return f
}

func (s *MethodSpec) mergeOverloads(cs *contract.MethodSpec, f MethodOverloadFlag) error {
	supports := make([]string, 0)
	for _, o := range s.Overloads {
		if !f.None() {
			of := FlagFromMethodOverload(*o)
			if of != f {
				return errors.Errorf("not compatible method:%s overloads expected:%d actual:%d", s.Name, f, of)
			}
		}
		supports = append(supports, o.NetworkTypes...)
	}
	s.NetworkTypes = append(s.NetworkTypes, supports...)
	of := CompareMethodSpec(s, cs)
	if f.None() || (f.Inputs() && of.Inputs()) {
		s.Inputs = NewNameAndTypeSpecMap(cs.InputMap)
	}
	if f.None() || (f.Output() && of.Output()) {
		s.Output = cs.Output
	}
	if f.None() || (f.Payable() && of.Payable()) {
		s.Payable = cs.Payable
	}
	if f.None() || (f.Readonly() && of.Readonly()) {
		s.Readonly = cs.ReadOnly
	}
	s.Overloads = s.Overloads[:0]
	return nil
}

type MethodOverload struct {
	NetworkTypes []string                              `json:"networkTypes"`
	Inputs       *map[string]*contract.NameAndTypeSpec `json:"inputs,omitempty"`
	Output       *contract.TypeSpec                    `json:"output,omitempty"`
	Payable      *bool                                 `json:"payable,omitempty"`
	Readonly     *bool                                 `json:"readonly,omitempty"`
}

func (o *MethodOverload) IsSupport(networkType string) bool {
	return StringSetContains(o.NetworkTypes, networkType)
}

type EventSpec struct {
	NetworkTypes []string                             `json:"networkTypes"`
	Name         string                               `json:"name"`
	Inputs       map[string]*contract.NameAndTypeSpec `json:"inputs"`
	Indexed      []string                             `json:"indexed"`
	Overloads    []*EventOverload                     `json:"overloads,omitempty"`
}

func (s *EventSpec) IsSupport(networkType string) bool {
	return StringSetContains(s.NetworkTypes, networkType)
}

func (s *EventSpec) Overload(networkType string) *EventOverload {
	for _, o := range s.Overloads {
		if o.IsSupport(networkType) {
			return o
		}
	}
	return nil
}

func (s *EventSpec) merge(cs *contract.EventSpec, networkTypes ...string) EventOverloadFlag {
	f := CompareEventSpec(s, cs)
	if f.None() {
		for _, networkType := range networkTypes {
			if !s.IsSupport(networkType) {
				if o := s.Overload(networkType); o != nil {
					o.NetworkTypes, _ = StringSetRemove(o.NetworkTypes, networkType)
					specLogger.Tracef("EventSpec:%s remove overload networkType:%s", s.Name, networkType)
				} else {
					s.NetworkTypes = append(s.NetworkTypes, networkType)
					specLogger.Tracef("EventSpec:%s append networkType:%s", s.Name, networkType)
				}
			}
		}
	} else {
		for _, networkType := range networkTypes {
			if s.IsSupport(networkType) {
				s.NetworkTypes, _ = StringSetRemove(s.NetworkTypes, networkType)
				specLogger.Tracef("EventSpec:%s remove networkType:%s", s.Name, networkType)
			}
		}
		var to *EventOverload
		for _, o := range s.Overloads {
			if CompareEventOverload(o, cs) {
				to = o
				break
			}
		}
		if to != nil {
			to.NetworkTypes = StringSetMerge(to.NetworkTypes, networkTypes)
			specLogger.Tracef("EventSpec:%s add overload networkTypes:%s", s.Name, strings.Join(networkTypes, ","))
		} else {
			to = NewEventOverload(f, cs, networkTypes...)
			s.Overloads = append(s.Overloads, to)
			specLogger.Tracef("EventSpec:%s new overload networkTypes:%s", s.Name, strings.Join(networkTypes, ","))
		}
	}
	return f
}

func (s *EventSpec) mergeOverloads(cs *contract.EventSpec, f EventOverloadFlag) error {
	supports := make([]string, 0)
	for _, o := range s.Overloads {
		if !f.None() {
			of := FlagFromEventOverload(*o)
			if of != f {
				return errors.Errorf("not compatible event:%s overloads expected:%d actual:%d", s.Name, f, of)
			}
		}
		supports = append(supports, o.NetworkTypes...)
	}
	s.NetworkTypes = append(s.NetworkTypes, supports...)
	of := CompareEventSpec(s, cs)
	if f.None() || (f.Inputs() && of.Inputs()) {
		s.Inputs = NewNameAndTypeSpecMap(cs.InputMap)
	}
	if f.None() || (f.Indexed() && of.Indexed()) {
		s.Indexed = NewEventSpecIndexed(cs)
	}
	s.Overloads = s.Overloads[:0]
	return nil
}

type EventOverload struct {
	NetworkTypes []string                              `json:"networkTypes"`
	Inputs       *map[string]*contract.NameAndTypeSpec `json:"inputs,omitempty"`
	Indexed      *[]string                             `json:"indexed,omitempty"`
}

func (o *EventOverload) IsSupport(networkType string) bool {
	return StringSetContains(o.NetworkTypes, networkType)
}

type StructSpec struct {
	NetworkTypes []string                             `json:"networkTypes"`
	Name         string                               `json:"name"`
	Fields       map[string]*contract.NameAndTypeSpec `json:"fields"`
	Conflicts    []*StructConflict                    `json:"conflicts,omitempty"`
}

func (s *StructSpec) IsSupport(networkType string) bool {
	return StringSetContains(s.NetworkTypes, networkType)
}

func (s *StructSpec) Conflict(networkType string) *StructConflict {
	for _, c := range s.Conflicts {
		if c.IsSupport(networkType) {
			return c
		}
	}
	return nil
}

func (s *StructSpec) merge(cs *contract.StructSpec, networkTypes ...string) bool {
	equals := EqualsNameAndTypes(s.Fields, cs.FieldMap)
	if equals {
		for _, networkType := range networkTypes {
			if !s.IsSupport(networkType) {
				if o := s.Conflict(networkType); o != nil {
					o.NetworkTypes, _ = StringSetRemove(o.NetworkTypes, networkType)
					specLogger.Tracef("StructSpec:%s remove overload networkType:%s", s.Name, networkType)
				}
				s.NetworkTypes = append(s.NetworkTypes, networkType)
				specLogger.Tracef("StructSpec:%s append networkType:%s", s.Name, networkType)
			}
		}
	} else {
		for _, networkType := range networkTypes {
			if s.IsSupport(networkType) {
				s.NetworkTypes, _ = StringSetRemove(s.NetworkTypes, networkType)
				specLogger.Tracef("StructSpec:%s remove networkType:%s", s.Name, networkType)
			}
		}
		var tc *StructConflict
		for _, c := range s.Conflicts {
			if EqualsNameAndTypes(c.Fields, cs.FieldMap) {
				tc = c
				break
			}
		}
		if tc != nil {
			tc.NetworkTypes = StringSetMerge(tc.NetworkTypes, networkTypes)
			specLogger.Tracef("StructSpec:%s add conflict networkTypes:%s", s.Name, strings.Join(networkTypes, ","))
		} else {
			tc = NewStructConflict(cs, networkTypes...)
			s.Conflicts = append(s.Conflicts, tc)
			specLogger.Tracef("StructSpec:%s new conflict networkTypes:%s", s.Name, strings.Join(networkTypes, ","))
		}
	}
	return equals
}

type StructConflict struct {
	NetworkTypes []string                             `json:"networkTypes"`
	Fields       map[string]*contract.NameAndTypeSpec `json:"fields"`
}

func (c *StructConflict) IsSupport(networkType string) bool {
	return StringSetContains(c.NetworkTypes, networkType)
}

func NewSpec(version, name string) Spec {
	return Spec{
		SpecVersion: version,
		Name:        name,
		Methods:     make(map[string]*MethodSpec),
		Events:      make(map[string]*EventSpec),
		Structs:     make(map[string]*StructSpec),
	}
}

func CopySpec(s Spec) Spec {
	cs := NewSpec(s.SpecVersion, s.Name)
	for k, v := range s.Methods {
		cs.Methods[k] = CopyMethodSpec(v)
	}
	for k, v := range s.Events {
		cs.Events[k] = CopyEventSpec(v)
	}
	for k, v := range s.Structs {
		cs.Structs[k] = CopyStructSpec(v)
	}
	return cs
}

func NewNameAndTypeSpecMap(m map[string]*contract.NameAndTypeSpec) map[string]*contract.NameAndTypeSpec {
	r := make(map[string]*contract.NameAndTypeSpec)
	for k, v := range m {
		r[k] = &contract.NameAndTypeSpec{
			Name:     v.Name,
			Type:     v.Type,
			Optional: v.Optional,
		}
	}
	return r
}

func NewMethodSpec(cs *contract.MethodSpec, networkTypes ...string) *MethodSpec {
	s := &MethodSpec{
		Name:      cs.Name,
		Inputs:    NewNameAndTypeSpecMap(cs.InputMap),
		Output:    cs.Output,
		Payable:   cs.Payable,
		Readonly:  cs.ReadOnly,
		Overloads: make([]*MethodOverload, 0),
	}
	s.NetworkTypes = append(s.NetworkTypes, networkTypes...)
	return s
}

func CopyMethodSpec(s *MethodSpec) *MethodSpec {
	cs := &MethodSpec{
		Name:      s.Name,
		Inputs:    NewNameAndTypeSpecMap(s.Inputs),
		Output:    s.Output,
		Payable:   s.Payable,
		Readonly:  s.Readonly,
		Overloads: make([]*MethodOverload, 0),
	}
	cs.NetworkTypes = append(cs.NetworkTypes, s.NetworkTypes...)
	for _, v := range s.Overloads {
		cs.Overloads = append(cs.Overloads, CopyMethodOverload(v))
	}
	return cs
}

func NewMethodOverload(f MethodOverloadFlag, cs *contract.MethodSpec, networkTypes ...string) *MethodOverload {
	o := &MethodOverload{}
	o.NetworkTypes = append(o.NetworkTypes, networkTypes...)
	if f.Inputs() {
		v := NewNameAndTypeSpecMap(cs.InputMap)
		o.Inputs = &v
	}
	if f.Output() {
		v := cs.Output
		o.Output = &v
	}
	if f.Payable() {
		v := cs.Payable
		o.Payable = &v
	}
	if f.Readonly() {
		v := cs.ReadOnly
		o.Readonly = &v
	}
	return o
}

func CopyMethodOverload(o *MethodOverload) *MethodOverload {
	co := &MethodOverload{}
	co.NetworkTypes = append(co.NetworkTypes, o.NetworkTypes...)
	if o.Inputs != nil {
		v := NewNameAndTypeSpecMap(*o.Inputs)
		co.Inputs = &v
	}
	if o.Output != nil {
		//FIXME co.Output = o.Output.Copy() // contract.TypeSpec.Copy()
		co.Output = o.Output
	}
	if o.Payable != nil {
		v := *o.Payable
		co.Payable = &v
	}
	if o.Readonly != nil {
		v := *o.Readonly
		co.Readonly = &v
	}
	return co
}

func NewEventSpecIndexed(spec *contract.EventSpec) []string {
	indexed := make([]string, 0)
	for i := 0; i < spec.Indexed; i++ {
		indexed = append(indexed, spec.Inputs[i].Name)
	}
	return indexed
}

func NewEventSpec(cs *contract.EventSpec, networkTypes ...string) *EventSpec {
	s := &EventSpec{
		Name:      cs.Name,
		Inputs:    NewNameAndTypeSpecMap(cs.InputMap),
		Indexed:   NewEventSpecIndexed(cs),
		Overloads: make([]*EventOverload, 0),
	}
	s.NetworkTypes = append(s.NetworkTypes, networkTypes...)
	return s
}

func CopyEventSpec(s *EventSpec) *EventSpec {
	cs := &EventSpec{
		Name:      s.Name,
		Inputs:    NewNameAndTypeSpecMap(s.Inputs),
		Overloads: make([]*EventOverload, 0),
	}
	cs.NetworkTypes = append(cs.NetworkTypes, s.NetworkTypes...)
	cs.Indexed = append(cs.Indexed, s.Indexed...)
	for _, v := range s.Overloads {
		cs.Overloads = append(cs.Overloads, CopyEventOverload(v))
	}
	return cs
}

func NewEventOverload(f EventOverloadFlag, cs *contract.EventSpec, networkTypes ...string) *EventOverload {
	o := &EventOverload{}
	o.NetworkTypes = append(o.NetworkTypes, networkTypes...)
	if f.Inputs() {
		v := NewNameAndTypeSpecMap(cs.InputMap)
		o.Inputs = &v
	}
	if f.Indexed() {
		v := NewEventSpecIndexed(cs)
		o.Indexed = &v
	}
	return o
}

func CopyEventOverload(o *EventOverload) *EventOverload {
	co := &EventOverload{}
	co.NetworkTypes = append(co.NetworkTypes, o.NetworkTypes...)
	if o.Inputs != nil {
		v := NewNameAndTypeSpecMap(*o.Inputs)
		co.Inputs = &v
	}
	if o.Indexed != nil {
		v := make([]string, 0)
		copy(v, *co.Indexed)
		co.Indexed = &v
	}
	return co
}

func NewStructSpec(cs *contract.StructSpec, networkTypes ...string) *StructSpec {
	s := &StructSpec{
		Name:      cs.Name,
		Fields:    NewNameAndTypeSpecMap(cs.FieldMap),
		Conflicts: make([]*StructConflict, 0),
	}
	s.NetworkTypes = append(s.NetworkTypes, networkTypes...)
	return s
}

func CopyStructSpec(s *StructSpec) *StructSpec {
	cs := &StructSpec{
		Name:      s.Name,
		Fields:    NewNameAndTypeSpecMap(s.Fields),
		Conflicts: make([]*StructConflict, 0),
	}
	cs.NetworkTypes = append(cs.NetworkTypes, s.NetworkTypes...)
	for _, v := range s.Conflicts {
		cs.Conflicts = append(cs.Conflicts, CopyStructConflict(v))
	}
	return cs
}

func NewStructConflict(spec *contract.StructSpec, networkTypes ...string) *StructConflict {
	c := &StructConflict{
		Fields: NewNameAndTypeSpecMap(spec.FieldMap),
	}
	c.NetworkTypes = append(c.NetworkTypes, networkTypes...)
	return c
}

func CopyStructConflict(c *StructConflict) *StructConflict {
	cc := &StructConflict{
		Fields: NewNameAndTypeSpecMap(c.Fields),
	}
	cc.NetworkTypes = append(cc.NetworkTypes, c.NetworkTypes...)
	return cc
}

func EqualsNameAndTypes(a map[string]*contract.NameAndTypeSpec, b map[string]*contract.NameAndTypeSpec) bool {
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

func EqualsEventIndexed(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
aLoop:
	for _, av := range a {
		for _, bv := range b {
			if av == bv {
				continue aLoop
			}
		}
		return false
	}
	return true
}

func EqualsTypeSpec(a contract.TypeSpec, b contract.TypeSpec) bool {
	if a.Dimension != b.Dimension {
		return false
	}
	if a.TypeID != b.TypeID {
		return false
	}
	switch a.TypeID {
	case contract.TUnknown:
		return a.Name == b.Name
	case contract.TStruct:
		//ignore StructSpec.Name
		return EqualsNameAndTypes(a.Resolved.FieldMap, b.Resolved.FieldMap)
	default:
		return true
	}
}

const (
	MethodOverloadNone   MethodOverloadFlag = 0
	MethodOverloadInputs MethodOverloadFlag = 1 << iota
	MethodOverloadOutput
	MethodOverloadPayable
	MethodOverloadReadonly
)

type MethodOverloadFlag int

func (f MethodOverloadFlag) None() bool {
	return f == MethodOverloadNone
}
func (f MethodOverloadFlag) Inputs() bool {
	return f&MethodOverloadInputs > 0
}

func (f MethodOverloadFlag) Output() bool {
	return f&MethodOverloadOutput > 0
}

func (f MethodOverloadFlag) Payable() bool {
	return f&MethodOverloadPayable > 0
}

func (f MethodOverloadFlag) Readonly() bool {
	return f&MethodOverloadReadonly > 0
}

func CompareMethodSpec(ss *MethodSpec, cs *contract.MethodSpec) MethodOverloadFlag {
	f := MethodOverloadNone
	if !EqualsNameAndTypes(ss.Inputs, cs.InputMap) {
		f |= MethodOverloadInputs
	}
	if !EqualsTypeSpec(ss.Output, cs.Output) {
		f |= MethodOverloadOutput
	}
	if ss.Payable != cs.Payable {
		f |= MethodOverloadPayable
	}
	if ss.Readonly != cs.ReadOnly {
		f |= MethodOverloadReadonly
	}
	return f
}

func CompareMethodOverload(o *MethodOverload, cs *contract.MethodSpec) bool {
	if o.Inputs != nil && !EqualsNameAndTypes(*o.Inputs, cs.InputMap) {
		return false
	}
	if o.Output != nil && !EqualsTypeSpec(*o.Output, cs.Output) {
		return false
	}
	if o.Payable != nil && *o.Payable != cs.Payable {
		return false
	}
	if o.Readonly != nil && *o.Readonly != cs.ReadOnly {
		return false
	}
	return true
}

func FlagFromMethodOverload(o MethodOverload) MethodOverloadFlag {
	f := MethodOverloadNone
	if o.Inputs != nil {
		f |= MethodOverloadInputs
	}
	if o.Output != nil {
		f |= MethodOverloadOutput
	}
	if o.Payable != nil {
		f |= MethodOverloadPayable
	}
	if o.Readonly != nil {
		f |= MethodOverloadReadonly
	}
	return f
}

const (
	EventOverloadNone   EventOverloadFlag = 0
	EventOverloadInputs EventOverloadFlag = 1 << iota
	EventOverloadIndexed
)

type EventOverloadFlag int

func (f EventOverloadFlag) None() bool {
	return f == EventOverloadNone
}
func (f EventOverloadFlag) Inputs() bool {
	return f&EventOverloadInputs > 0
}

func (f EventOverloadFlag) Indexed() bool {
	return f&EventOverloadIndexed > 0
}

func CompareEventSpec(ss *EventSpec, cs *contract.EventSpec) EventOverloadFlag {
	f := EventOverloadNone
	if !EqualsNameAndTypes(ss.Inputs, cs.InputMap) {
		f |= EventOverloadInputs
	}
	if !EqualsEventIndexed(ss.Indexed, NewEventSpecIndexed(cs)) {
		f |= EventOverloadIndexed
	}
	return f
}

func CompareEventOverload(o *EventOverload, cs *contract.EventSpec) bool {
	if o.Inputs != nil && !EqualsNameAndTypes(*o.Inputs, cs.InputMap) {
		return false
	}
	if o.Indexed != nil && !EqualsEventIndexed(*o.Indexed, NewEventSpecIndexed(cs)) {
		return false
	}
	return true
}

func FlagFromEventOverload(o EventOverload) EventOverloadFlag {
	f := EventOverloadNone
	if o.Inputs != nil {
		f |= EventOverloadInputs
	}
	if o.Indexed != nil {
		f |= EventOverloadIndexed
	}
	return f
}

func StringSetIndex(l []string, v string) int {
	for i, elem := range l {
		if elem == v {
			return i
		}
	}
	return -1
}

func StringSetContains(l []string, v string) bool {
	return StringSetIndex(l, v) >= 0
}

func StringSetAdd(l []string, v string) ([]string, bool) {
	i := StringSetIndex(l, v)
	if i < 0 {
		return append(l, v), true
	}
	return l, false
}

func StringSetRemove(l []string, v string) ([]string, bool) {
	i := StringSetIndex(l, v)
	if i >= 0 {
		return append(l[:i], l[i+1:]...), true
	}
	return l, false
}

func StringSetMerge(a []string, b []string) []string {
	r := a[:]
	for _, bv := range b {
		if !StringSetContains(a, bv) {
			r = append(r, bv)
		}
	}
	return r
}

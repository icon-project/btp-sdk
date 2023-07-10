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
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"math/big"
	"reflect"
	"strings"

	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/intconv"
	"github.com/icon-project/btp2/common/log"
)

func MustParamOf(value interface{}) interface{} {
	ret, err := ParamOf(value)
	if err != nil {
		log.Panicf("fail to ParamOf err:%v", err)
	}
	return ret
}

func ParamOf(value interface{}) (interface{}, error) {
	var err error
	switch v := value.(type) {
	case Params:
		return ParamsOf(v)
	case Struct:
		return StructOf(v)
	case Address:
		return v, nil
	case Integer, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, big.Int, *big.Int:
		return IntegerOf(v)
	case Boolean, bool:
		return BooleanOf(v)
	case String, string:
		return StringOf(v)
	case Bytes, []byte:
		return BytesOf(v)
	default:
		rv := reflect.ValueOf(value)
		rt := rv.Type()
		var p interface{}
		switch rv.Kind() {
		case reflect.Array, reflect.Slice:
			convert := false
			var ret reflect.Value
			for i := 0; i < rv.Len(); i++ {
				if p, err = ParamOf(rv.Index(i).Interface()); err != nil {
					return nil, err
				}
				pv := reflect.ValueOf(p)
				if !convert && rt.Elem() != pv.Type() {
					ret = reflect.MakeSlice(reflect.SliceOf(pv.Type()), 0, rv.Len())
					convert = true
				}
				ret = reflect.Append(ret, pv)
			}
			if convert {
				return ret.Interface(), nil
			}
			return rv, nil
		case reflect.Struct:
			return StructOf(v)
		case reflect.Map:
			return ParamsOf(v)
		default:
			return nil, ErrorCodeInvalidParam.Errorf("not supported type %T", v)
		}
	}
}
func ParamOfWithSpec(s TypeSpec, value interface{}) (interface{}, error) {
	if s.Dimension > 0 {
		return arrayParamOf(s, 1, reflect.ValueOf(value))
	} else {
		switch s.TypeID {
		case TStruct:
			return StructOfWithSpec(*s.Resolved, value)
		default:
			return primitiveParamOf(s.TypeID, value)
		}
	}
}

func primitiveParamOf(s TypeTag, value interface{}) (interface{}, error) {
	if v, ok := value.(reflect.Value); ok {
		value = v.Interface()
	}
	switch s {
	case TInteger:
		return IntegerOf(value)
	case TString:
		return StringOf(value)
	case TAddress:
		return AddressOf(value)
	case TBytes:
		return BytesOf(value)
	case TBoolean:
		return BooleanOf(value)
	default:
		return nil, ErrorCodeInvalidParam.Errorf("fail primitiveParamOf, not supported %+v", s)
	}
}

func arrayParamOf(s TypeSpec, dimension int, v reflect.Value) ([]interface{}, error) {
	if v.Type().Kind() != reflect.Array && v.Type().Kind() != reflect.Slice {
		return nil, ErrorCodeInvalidParam.Errorf("fail to arrayParamOf, invalid type %v", v.Type().Kind())
	}
	var err error
	l := make([]interface{}, v.Len())
	for i := 0; i < v.Len(); i++ {
		element := v.Index(i)
		if s.Dimension > dimension {
			l[i], err = arrayParamOf(s, dimension+1, element)
		} else {
			if s.TypeID == TStruct {
				l[i], err = StructOfWithSpec(*s.Resolved, element.Interface())
			} else {
				l[i], err = primitiveParamOf(s.TypeID, element.Interface())
			}
		}
		if err != nil {
			return nil, err
		}
	}
	return l, nil
}

func MustIntegerOf(value interface{}) Integer {
	ret, err := IntegerOf(value)
	if err != nil {
		log.Panicf("fail to IntegerOf err:%v", err)
	}
	return ret
}

const (
	invalidInteger = ""
)

func integerOf(s string) (Integer, error) {
	i := Integer(s)
	if _, err := i.AsBigInt(); err != nil {
		return invalidInteger, ErrorCodeInvalidParam.Wrapf(err, "invalid param err:%s", err.Error())
	}
	return i, nil
}

func IntegerOf(value interface{}) (Integer, error) {
	switch v := value.(type) {
	case Integer:
		return v, nil
	case String:
		return integerOf(string(v))
	case string:
		return integerOf(v)
	case []byte:
		return Integer(intconv.FormatBigInt(intconv.BigIntSetBytes(new(big.Int), v))), nil
	case big.Int:
		return Integer(intconv.FormatBigInt(&v)), nil
	case *big.Int:
		return Integer(intconv.FormatBigInt(v)), nil
	default:
		rv := reflect.ValueOf(value)
		if rv.CanInt() {
			return Integer(intconv.FormatBigInt(big.NewInt(rv.Int()))), nil
		} else if rv.CanUint() {
			return Integer(intconv.FormatBigInt(new(big.Int).SetUint64(rv.Uint()))), nil
		} else {
			return invalidInteger, ErrorCodeInvalidParam.Errorf("invalid type %T", value)
		}
	}
}

func MustBooleanOf(value interface{}) Boolean {
	ret, err := BooleanOf(value)
	if err != nil {
		log.Panicf("fail to BooleanOf err:%v", err)
	}
	return ret
}

func BooleanOf(value interface{}) (Boolean, error) {
	switch v := value.(type) {
	case Boolean:
		return v, nil
	case bool:
		return Boolean(v), nil
	default:
		return false, ErrorCodeInvalidParam.Errorf("invalid type %T", v)
	}
}

func MustStringOf(value interface{}) String {
	ret, err := StringOf(value)
	if err != nil {
		log.Panicf("fail to StringOf err:%v", err)
	}
	return ret
}

func StringOf(value interface{}) (String, error) {
	switch v := value.(type) {
	case String:
		return v, nil
	case string:
		return String(v), nil
	default:
		return "", ErrorCodeInvalidParam.Errorf("invalid type %T", v)
	}
}

func MustBytesOf(value interface{}) Bytes {
	ret, err := BytesOf(value)
	if err != nil {
		log.Panicf("fail to BytesOf err:%v", err)
	}
	return ret
}

func bytesOf(s string) (Bytes, error) {
	parseFunc := base64.StdEncoding.DecodeString
	if strings.HasPrefix(s, "0x") {
		parseFunc = hex.DecodeString
		s = s[2:]
	}
	b, err := parseFunc(s)
	if err != nil {
		return nil, ErrorCodeInvalidParam.Wrapf(err, "invalid param err:%s", err.Error())
	}
	return b, nil
}

func BytesOf(value interface{}) (Bytes, error) {
	switch v := value.(type) {
	case Bytes:
		return v, nil
	case []byte:
		return v, nil
	case String:
		return bytesOf(string(v))
	case string:
		return bytesOf(v)
	default:
		return nil, ErrorCodeInvalidParam.Errorf("invalid type %T", v)
	}
}

func MustStructOf(value interface{}) Struct {
	ret, err := StructOf(value)
	if err != nil {
		log.Panicf("fail to StructOf err:%v", err)
	}
	return ret
}

func structOf(value interface{}) (Struct, error) {
	st, ok := value.(Struct)
	if !ok {
		rv := reflect.ValueOf(value)
		rt := rv.Type()
		if rt.Kind() != reflect.Struct {
			return st, ErrorCodeInvalidParam.Errorf("invalid type:%T", value)
		}
		st = Struct{
			Name:   rt.Name(),
			Fields: make([]KeyValue, rv.NumField()),
		}
		for i := 0; i < rv.NumField(); i++ {
			name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ",")
			if name == "" {
				name = rt.Field(i).Name
			}
			st.Fields[i] = KeyValue{Key: name, Value: rv.Field(i).Interface()}
		}
	}
	return st, nil
}

func StructOf(value interface{}) (Struct, error) {
	st, err := structOf(value)
	if err != nil {
		return st, err
	}
	for _, f := range st.Fields {
		p, err := ParamOf(f.Value)
		if err != nil {
			return st, err
		}
		if reflect.TypeOf(p) != reflect.TypeOf(f.Value) {
			f.Value = p
		}
	}
	return st, nil
}

func StructOfWithSpec(spec StructSpec, value interface{}) (interface{}, error) {
	var params Params
	st, err := structOf(value)
	if err != nil {
		var pErr error
		if params, pErr = paramsOf(value); pErr != nil {
			return nil, ErrorCodeInvalidParam.Wrapf(pErr, "fail StructOfWithSpec, err:%s pErr:%s", err.Error(), pErr.Error())
		}
	} else {
		params = st.Params()
	}
	for k, v := range params {
		s, ok := spec.FieldMap[k]
		if !ok {
			return nil, ErrorCodeInvalidParam.Errorf("unknown field name:%s", k)
		}
		param, err := ParamOfWithSpec(s.Type, v)
		if err != nil {
			return nil, err
		}
		params[k] = param
	}

	ret := Struct{
		Name:   spec.Name,
		Fields: make([]KeyValue, len(spec.Fields)),
	}
	for i, f := range spec.Fields {
		ret.Fields[i] = KeyValue{Key: f.Name, Value: params[f.Name]}
	}
	return ret, nil
}

func MustAddressOf(value interface{}) Address {
	ret, err := AddressOf(value)
	if err != nil {
		log.Panicf("fail to AddressOf err:%v", err)
	}
	return ret
}

func AddressOf(value interface{}) (Address, error) {
	switch v := value.(type) {
	case Address:
		return v, nil
	case String:
		return Address(v), nil
	case string:
		return Address(v), nil
	default:
		return "", ErrorCodeInvalidParam.Errorf("invalid type %T", v)
	}
}

func MustParamsOf(value interface{}) Params {
	ret, err := ParamsOf(value)
	if err != nil {
		log.Panicf("fail to ParamsOf err:%v", err)
	}
	return ret
}

func paramsOf(value interface{}) (Params, error) {
	params, ok := value.(Params)
	if !ok {
		rv := reflect.ValueOf(value)
		rt := rv.Type()
		if rt.Kind() != reflect.Map {
			return nil, ErrorCodeInvalidParam.Errorf("invalid type:%T", value)
		}
		if rt.Key().Kind() != reflect.String {
			return nil, ErrorCodeInvalidParam.Errorf("not supported key type %v", rt.Key())
		}
		params = make(Params)
		for _, k := range rv.MapKeys() {
			params[k.String()] = rv.MapIndex(k).Interface()
		}
	}
	return params, nil
}

func ParamsOf(value interface{}) (Params, error) {
	params, err := paramsOf(value)
	if err != nil {
		return nil, err
	}
	for k, v := range params {
		param, err := ParamOf(v)
		if err != nil {
			return nil, err
		}
		params[k] = param
	}
	return params, nil
}

func ParamsOfWithSpec(spec map[string]*NameAndTypeSpec, value interface{}) (Params, error) {
	params, err := paramsOf(value)
	if err != nil {
		return nil, err
	}
	for k, v := range params {
		s, ok := spec[k]
		if !ok {
			return nil, ErrorCodeInvalidParam.Errorf("unknown param name:%s", k)
		}
		param, err := ParamOfWithSpec(s.Type, v)
		if err != nil {
			return nil, err
		}
		params[k] = param
	}
	return params, nil
}

func EqualParam(s TypeSpec, a, b interface{}) (equals bool, err error) {
	if s.Dimension == 0 {
		switch s.TypeID {
		case TInteger:
			var ca, cb Integer
			if ca, err = IntegerOf(a); err != nil {
				return false, err
			}
			if cb, err = IntegerOf(b); err != nil {
				return false, err
			}
			return ca == cb, nil
		case TBoolean:
			var ca, cb Boolean
			if ca, err = BooleanOf(a); err != nil {
				return false, err
			}
			if cb, err = BooleanOf(b); err != nil {
				return false, err
			}
			return ca == cb, nil
		case TString:
			var ca, cb String
			if ca, err = StringOf(a); err != nil {
				return false, err
			}
			if cb, err = StringOf(b); err != nil {
				return false, err
			}
			return ca == cb, nil
		case TBytes:
			var ca, cb Bytes
			if ca, err = BytesOf(a); err != nil {
				return false, err
			}
			if cb, err = BytesOf(b); err != nil {
				return false, err
			}
			return bytes.Equal(ca, cb), nil
		case TStruct:
			var ca, cb Struct
			if ca, err = StructOf(a); err != nil {
				return false, err
			}
			if cb, err = StructOf(b); err != nil {
				return false, err
			}
			pa, pb := ca.Params(), cb.Params()
			for n, f := range s.Resolved.FieldMap {
				if equals, err = EqualParam(f.Type, pa[n], pb[n]); err != nil {
					return false, err
				} else if !equals {
					return false, nil
				}
			}
			return true, nil
		case TAddress:
			var ca, cb Address
			if ca, err = AddressOf(a); err != nil {
				return false, err
			}
			if cb, err = AddressOf(b); err != nil {
				return false, err
			}
			return ca == cb, nil
		default:
			return false, errors.Errorf("not comparable spec %v", s)
		}
	} else {
		av, bv := reflect.ValueOf(a), reflect.ValueOf(b)
		if av.Kind() != reflect.Array && av.Kind() != reflect.Slice &&
			bv.Kind() != reflect.Array && bv.Kind() != reflect.Slice {
			return false, errors.Errorf("invalid type a:%T b:%T", a, b)
		}
		if av.Len() != bv.Len() {
			return false, nil
		}
		es := TypeSpec{
			Name:      s.Name,
			Dimension: s.Dimension - 1,
			Type:      s.Type,
			TypeID:    s.TypeID,
			Resolved:  s.Resolved,
		}
		for i := 0; i < av.Len(); i++ {
			ae := av.Index(i).Interface()
			be := bv.Index(i).Interface()
			equals, err = EqualParam(es, ae, be)
			if err != nil {
				return false, err
			}
			if !equals {
				return false, nil
			}
		}
		return true, nil
	}
}

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
			return nil, errors.Errorf("not supported type %T", v)
		}
	}
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

func IntegerOf(value interface{}) (Integer, error) {
	switch v := value.(type) {
	case Integer:
		return v, nil
	case string:
		i := Integer(v)
		if _, err := i.AsBigInt(); err != nil {
			return invalidInteger, err
		}
		return i, nil
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
			return invalidInteger, errors.Errorf("invalid type %T", value)
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
		return false, errors.Errorf("invalid type %T", v)
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
		return "", errors.Errorf("invalid type %T", v)
	}
}

func MustBytesOf(value interface{}) Bytes {
	ret, err := BytesOf(value)
	if err != nil {
		log.Panicf("fail to BytesOf err:%v", err)
	}
	return ret
}

func BytesOf(value interface{}) (Bytes, error) {
	switch v := value.(type) {
	case Bytes:
		return v, nil
	case []byte:
		return v, nil
	default:
		return nil, errors.Errorf("invalid type %T", v)
	}
}

func MustStructOf(value interface{}) Struct {
	ret, err := StructOf(value)
	if err != nil {
		log.Panicf("fail to StructOf err:%v", err)
	}
	return ret
}

func StructOf(value interface{}) (Struct, error) {
	if v, ok := value.(Struct); ok {
		for _, f := range v.Fields {
			p, err := ParamOf(f.Value)
			if err != nil {
				return v, err
			}
			if reflect.TypeOf(p) != reflect.TypeOf(f.Value) {
				f.Value = p
			}
		}
		return v, nil
	} else {
		rv := reflect.ValueOf(value)
		rt := rv.Type()
		if rt.Kind() != reflect.Struct {
			return v, errors.Errorf("invalid type:%T", value)
		}
		ret := Struct{
			Name:   rt.Name(),
			Fields: make([]KeyValue, rv.NumField()),
		}
		for i := 0; i < rv.NumField(); i++ {
			name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ",")
			if name == "" {
				name = rt.Field(i).Name
			}
			p, err := ParamOf(rv.Field(i).Interface())
			if err != nil {
				return v, err
			}
			ret.Fields[i] = KeyValue{Key: name, Value: p}
		}
		return ret, nil
	}
}

func MustParamsOf(value interface{}) Params {
	ret, err := ParamsOf(value)
	if err != nil {
		log.Panicf("fail to ParamsOf err:%v", err)
	}
	return ret
}
func ParamsOf(value interface{}) (Params, error) {
	if v, ok := value.(Params); ok {
		var err error
		for k, p := range v {
			if v[k], err = ParamOf(p); err != nil {
				return nil, err
			}
		}
		return v, nil
	} else {
		rv := reflect.ValueOf(value)
		rt := rv.Type()
		if rt.Kind() != reflect.Map {
			return nil, errors.Errorf("invalid type:%T", value)
		}
		if rt.Key().Kind() != reflect.String {
			return nil, errors.Errorf("not supported key type %v", rt.Key())
		}
		ret := make(Params)
		for _, k := range rv.MapKeys() {
			p, err := ParamOf(rv.MapIndex(k).Interface())
			if err != nil {
				return nil, err
			}
			ret[k.String()] = p
		}
		return ret, nil
	}
}

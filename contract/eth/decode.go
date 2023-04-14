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
	"math/big"
	"reflect"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/icon-project/btp2/common/errors"

	"github.com/icon-project/btp-sdk/contract"
)

func t_decode(s abi.Type, value interface{}) (interface{}, error) {
	if value == nil {
		return nil, nil
	}
	codecLogger.Traceln("decode spec:", s.TupleRawName, "type:", s.String(), "reflect:", s.GetType(), reflect.TypeOf(value))
	switch s.T {
	case abi.TupleTy:
		return t_decodeStruct(s, value)
	case abi.ArrayTy, abi.SliceTy:
		return t_decodeArray(s, reflect.ValueOf(value))
	default:
		return t_decodePrimitive(s, value)
	}
}

func t_decodePrimitive(s abi.Type, value interface{}) (interface{}, error) {
	codecLogger.Traceln("decodePrimitive type:", s.String(), "reflect:", s.GetType())
	v, ok := value.(reflect.Value)
	if !ok {
		v = reflect.ValueOf(value)
	}
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}
	if s.GetType() != v.Type() {
		return nil, errors.Errorf("fail decodePrimitive, invalid type expected:%v actual:%v",
			s.GetType(), v.Type())
	}
	switch s.T {
	case abi.IntTy, abi.UintTy:
		switch s.Size {
		case 8, 16, 32, 64:
			if s.T == abi.IntTy {
				return contract.FromInt64(v.Int()), nil
			} else {
				return contract.FromUint64(v.Uint()), nil
			}
		default:
			i := v.Interface().(*big.Int)
			return contract.FromBigInt(i), nil
		}
	case abi.StringTy:
		return contract.String(v.String()), nil
	case abi.AddressTy:
		return contract.Address(v.Interface().(common.Address).String()), nil
	case abi.BytesTy:
		return contract.Bytes(v.Bytes()), nil
	case abi.BoolTy:
		return contract.Boolean(v.Bool()), nil
	default: //abi.FixedBytesTy
		return nil, errors.New("fail decodePrimitive, not supported")
	}
}

func t_decodeStruct(s abi.Type, value interface{}) (interface{}, error) {
	codecLogger.Traceln("decodeStruct spec:", s.TupleRawName)
	ret := make(map[string]interface{})
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Struct {
		return nil, errors.Errorf("fail decodeStruct, invalid type %T", value)
	}
	var err error
	for i, element := range s.TupleElems {
		k := s.TupleRawNames[i]
		if ret[k], err = t_decode(*element, v.Field(i)); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func t_decodeArray(s abi.Type, v reflect.Value) (interface{}, error) {
	codecLogger.Traceln("decodeArray spec:", s.TupleRawName, "type:", s.String(), "reflect:", s.GetType())
	if v.Type().Kind() == reflect.Interface {
		v = v.Elem()
	}
	if v.Type().Kind() != reflect.Array && v.Type().Kind() != reflect.Slice {
		return nil, errors.Errorf("fail decodeArray, invalid type %v", v.Type().Kind())
	}

	ret := reflect.MakeSlice(s.GetType(), 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		re, err := t_decode(*s.Elem, v.Index(i).Interface())
		if err != nil {
			return nil, err
		}
		ret = reflect.Append(ret, reflect.ValueOf(re))
	}
	return ret.Interface(), nil
}

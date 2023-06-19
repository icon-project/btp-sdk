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
	"reflect"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
)

var (
	codecLogger = log.New()
)

func init() {
	codecLogger.SetLevel(log.DebugLevel)
}

func encode(s abi.Type, value interface{}) (interface{}, error) {
	switch s.T {
	case abi.TupleTy:
		return encodeStruct(s, value)
	case abi.ArrayTy, abi.SliceTy:
		return encodeArray(s, reflect.ValueOf(value))
	default:
		return encodePrimitive(s, value)
	}
}

func encodePrimitive(s abi.Type, value interface{}) (interface{}, error) {
	if v, ok := value.(reflect.Value); ok {
		value = v.Interface()
	}
	switch s.T {
	case abi.IntTy, abi.UintTy:
		v, err := contract.IntegerOf(value)
		if err != nil {
			return nil, errors.Wrapf(err, "fail encodePrimitive integer, err:%s", err.Error())
		}
		bi, err := v.AsBigInt()
		if err != nil {
			return nil, errors.Wrapf(err, "fail encodePrimitive integer, err:%s", err.Error())
		}
		bl := s.Size
		if s.T == abi.IntTy {
			bl = bl + 1
		} else {
			if bi.Sign() < 0 {
				return nil, errors.Errorf("fail encodePrimitive integer, invalid sign")
			}
		}
		if bl < bi.BitLen() {
			return nil, errors.Errorf("fail encodePrimitive integer, invalid bit length expected:%d actual:%d",
				bl, bi.BitLen())
		}
		switch s.Size {
		case 8, 16, 32, 64:
			if s.T == abi.IntTy {
				i := bi.Int64()
				switch s.Size {
				case 8:
					return int8(i), nil
				case 16:
					return int16(i), nil
				case 32:
					return int32(i), nil
				}
				return i, nil
			} else {
				i := bi.Uint64()
				switch s.Size {
				case 8:
					return uint8(i), nil
				case 16:
					return uint16(i), nil
				case 32:
					return uint32(i), nil
				}
				return i, nil
			}
		default:
			return bi, nil
		}
	case abi.StringTy:
		v, err := contract.StringOf(value)
		if err != nil {
			return nil, errors.Wrapf(err, "fail encodePrimitive string, err:%s", err.Error())
		}
		return string(v), nil
	case abi.AddressTy:
		v, err := contract.AddressOf(value)
		if err != nil {
			return nil, errors.Wrapf(err, "fail encodePrimitive address, err:%s", err.Error())
		}
		if !common.IsHexAddress(string(v)) {
			return nil, errors.Errorf("fail encodePrimitive address, required hex")
		}
		return common.HexToAddress(string(v)), nil
	case abi.BytesTy:
		v, err := contract.BytesOf(value)
		if err != nil {
			return nil, errors.Wrapf(err, "fail encodePrimitive bytes, err:%s", err.Error())
		}
		return []byte(v), nil
	case abi.BoolTy:
		v, err := contract.BooleanOf(value)
		if err != nil {
			return nil, errors.Wrapf(err, "fail encodePrimitive boolean, err:%s", err.Error())
		}
		return bool(v), nil
	default: //abi.FixedBytesTy
		return nil, errors.Errorf("fail encodePrimitive, not supported %v", s)
	}
}

func encodeStruct(s abi.Type, value interface{}) (interface{}, error) {
	var params contract.Params
	st, err := contract.StructOf(value)
	if err != nil {
		var pErr error
		if params, pErr = contract.ParamsOf(value); err != nil {
			return nil, errors.Wrapf(err, "fail encodeStruct, err:%s pErr:%s", err.Error(), pErr.Error())
		}
	} else {
		params = st.Params()
	}
	ret := reflect.New(s.TupleType).Elem()
	for i, n := range s.TupleRawNames {
		field, err := encode(*s.TupleElems[i], params[n])
		if err != nil {
			return nil, err
		}
		v := reflect.ValueOf(field)
		ret.Field(i).Set(v)
	}
	return ret.Interface(), nil
}

func encodeArray(s abi.Type, v reflect.Value) (interface{}, error) {
	if v.Type().Kind() != reflect.Array && v.Type().Kind() != reflect.Slice {
		return nil, errors.Errorf("fail encodeArray, invalid type %v", v.Type().Kind())
	}
	ret := reflect.MakeSlice(s.GetType(), 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		re, err := encode(*s.Elem, v.Index(i).Interface())
		if err != nil {
			return nil, err
		}
		ret = reflect.Append(ret, reflect.ValueOf(re))
	}
	return ret.Interface(), nil
}

func decode(s contract.TypeSpec, value interface{}) (interface{}, error) {
	if value == nil {
		return nil, nil
	}
	codecLogger.Traceln("decode spec:", s.Name, "type:", s.TypeID.String(), "reflect:", s.Type, value)
	if s.Dimension > 0 {
		return decodeArray(s, 1, reflect.ValueOf(value))
	} else {
		switch s.TypeID {
		case contract.TVoid:
			return nil, nil
		case contract.TStruct:
			return decodeStruct(s.Resolved, value)
		default:
			return decodePrimitive(s.TypeID, value)
		}
	}
}

func decodePrimitive(s contract.TypeTag, value interface{}) (interface{}, error) {
	codecLogger.Traceln("decodePrimitive type:", s.String(), "reflect:", s.Type(), value)
	switch s {
	case contract.TInteger:
		return contract.IntegerOf(value)
	case contract.TString:
		v, ok := value.(string)
		if !ok {
			return nil, errors.Errorf("fail decodePrimitive string, invalid type %T", value)
		}
		return contract.String(v), nil
	case contract.TAddress:
		v, ok := value.(common.Address)
		if !ok {
			return nil, errors.Errorf("fail decodePrimitive address, invalid type %T", value)
		}
		return contract.Address(v.String()), nil
	case contract.TBytes:
		v, ok := value.([]byte)
		if !ok {
			return nil, errors.Errorf("fail decodePrimitive bytes, invalid type %T", value)
		}
		return contract.Bytes(v), nil
	case contract.TBoolean:
		v, ok := value.(bool)
		if !ok {
			return nil, errors.Errorf("fail decodePrimitive bool, invalid type %T", value)
		}
		return contract.Boolean(v), nil
	default:
		return nil, errors.New("fail decodePrimitive, not supported")
	}
}

func decodeStruct(s *contract.StructSpec, value interface{}) (interface{}, error) {
	codecLogger.Traceln("decodeStruct spec:", s.Name, value)
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Struct {
		return nil, errors.Errorf("fail decodeStruct, invalid type %T", value)
	}
	fields := make([]contract.KeyValue, len(s.Fields))
	for i, f := range s.Fields {
		fv, err := decode(f.Type, v.Field(i).Interface())
		if err != nil {
			return nil, err
		}
		fields[i] = contract.KeyValue{Key: f.Name, Value: fv}
	}
	return contract.Struct{
		Name:   s.Name,
		Fields: fields,
	}, nil
}

func decodeArray(s contract.TypeSpec, dimension int, v reflect.Value) (interface{}, error) {
	codecLogger.Traceln("decodeArray spec:", s.Name, "type:", s.TypeID.String(), "reflect:", s.Type, v)
	if v.Type().Kind() == reflect.Interface {
		v = v.Elem()
	}
	if v.Type().Kind() != reflect.Array && v.Type().Kind() != reflect.Slice {
		return nil, errors.Errorf("fail decodeArray, invalid type %v", v.Type().Kind())
	}
	var err error
	ret := reflect.MakeSlice(s.Type, 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		element := v.Index(i)
		var re interface{}
		if s.Dimension > dimension {
			re, err = decodeArray(s, dimension+1, element)
		} else {
			if s.TypeID == contract.TStruct {
				re, err = decodeStruct(s.Resolved, element.Interface())
			} else {
				re, err = decodePrimitive(s.TypeID, element.Interface())
			}
		}
		if err != nil {
			return nil, err
		}
		ret = reflect.Append(ret, reflect.ValueOf(re))
	}
	return ret.Interface(), nil
}

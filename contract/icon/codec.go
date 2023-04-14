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

package icon

import (
	"reflect"

	"github.com/icon-project/btp2/chain/icon/client"
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

func encode(s contract.TypeSpec, value interface{}) (interface{}, error) {
	if s.Dimension > 0 {
		return encodeArray(s, 1, reflect.ValueOf(value))
	}
	if s.TypeID == contract.TStruct {
		return encodeStruct(s.Resolved, value)
	} else {
		return encodePrimitive(s.TypeID, value)
	}
}

func encodePrimitive(s contract.TypeTag, value interface{}) (interface{}, error) {
	if v, ok := value.(reflect.Value); ok {
		value = v.Interface()
	}
	switch s {
	case contract.TInteger:
		if _, ok := value.(contract.Integer); !ok {
			return nil, errors.Errorf("fail encodePrimitive integer, invalid type %T", value)
		}
		return value, nil
	case contract.TString:
		if _, ok := value.(contract.String); !ok {
			return nil, errors.Errorf("fail encodePrimitive string, invalid type %T", value)
		}
		return value, nil
	case contract.TAddress:
		if _, ok := value.(contract.Address); !ok {
			return nil, errors.Errorf("fail encodePrimitive address, invalid type %T", value)
		}
		return value, nil
	case contract.TBytes:
		b, ok := value.(contract.Bytes)
		if !ok {
			return nil, errors.Errorf("fail encodePrimitive []byte, invalid type %T", value)
		}
		return client.NewHexBytes(b), nil
	case contract.TBoolean:
		b, ok := value.(contract.Boolean)
		if !ok {
			return nil, errors.Errorf("fail encodePrimitive bool, invalid type %T", value)
		}
		return NewHexBool(bool(b)), nil
	default:
		return nil, errors.New("fail encodePrimitive, not supported")
	}
}

func encodeStruct(s *contract.StructSpec, value interface{}) (interface{}, error) {
	m, ok := value.(map[string]interface{})
	if !ok {
		return nil, errors.Errorf("fail encodeStruct, invalid type %T", value)
	}
	ret := make(map[string]interface{})
	var err error
	for k, v := range s.FieldMap {
		if ret[k], err = encode(v.Type, m[k]); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func encodeArray(s contract.TypeSpec, dimension int, v reflect.Value) ([]interface{}, error) {
	if v.Type().Kind() != reflect.Array && v.Type().Kind() != reflect.Slice {
		return nil, errors.Errorf("fail encodeArray, invalid type %v", v.Type().Kind())
	}
	var err error
	l := make([]interface{}, v.Len())
	for i := 0; i < v.Len(); i++ {
		element := v.Index(i)
		if s.Dimension > dimension {
			l[i], err = encodeArray(s, dimension+1, element)
		} else {
			if s.TypeID == contract.TStruct {
				l[i], err = encodeStruct(s.Resolved, element)
			} else {
				l[i], err = encodePrimitive(s.TypeID, element)
			}
		}
		if err != nil {
			return nil, err
		}
	}
	return l, nil
}

func decode(s contract.TypeSpec, value interface{}) (interface{}, error) {
	if value == nil {
		return nil, nil
	}
	codecLogger.Traceln("decode spec:", s.Name, "type:", s.TypeID.String(), "reflect:", s.Type)
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
	codecLogger.Traceln("decodePrimitive type:", s.String(), "reflect:", s.Type())
	str, ok := value.(string)
	if !ok {
		return nil, errors.Errorf("fail decodePrimitive, invalid type expected:%v actual:%v",
			"string", value)
	}
	switch s {
	case contract.TInteger:
		return contract.Integer(str), nil
	case contract.TString:
		return contract.String(str), nil
	case contract.TAddress:
		return contract.Address(str), nil
	case contract.TBytes:
		v, err := client.HexBytes(str).Value()
		if err != nil {
			return nil, errors.Wrapf(err, "fail decodePrimitive to bytes err:%s", err.Error())
		}
		return contract.Bytes(v), nil
	case contract.TBoolean:
		v, err := HexBool(str).Value()
		if err != nil {
			return nil, errors.Wrapf(err, "fail decodePrimitive to bool err:%s", err.Error())
		}
		return contract.Boolean(v), nil
	default:
		return nil, errors.New("fail decodePrimitive, not supported")
	}
}

func decodeStruct(s *contract.StructSpec, value interface{}) (interface{}, error) {
	codecLogger.Traceln("decodeStruct spec:", s.Name)
	ret := make(map[string]interface{})
	m, ok := value.(map[string]interface{})
	if !ok {
		return nil, errors.Errorf("fail decodeStruct, invalid type %T", value)
	}
	var err error
	for k, v := range s.FieldMap {
		if ret[k], err = decode(v.Type, m[k]); err != nil {
			return nil, err
		}
	}
	return ret, nil
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

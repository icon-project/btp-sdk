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
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/types"
	"github.com/stretchr/testify/assert"

	"github.com/icon-project/btp-sdk/contract"
)

const (
	specFile = "../../example/solidity/artifacts/contracts/HelloWorld.sol/HelloWorld.abi.json"
)

var (
	byteVal       = contract.Integer("0x7f")
	shortVal      = contract.Integer("0x7fff")
	intVal        = contract.Integer("0x7fffffff")
	longVal       = contract.Integer("0x7fffffffffffffff")
	bigIntegerVal = contract.Integer("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
	int24Val      = contract.Integer("0x7fffff")
	int40Val      = contract.Integer("0x7fffffffff")
	int72Val      = contract.Integer("0x7fffffffffffffffff")
	charVal       = contract.Integer("0xffff")
	booleanVal    = contract.Boolean(true)
	stringVal     = contract.String("string")
	bytesVal      = contract.Bytes("bytes")
	addressVal    = contract.Address(w.Address())
	structVal     = struct {
		BooleanVal contract.Boolean `json:"booleanVal"`
	}{booleanVal}
)

func handler(t *testing.T, networkType string) (*Handler, *Adaptor) {
	b, err := os.ReadFile(specFile)
	if err != nil {
		assert.FailNow(t, "fail to read file", err)
	}
	a := adaptor(t, networkType)
	h, err := a.Handler(b, addr)
	if err != nil {
		assert.FailNow(t, "fail to Handler", err)
	}
	return h.(*Handler), a
}

func Test_callInteger(t *testing.T) {
	h, _ := handler(t, NetworkTypeEth2)
	method := "callInteger"
	params := make(contract.Params)
	params["arg1"] = byteVal
	params["arg2"] = shortVal
	params["arg3"] = intVal
	params["arg4"] = longVal
	params["arg5"] = int24Val
	params["arg6"] = int40Val
	params["arg7"] = int72Val
	params["arg8"] = bigIntegerVal

	options := make(contract.Options)
	r, err := h.Call(method, params, options)
	assert.NoError(t, err)
	t.Log(r)

	assertStruct(t, params, r)
}

func Test_callUnsignedInteger(t *testing.T) {
	h, _ := handler(t, NetworkTypeEth2)
	method := "callUnsignedInteger"
	params := make(contract.Params)
	params["arg1"] = contract.Integer("0x" + strings.Repeat("ff", 1))
	params["arg2"] = charVal
	params["arg3"] = contract.Integer("0x" + strings.Repeat("ff", 4))
	params["arg4"] = contract.Integer("0x" + strings.Repeat("ff", 8))
	params["arg5"] = contract.Integer("0x" + strings.Repeat("ff", 3))
	params["arg6"] = contract.Integer("0x" + strings.Repeat("ff", 5))
	params["arg7"] = contract.Integer("0x" + strings.Repeat("ff", 9))
	params["arg8"] = contract.Integer("0x" + strings.Repeat("ff", 32))

	options := make(contract.Options)
	r, err := h.Call(method, params, options)
	assert.NoError(t, err)
	t.Log(r)

	assertStruct(t, params, r)
}

func Test_callPrimitive(t *testing.T) {
	h, _ := handler(t, NetworkTypeEth2)
	method := "callPrimitive"
	params := make(contract.Params)
	params["arg1"] = bigIntegerVal
	params["arg2"] = booleanVal
	params["arg3"] = stringVal
	params["arg4"] = bytesVal
	params["arg5"] = addressVal

	options := make(contract.Options)
	r, err := h.Call(method, params, options)
	assert.NoError(t, err)
	t.Log(r)

	assertStruct(t, params, r)
}

func Test_callStruct(t *testing.T) {
	h, _ := handler(t, NetworkTypeEth2)
	method := "callStruct"
	params := make(contract.Params)
	params["arg1"] = contract.MustStructOf(structVal)
	options := make(contract.Options)
	r, err := h.Call(method, params, options)
	assert.NoError(t, err)
	t.Log(r)

	assertStruct(t, params["arg1"], r)
}

func Test_callArray(t *testing.T) {
	h, _ := handler(t, NetworkTypeEth2)
	method := "callArray"
	params := make(contract.Params)
	params["arg1"] = []contract.Integer{bigIntegerVal}
	params["arg2"] = []contract.Boolean{booleanVal}
	params["arg3"] = []contract.String{stringVal}
	params["arg4"] = []contract.Bytes{bytesVal}
	params["arg5"] = []contract.Address{addressVal}
	params["arg6"] = []contract.Struct{contract.MustStructOf(structVal)}
	options := make(contract.Options)
	r, err := h.Call(method, params, options)
	assert.NoError(t, err)
	t.Logf("params:%+v ret:%+v", params, r)

	ret, ok := r.(contract.Struct)
	assert.True(t, ok)
	rp := ret.Params()
	for k, v := range params {
		if k == "arg6" {
			al := rp[k].([]contract.Struct)
			for i, es := range v.([]contract.Struct) {
				assert.Equal(t, es.Params(), al[i].Params())
			}
		} else {
			assert.Equal(t, v, rp[k])
		}
	}
}

func Test_callOptional(t *testing.T) {
	h, _ := handler(t, NetworkTypeEth2)
	method := "callOptional"
	params := make(contract.Params)
	options := make(contract.Options)
	r, err := h.Call(method, params, options)
	assert.NoError(t, err)
	assert.Equal(t, "callOptional()", string(r.(contract.String)))
	t.Log(r)

	params["arg1"] = stringVal
	r, err = h.Call(method, params, options)
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("callOptional(%s)", stringVal), string(r.(contract.String)))
	t.Log(r)
}

func autoSign(w types.Wallet, h *Handler, method string, params contract.Params) (contract.TxID, error) {
	gasPrice, err := h.a.SuggestGasPrice(context.Background())
	if err != nil {
		return nil, err
	}
	opt := &InvokeOptions{
		From:     contract.Address(w.Address()),
		GasPrice: contract.MustIntegerOf(gasPrice),
	}
	options, err := contract.EncodeOptions(opt)
	if err != nil {
		return nil, err
	}
	txId, err := h.Invoke(method, params, options)
	if err != nil {
		if rse, ok := err.(contract.RequireSignatureError); ok {
			var sig []byte
			if sig, err = w.Sign(rse.Data()); err != nil {
				return nil, errors.Wrapf(err, "fail to Sign err:%s", err.Error())
			}
			if err = contract.DecodeOptions(rse.Options(), opt); err != nil {
				return nil, err
			}
			opt.Signature = sig
			if options, err = contract.EncodeOptions(opt); err != nil {
				return nil, err
			}
			return h.Invoke(method, params, options)
		} else {
			return nil, err
		}
	}
	return txId, nil
}

func assertBaseEvent(t *testing.T, el contract.BaseEvent, address contract.Address, signature string, indexed int, params contract.Params) {
	assert.Equal(t, address, el.Address())
	assert.True(t, el.SignatureMatcher().Match(signature))
	assert.Equal(t, indexed, el.Indexed())
	for i := 0; i < el.Indexed(); i++ {
		name := fmt.Sprintf("arg%d", i+1)
		v, ok := params[name]
		assert.True(t, ok, "not found param %s", name)
		if ok {
			nt, _ := NewTopic(v)
			assert.True(t, el.IndexedValue(i).Match(v),
				"index:%d expected:%v actual:%v", i, nt, el.IndexedValue(i))
		}
	}
}

func assertEvent(t *testing.T, e contract.Event, address contract.Address, signature string, indexed int, params contract.Params) {
	assertBaseEvent(t, e, address, signature, indexed, params)
	assertParams(t, e.Params(), params)
}

func assertParams(t *testing.T, e contract.Params, a interface{}) {
	var ap contract.Params
	switch at := a.(type) {
	case contract.Struct:
		ap = at.Params()
	case contract.Params:
		ap = at
	default:
		assert.FailNow(t, "invalid actual type", "%T", a)
	}
	assert.Equal(t, len(e), len(ap))
	for k, ev := range e {
		av, ok := ap[k]
		assert.True(t, ok, "Params name:%s not found", k)
		assertParam(t, k, ev, av)
	}
}

func assertParam(t *testing.T, k string, e, a interface{}) {
	switch v := e.(type) {
	case contract.EventIndexedValue:
		assert.True(t, v.Match(a), "EventIndexedValue name:%s expected:%v actual:%v", k, e, a)
	case contract.Struct:
		assertStruct(t, e, a)
	case contract.Params:
		assertParams(t, v, a)
	case contract.Bytes, contract.Integer, contract.Address, contract.String, contract.Boolean:
		assert.Equal(t, e, a, "Equals name:%s expected:%v, actual:%v", k, e, a)
	default:
		ev, av := reflect.ValueOf(e), reflect.ValueOf(a)
		switch ev.Kind() {
		case reflect.Array, reflect.Slice:
			assert.Equal(t, ev.Kind(), av.Kind())
			assert.Equal(t, ev.Len(), av.Len())
			for i := 0; i < ev.Len(); i++ {
				assertParam(t, k, ev.Index(i).Interface(), av.Index(i).Interface())
			}
		default:
			assert.FailNow(t, "invalid compare expected", "%T", e)
		}
	}
}

func assertStruct(t *testing.T, expected, actual interface{}) {
	var ep contract.Params
	switch e := expected.(type) {
	case contract.Struct:
		ep = e.Params()
	case contract.Params:
		ep = e
	default:
		assert.FailNow(t, "invalid expected type", "%T", expected)
	}
	a, ok := actual.(contract.Struct)
	if !ok {
		assert.FailNow(t, "invalid actual type", "%T", actual)
	}
	assertParams(t, ep, a.Params())
}

func Test_invokeInteger(t *testing.T) {
	h, a := handler(t, NetworkTypeEth2)
	method := "invokeInteger"
	params := make(contract.Params)
	params["arg1"] = byteVal
	params["arg2"] = shortVal
	params["arg3"] = intVal
	params["arg4"] = longVal
	params["arg5"] = int24Val
	params["arg6"] = int40Val
	params["arg7"] = int72Val
	params["arg8"] = bigIntegerVal

	txId, err := autoSign(w, h, method, params)
	assert.NoError(t, err)
	assert.NotNil(t, txId)
	t.Log(txId)

	r, err := a.GetResult(txId)
	assert.NoError(t, err)
	assert.True(t, r.Success())
	assert.Equal(t, 1, len(r.Events()))

	el := r.Events()[0]
	address := contract.Address(h.address.String())
	sig := "IntegerEvent(int8,int16,int32,int64,int24,int40,int72,int256)"
	indexed := 3
	assertBaseEvent(t, el, address, sig, indexed, params)

	event := "IntegerEvent"
	evtParams := make(contract.Params)
	for k, v := range params {
		evtParams[k] = v
		ef, err := h.EventFilter(event, evtParams)
		assert.NoError(t, err)

		e, err := ef.Filter(el)
		assert.NoError(t, err)
		assert.NotNil(t, e, "event should match")

		assertEvent(t, e, address, sig, indexed, params)
	}
}

func Test_invokePrimitive(t *testing.T) {
	h, a := handler(t, NetworkTypeEth2)
	method := "invokePrimitive"
	params := make(contract.Params)
	params["arg1"] = bigIntegerVal
	params["arg2"] = booleanVal
	params["arg3"] = stringVal
	params["arg4"] = bytesVal
	params["arg5"] = addressVal

	txId, err := autoSign(w, h, method, params)
	assert.NoError(t, err)
	t.Log(txId)

	r, err := a.GetResult(txId)
	assert.NoError(t, err)
	assert.True(t, r.Success())
	assert.Equal(t, 1, len(r.Events()))

	el := r.Events()[0]
	address := contract.Address(h.address.String())
	sig := "PrimitiveEvent(int256,bool,string,bytes,address)"
	indexed := 3
	assertBaseEvent(t, el, address, sig, indexed, params)

	event := "PrimitiveEvent"
	evtParams := make(contract.Params)
	for k, v := range params {
		evtParams[k] = v
		ef, err := h.EventFilter(event, evtParams)
		assert.NoError(t, err)

		e, err := ef.Filter(el)
		assert.NoError(t, err)
		assert.NotNil(t, e, "event should match")

		assertEvent(t, e, address, sig, indexed, params)
	}
}

func Test_invokeStruct(t *testing.T) {
	h, a := handler(t, NetworkTypeEth2)
	method := "invokeStruct"
	params := make(contract.Params)
	params["arg1"] = contract.MustStructOf(structVal)

	txId, err := autoSign(w, h, method, params)
	assert.NoError(t, err)
	t.Log(txId)

	r, err := a.GetResult(txId)
	assert.NoError(t, err)
	assert.True(t, r.Success())
	assert.Equal(t, 1, len(r.Events()))

	for k, e := range h.out.Events {
		t.Log("abi.event name:", k, "signature:", e.Sig)
	}

	el := r.Events()[0]
	address := contract.Address(h.address.String())
	sig := "StructEvent((bool))"
	indexed := 1
	evtParams := make(contract.Params)
	evtParams["arg1"] = common.HexToHash("0xb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf6")
	assertBaseEvent(t, el, address, sig, indexed, params)

	event := "StructEvent"
	ef, err := h.EventFilter(event, params)
	assert.NoError(t, err)
	t.Log("signature", ef.Signature())

	e, err := ef.Filter(el)
	assert.NoError(t, err)
	assert.NotNil(t, e, "event should match")

	assertEvent(t, e, address, sig, indexed, params)
}

func Test_invokeArray(t *testing.T) {
	h, a := handler(t, NetworkTypeEth2)
	method := "invokeArray"
	params := make(contract.Params)
	params["arg1"] = []contract.Integer{bigIntegerVal}
	params["arg2"] = []contract.Boolean{booleanVal}
	params["arg3"] = []contract.String{stringVal}
	params["arg4"] = []contract.Bytes{bytesVal}
	params["arg5"] = []contract.Address{addressVal}
	params["arg6"] = []contract.Struct{contract.MustStructOf(structVal)}

	txId, err := autoSign(w, h, method, params)
	assert.NoError(t, err)
	t.Log(txId)

	r, err := a.GetResult(txId)
	assert.NoError(t, err)
	assert.True(t, r.Success())
	assert.Equal(t, 1, len(r.Events()))

	for k, e := range h.out.Events {
		t.Log("abi.event name:", k, "signature:", e.Sig)
	}

	el := r.Events()[0]
	address := contract.Address(h.address.String())
	sig := "ArrayEvent(int256[],bool[],string[],bytes[],address[],(bool)[])"
	indexed := 3
	assertBaseEvent(t, el, address, sig, indexed, params)

	event := "ArrayEvent"
	evtParams := make(contract.Params)
	for k, v := range params {
		evtParams[k] = v
		ef, err := h.EventFilter(event, evtParams)
		assert.NoError(t, err)

		e, err := ef.Filter(el)
		assert.NoError(t, err)
		assert.NotNil(t, e, "event should match")

		assertEvent(t, e, address, sig, indexed, params)
	}

	ef, err := h.EventFilter(event, params)
	assert.NoError(t, err)

	e, err := ef.Filter(el)
	assert.NoError(t, err)
	assert.NotNil(t, e, "event should match")

	assertEvent(t, e, address, sig, indexed, params)

	ch := make(chan contract.Event, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		err = h.MonitorEvent(context.Background(), func(e contract.Event) error {
			ch <- e
			return nil
		}, map[string][]contract.Params{event: nil}, r.BlockHeight())
		assert.Equal(t, ctx.Err(), err)
	}()
	select {
	case actual := <-ch:
		t.Logf("%+v", actual)
		assertEvent(t, actual, address, sig, indexed, evtParams)
	case <-time.After(time.Second * 10):
		assert.FailNow(t, "timeout")
	}
	cancel()
}

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
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/icon-project/btp2/chain/icon/client"
	"github.com/icon-project/btp2/common/codec"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/wallet"
	"github.com/stretchr/testify/assert"

	"github.com/icon-project/btp-sdk/contract"
)

const (
	specFile = "../../example/javascore/build/generated/contractSpec.json"
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
	assert.True(t, ok, "invalid actual type %T", actual)
	ap := a.Params()
	assert.Equal(t, ep, ap)
}

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
	h, _ := handler(t, NetworkTypeIcon)
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
	h, _ := handler(t, NetworkTypeIcon)
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
	h, _ := handler(t, NetworkTypeIcon)
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
	h, _ := handler(t, NetworkTypeIcon)
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
	h, _ := handler(t, NetworkTypeIcon)
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
	h, _ := handler(t, NetworkTypeIcon)
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

func autoSign(w wallet.Wallet, h *Handler, method string, params contract.Params) (contract.TxID, error) {
	opt := &InvokeOptions{
		From: contract.Address(w.Address()),
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
		v := params[fmt.Sprintf("arg%d", i+1)]
		assert.True(t, el.IndexedValue(i).Match(v),
			"index:%d expected:%v actual:%v", i, v, el.IndexedValue(i))
	}
}

func assertEvent(t *testing.T, e contract.Event, address contract.Address, signature string, indexed int, params contract.Params) {
	assertBaseEvent(t, e, address, signature, indexed, params)
	for k, v := range params {
		p := e.Params()[k]
		if iv, ok := p.(contract.EventIndexedValue); ok {
			assert.True(t, iv.Match(v))
		} else {
			assert.Equal(t, v, p)
		}
	}
}

func Test_invokeInteger(t *testing.T) {
	h, a := handler(t, NetworkTypeIcon)
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
	t.Log(txId)

	r, err := a.GetResult(txId)
	assert.NoError(t, err)
	assert.True(t, r.Success())
	assert.Equal(t, 1, len(r.Events()))

	el := r.Events()[0]
	address := contract.Address(h.address)
	sig := "IntegerEvent(int,int,int,int,int,int,int,int)"
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
	h, a := handler(t, NetworkTypeIcon)
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
	address := contract.Address(h.address)
	sig := "PrimitiveEvent(int,bool,str,bytes,Address)"
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
	h, a := handler(t, NetworkTypeIcon)
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

	el := r.Events()[0]
	address := contract.Address(h.address)
	sig := "StructEvent(bytes)"
	indexed := 1
	evtParams := make(contract.Params)
	evtParams["arg1"] = contract.Bytes(codec.RLP.MustMarshalToBytes(structVal))
	assertBaseEvent(t, el, address, sig, indexed, evtParams)

	event := "StructEvent"
	ef, err := h.EventFilter(event, evtParams)
	assert.NoError(t, err)

	e, err := ef.Filter(el)
	assert.NoError(t, err)
	assert.NotNil(t, e, "event should match")

	assertEvent(t, e, address, sig, indexed, evtParams)
}

func Test_invokeArray(t *testing.T) {
	h, a := handler(t, NetworkTypeIcon)
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

	el := r.Events()[0]
	address := contract.Address(h.address)
	sig := "ArrayEvent(bytes,bytes,bytes,bytes,bytes,bytes)"
	indexed := 3
	evtParams := make(contract.Params)
	bs, _ := bigIntegerVal.AsBytes()
	evtParams["arg1"] = contract.Bytes(codec.RLP.MustMarshalToBytes([][]byte{bs}))
	evtParams["arg2"] = contract.Bytes(codec.RLP.MustMarshalToBytes(params["arg2"]))
	evtParams["arg3"] = contract.Bytes(codec.RLP.MustMarshalToBytes(params["arg3"]))
	evtParams["arg4"] = contract.Bytes(codec.RLP.MustMarshalToBytes(params["arg4"]))
	b, _ := client.Address(addressVal).Value()
	evtParams["arg5"] = contract.Bytes(codec.RLP.MustMarshalToBytes([]contract.Bytes{b}))
	evtParams["arg6"] = contract.Bytes(codec.RLP.MustMarshalToBytes([]interface{}{structVal}))
	assertBaseEvent(t, el, address, sig, indexed, evtParams)

	event := "ArrayEvent"
	ef, err := h.EventFilter(event, evtParams)
	assert.NoError(t, err)

	e, err := ef.Filter(el)
	assert.NoError(t, err)
	assert.NotNil(t, e, "event should match")

	assertEvent(t, e, address, sig, indexed, evtParams)

	var height int64
	if height, err = r.(*TxResult).BlockHeight.Value(); err != nil {
		assert.FailNow(t, "invalid blockHeight err:%s", err.Error())
	}
	ch := make(chan contract.Event, 1)
	go func() {
		err = h.MonitorEvent(func(e contract.Event) error {
			ch <- e
			return nil
		}, map[string][]contract.Params{event: nil}, height)
	}()
	select {
	case actual := <-ch:
		t.Logf("%+v", actual)
		assertEvent(t, actual, address, sig, indexed, evtParams)
	case <-time.After(time.Second * 5):
		assert.FailNow(t, "timeout")
	}
}

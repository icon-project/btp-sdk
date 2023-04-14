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
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"
	"github.com/icon-project/btp2/common/wallet"
	"github.com/stretchr/testify/assert"

	"github.com/icon-project/btp-sdk/contract"
)

const (
	endpoint     = "http://localhost:8545"
	spec         = "../../example/solidity/artifacts/contracts/HelloWorld.sol/HelloWorld.json"
	addr         = "0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9"
	keystoreName = "../../example/solidity/test/keystore.json"
	keystorePass = "hardhat"
)

var (
	w             = loadWallet(keystoreName, keystorePass)
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

	adaptorOpt = AdaptorOption{}
)

type Wallet struct {
	*keystore.Key
	PubKey *ecdsa.PublicKey
}

func (w *Wallet) Address() string {
	return w.Key.Address.String()
}

func (w *Wallet) Sign(data []byte) ([]byte, error) {
	return crypto.Sign(data, w.PrivateKey)
}

func (w *Wallet) PublicKey() []byte {
	return crypto.FromECDSAPub(w.PubKey)
}

func (w *Wallet) ECDH(pubKey []byte) ([]byte, error) {
	return nil, errors.New("not implemented")
}

func loadWallet(keyStoreFile, keyPassword string) wallet.Wallet {
	ks, err := ioutil.ReadFile(keyStoreFile)
	if err != nil {
		panic(err)
	}
	key, err := keystore.DecryptKey(ks, keyPassword)
	if err != nil {
		panic(err)
	}
	return &Wallet{
		Key:    key,
		PubKey: &key.PrivateKey.PublicKey,
	}
}

func handler(t *testing.T) *Handler {
	l := log.GlobalLogger()
	l.SetLevel(log.TraceLevel)
	opt, err := contract.EncodeOptions(adaptorOpt)
	if err != nil {
		assert.FailNow(t, "fail to EncodeOptions", err)
	}
	a, err := contract.NewAdaptor(NetworkType, endpoint, opt, l)
	if err != nil {
		assert.FailNow(t, "fail to NewAdaptor", err)
	}
	b, err := ioutil.ReadFile(spec)
	if err != nil {
		assert.FailNow(t, "fail to read file", err)
	}
	st := struct {
		ABI json.RawMessage `json:"abi"`
	}{}
	err = json.Unmarshal(b, &st)
	if err != nil {
		assert.FailNow(t, "fail to Unmarshal", err)
	}
	h, err := NewHandler(st.ABI, common.HexToAddress(addr), a.(*Adaptor), l)
	if err != nil {
		assert.FailNow(t, "fail to NewHandler", err)
	}
	return h.(*Handler)
}

func Test_callInteger(t *testing.T) {
	h := handler(t)
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

	ret, ok := r.(map[string]interface{})
	assert.True(t, ok)
	for k, v := range params {
		assert.Equal(t, v, ret[k])
	}
}

func Test_callPrimitive(t *testing.T) {
	h := handler(t)
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

	ret, ok := r.(map[string]interface{})
	assert.True(t, ok)
	for k, v := range params {
		assert.Equal(t, v, ret[k])
	}
}

func Test_callStruct(t *testing.T) {
	h := handler(t)
	method := "callStruct"
	input := struct {
		BooleanVal contract.Boolean
	}{
		BooleanVal: booleanVal,
	}
	params := make(contract.Params)
	params["arg1"] = contract.StructToMap(input)
	options := make(contract.Options)
	r, err := h.Call(method, params, options)
	assert.NoError(t, err)
	t.Log(r)

	ret, ok := r.(map[string]interface{})
	assert.True(t, ok)
	for k, v := range params["arg1"].(map[string]interface{}) {
		assert.Equal(t, v, ret[k])
	}
}

func Test_callPrimitiveArray(t *testing.T) {
	h := handler(t)
	method := "callPrimitiveArray"
	params := make(contract.Params)
	params["arg1"] = []contract.Integer{bigIntegerVal}
	params["arg2"] = []contract.Boolean{booleanVal}
	params["arg3"] = []contract.String{stringVal}
	params["arg4"] = []contract.Bytes{bytesVal}
	params["arg5"] = []contract.Address{addressVal}
	options := make(contract.Options)
	r, err := h.Call(method, params, options)
	assert.NoError(t, err)
	t.Logf("params:%+v ret:%+v", params, r)

	ret, ok := r.(map[string]interface{})
	assert.True(t, ok)
	t.Logf("%+v\n", ret)
	for k, v := range params {
		pl := reflect.ValueOf(v)
		rl := reflect.ValueOf(ret[k])
		t.Logf("pl:%+v rl:%+v\n", pl, rl)
		assert.Equal(t, pl.Len(), rl.Len())
		for i := 0; i < pl.Len(); i++ {
			assert.Equal(t, pl.Index(i).Interface(), rl.Index(i).Interface())
		}
	}
}

func Test_callOptional(t *testing.T) {
	h := handler(t)
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
	gasPrice, err := h.a.SuggestGasPrice(context.Background())
	if err != nil {
		return nil, err
	}
	opt := &InvokeOptions{
		From:     contract.Address(w.Address()),
		GasPrice: contract.FromBigInt(gasPrice),
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
	assert.True(t, el.Signature().Match(signature))
	assert.Equal(t, indexed, el.Indexed())
	for i := 0; i < el.Indexed(); i++ {
		v := params[fmt.Sprintf("arg%d", i+1)]
		assert.True(t, el.IndexedValue(i).Match(v))
	}
}

func assertEvent(t *testing.T, e contract.Event, address contract.Address, signature string, indexed int, params contract.Params) {
	assertBaseEvent(t, e, address, signature, indexed, params)
	for k, v := range params {
		p := e.Params()[k]
		if hv, ok := p.(contract.HashValue); ok {
			assert.True(t, hv.Match(v))
		} else {
			assert.Equal(t, v, p)
		}
	}
}

func Test_invokeInteger(t *testing.T) {
	h := handler(t)
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

	r, err := h.GetResult(txId)
	assert.NoError(t, err)
	assert.True(t, r.Success())
	assert.Equal(t, 1, len(r.Events()))

	el := r.Events()[0]
	address := contract.Address(h.address.String())
	sig := "IntegerEvent(int8,int16,int32,int64,int24,int40,int72,int256)"
	indexed := 3
	assertBaseEvent(t, el, address, sig, indexed, params)

	event := "IntegerEvent"
	ef, err := h.EventFilter(event, params)
	assert.NoError(t, err)

	e, err := ef.Filter(el)
	assert.NoError(t, err)
	assert.NotNil(t, e, "event should match")

	assertEvent(t, e, address, sig, indexed, params)
}

func Test_invokePrimitive(t *testing.T) {
	h := handler(t)
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

	r, err := h.GetResult(txId)
	assert.NoError(t, err)
	assert.True(t, r.Success())
	assert.Equal(t, 1, len(r.Events()))

	el := r.Events()[0]
	address := contract.Address(h.address.String())
	sig := "PrimitiveEvent(int256,bool,string,bytes,address)"
	indexed := 3
	assertBaseEvent(t, el, address, sig, indexed, params)

	event := "PrimitiveEvent"
	ef, err := h.EventFilter(event, params)
	assert.NoError(t, err)

	e, err := ef.Filter(el)
	assert.NoError(t, err)
	assert.NotNil(t, e, "event should match")

	assertEvent(t, e, address, sig, indexed, params)
}

func Test_invokeStruct(t *testing.T) {
	h := handler(t)
	method := "invokeStruct"
	input := struct {
		BooleanVal contract.Boolean
	}{
		BooleanVal: booleanVal,
	}
	params := make(contract.Params)
	params["arg1"] = contract.StructToMap(input)

	txId, err := autoSign(w, h, method, params)
	assert.NoError(t, err)
	t.Log(txId)

	r, err := h.GetResult(txId)
	assert.NoError(t, err)
	assert.True(t, r.Success())
	assert.Equal(t, 1, len(r.Events()))

	for k, e := range h.out.Events {
		t.Log("abi.event name:", k, "signature:", e.Sig)
	}

	el := r.Events()[0]
	address := contract.Address(h.address.String())
	sig := "StructEvent((bool))"
	indexed := 0
	evtParams := make(contract.Params)
	evtParams["arg1"] = params["arg1"]
	assertBaseEvent(t, el, address, sig, indexed, evtParams)

	event := "StructEvent"
	ef, err := h.EventFilter(event, evtParams)
	assert.NoError(t, err)
	t.Log("signature", ef.Signature())

	e, err := ef.Filter(el)
	assert.NoError(t, err)
	assert.NotNil(t, e, "event should match")

	assertEvent(t, e, address, sig, indexed, evtParams)
}

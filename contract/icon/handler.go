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
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/icon-project/btp2/chain/icon/client"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
)

func NewHandler(spec []byte, address client.Address, a *Adaptor, l log.Logger) (contract.Handler, error) {
	cspec := contract.Spec{}
	if err := json.Unmarshal(spec, &cspec); err != nil {
		return nil, errors.Wrapf(err, "fail to unmarshal spec err:%s", err.Error())
	}
	return &Handler{
		spec:    cspec,
		address: address,
		a:       a,
		l:       l,
	}, nil
}

type Handler struct {
	spec    contract.Spec
	address client.Address
	a       *Adaptor
	l       log.Logger
}

func (h *Handler) method(name string, readonly bool) (*contract.MethodSpec, error) {
	m, has := h.spec.MethodMap[name]
	if !has {
		return nil, errors.New("not found method")
	}
	if m.ReadOnly != readonly {
		return nil, errors.Errorf("mismatch readonly, expected:%v", readonly)
	}
	return m, nil
}

func (h *Handler) callData(m *contract.MethodSpec, params contract.Params) (*client.CallData, error) {
	var err error
	r := make(map[string]interface{})
	for k, v := range m.InputMap {
		param, ok := params[k]
		if !ok || param == nil {
			if !v.Optional {
				return nil, errors.New("required param " + k)
			}
		} else {
			if r[k], err = encode(v.Type, param); err != nil {
				return nil, err
			}
		}
	}
	return &client.CallData{
		Method: m.Name,
		Params: r,
	}, nil
}

type InvokeOptions struct {
	From      contract.Address
	Value     contract.Integer
	StepLimit contract.Integer
	Timestamp contract.Integer
	Signature contract.Bytes
}

func (h *Handler) Invoke(method string, params contract.Params, options contract.Options) (contract.TxID, error) {
	m, err := h.method(method, false)
	if err != nil {
		return nil, err
	}
	data, err := h.callData(m, params)
	if err != nil {
		return nil, err
	}
	p := h.a.NewTransactionParam(h.address, data)

	//Options convert
	opt := &InvokeOptions{}
	if err = contract.DecodeOptions(options, opt); err != nil {
		return nil, err
	}
	//required fields
	if len(opt.From) == 0 {
		return nil, errors.New("required 'from'")
	}
	p.FromAddress = client.Address(opt.From)
	//optional fields
	if len(opt.Value) > 0 {
		p.Value = client.HexInt(opt.Value)
	}
	if len(opt.StepLimit) > 0 {
		p.StepLimit = client.HexInt(opt.StepLimit)
	}
	//generated fields
	p.Timestamp = client.HexInt(opt.Timestamp)
	if len(opt.Signature) == 0 {
		if len(p.Timestamp) == 0 {
			p.Timestamp = client.NewHexInt(time.Now().UnixNano() / int64(time.Microsecond))
			opt.Timestamp = contract.Integer(p.Timestamp)
			h.l.Debugln("Timestamp generated", p.Timestamp)
			if options, err = contract.EncodeOptions(opt); err != nil {
				return nil, err
			}
		}
		var hash []byte
		if hash, err = h.a.HashForSignature(p); err != nil {
			return nil, err
		}
		return nil, contract.NewRequireSignatureError(hash, options)
	}
	p.Signature = base64.StdEncoding.EncodeToString(opt.Signature)
	//FIXME convert client.HexBytes to contract.TxID
	return h.a.SendTransaction(p)
}

type CallOption struct {
	From contract.Address
}

func (h *Handler) Call(method string, params contract.Params, options contract.Options) (contract.ReturnValue, error) {
	m, err := h.method(method, true)
	if err != nil {
		return nil, err
	}
	data, err := h.callData(m, params)
	if err != nil {
		return nil, err
	}
	p := &client.CallParam{
		ToAddress: h.address,
		DataType:  "call",
		Data:      data,
	}

	//Options convert
	opt := &CallOption{}
	if err = contract.DecodeOptions(options, opt); err != nil {
		return nil, err
	}
	//optional fields
	p.FromAddress = client.Address(opt.From)

	var ret interface{}
	if err = h.a.Call(p, &ret); err != nil {
		return nil, err
	}
	return decode(m.Output, ret)
}

func (h *Handler) EventFilter(name string, params contract.Params) (contract.EventFilter, error) {
	spec, has := h.spec.EventMap[name]
	if !has {
		return nil, errors.New("not found event")
	}
	if err := contract.ParamsTypeCheck(spec, params); err != nil {
		return nil, err
	}
	return &EventFilter{
		spec:    *spec,
		address: contract.Address(h.address),
		params:  params,
	}, nil
}

func (h *Handler) Spec() contract.Spec {
	return h.spec
}

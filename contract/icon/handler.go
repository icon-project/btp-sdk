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
	"context"
	"encoding/base64"
	"time"

	"github.com/icon-project/btp2/chain/icon/client"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
)

func NewHandler(spec []byte, address client.Address, a *Adaptor, l log.Logger) (contract.Handler, error) {
	s, err := NewSpec(spec)
	if err != nil {
		return nil, err
	}
	return &Handler{
		spec:    *s,
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
		return nil, contract.ErrorCodeNotFoundMethod.Errorf("not found method:%s", name)
	}
	if m.ReadOnly != readonly {
		return nil, contract.ErrorCodeMismatchReadonly.Errorf("mismatch readonly, method:%s expected:%v", name, readonly)
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
				return nil, contract.ErrorCodeInvalidParam.Errorf("required param:%s", k)
			}
		} else {
			if r[k], err = encode(v.Type, param); err != nil {
				return nil, contract.ErrorCodeInvalidParam.Wrapf(err, "invalid param:%s err:%s", k, err.Error())
			}
		}
		h.l.Tracef("callData name:%s param:%v encoded:%v type:%T\n",
			v.Name, param, r[k], r[k])
	}
	return &client.CallData{
		Method: m.Name,
		Params: r,
	}, nil
}

type InvokeOptions struct {
	From      contract.Address `json:"from,omitempty"`
	Value     contract.Integer `json:"value,omitempty"`
	StepLimit contract.Integer `json:"stepLimit,omitempty"`
	Timestamp contract.Integer `json:"timestamp,omitempty"`
	Signature contract.Bytes   `json:"signature,omitempty"`
	Estimate  contract.Boolean `json:"estimate,omitempty"`
}

func (h *Handler) newTransactionParam(opt *InvokeOptions, data *client.CallData) (*client.TransactionParam, error) {
	p := h.a.NewTransactionParam(h.address, data)
	if len(opt.From) == 0 {
		return nil, contract.ErrorCodeInvalidOption.Errorf("required 'from'")
	}
	p.FromAddress = client.Address(opt.From)
	if _, err := p.FromAddress.Value(); err != nil {
		return nil, contract.ErrorCodeInvalidOption.Wrapf(err, "invalid 'from' err:%s", err.Error())
	}
	if len(opt.StepLimit) > 0 {
		p.StepLimit = client.HexInt(opt.StepLimit)
	} else {
		if opt.Estimate {
			stepLimit, err := h.a.EstimateStep(client.NewTransactionParamForEstimate(p))
			if err != nil {
				return nil, err
			}
			p.StepLimit = client.NewHexInt(stepLimit)
		} else {
			p.StepLimit = DefaultStepLimit
		}
		opt.StepLimit = contract.Integer(p.StepLimit)
	}
	if len(opt.Timestamp) > 0 {
		p.Timestamp = client.HexInt(opt.Timestamp)
	} else {
		p.Timestamp = client.NewHexInt(time.Now().UnixNano() / int64(time.Microsecond))
		opt.Timestamp = contract.Integer(p.Timestamp)
	}
	//optional fields
	if len(opt.Value) > 0 {
		p.Value = client.HexInt(opt.Value)
	}
	return p, nil
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
	opt := &InvokeOptions{}
	if err = contract.DecodeOptions(options, opt); err != nil {
		return nil, err
	}
	p, err := h.newTransactionParam(opt, data)
	if err != nil {
		return nil, err
	}

	if len(opt.Signature) == 0 {
		var hash []byte
		if hash, err = h.a.HashForSignature(p); err != nil {
			return nil, err
		}
		if options, err = contract.EncodeOptions(opt); err != nil {
			return nil, err
		}
		return nil, contract.NewRequireSignatureError(hash, options)
	}
	p.Signature = base64.StdEncoding.EncodeToString(opt.Signature)
	txh, err := h.a.SendTransaction(p)
	if err != nil {
		return nil, errors.Wrapf(err, "fail to SendTransaction err:%s", err.Error())
	}
	return NewTxID(txh), nil
}

type CallOption struct {
	From contract.Address `json:"from,omitempty"`
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
		return nil, errors.Wrapf(err, "fail to Call err:%s", err.Error())
	}
	return decode(m.Output, ret)
}

func (h *Handler) EventFilter(name string, params contract.Params) (contract.EventFilter, error) {
	spec, has := h.spec.EventMap[name]
	if !has {
		return nil, contract.ErrorCodeNotFoundEvent.Errorf("not found event:%s", name)
	}
	validParams, err := contract.ParamsOfWithSpec(spec.InputMap, params)
	if err != nil {
		return nil, err
	}
	return &EventFilter{
		spec:    *spec,
		address: contract.Address(h.address),
		params:  validParams,
	}, nil
}

func (h *Handler) Spec() contract.Spec {
	return h.spec
}

func (h *Handler) Address() contract.Address {
	return contract.Address(h.address)
}

func (h *Handler) MonitorEvent(
	ctx context.Context,
	cb contract.EventCallback,
	nameToParams map[string][]contract.Params,
	height int64) error {
	efs := make([]contract.EventFilter, 0)
	for name, l := range nameToParams {
		if len(l) == 0 {
			l = []contract.Params{nil}
		}
		for _, params := range l {
			ef, err := h.EventFilter(name, params)
			if err != nil {
				return err
			}
			efs = append(efs, ef)
		}
	}
	return h.a.MonitorEvent(ctx, cb, efs, height)
}

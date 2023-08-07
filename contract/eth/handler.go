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
	"bytes"
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
)

func NewHandler(spec []byte, address common.Address, a *Adaptor, l log.Logger) (contract.Handler, error) {
	out, err := abi.JSON(bytes.NewBuffer(spec))
	if err != nil {
		return nil, err
	}
	in, err := SpecFromABI(out)
	if err != nil {
		return nil, err
	}
	return &Handler{
		in:      *in,
		out:     out,
		address: address,
		a:       a,
		l:       l,
		signer:  types.LatestSignerForChainID(a.chainID),
	}, nil
}

type Handler struct {
	in      contract.Spec
	out     abi.ABI
	address common.Address
	a       *Adaptor
	l       log.Logger

	signer types.Signer
}

func (h *Handler) method(name string, params contract.Params, readonly bool) (*contract.MethodSpec, *abi.Method, error) {
	methods := make([]abi.Method, 0)
	for _, m := range h.out.Methods {
		if m.RawName == name {
			if m.IsConstant() != readonly {
				return nil, nil, contract.ErrorCodeMismatchReadonly.Errorf("mismatch readonly, method:%s expected:%v", name, readonly)
			}
			methods = append(methods, m)
		}
	}
	in := h.in.MethodMap[name]
	if len(methods) == 1 {
		return in, &methods[0], nil
	} else {
		for i, m := range methods {
			if len(m.Inputs) == len(params) {
				return in, &methods[i], nil
			}
		}
		return nil, nil, contract.ErrorCodeNotFoundMethod.Errorf("not found method:%s", name)
	}
}

func (h *Handler) callData(m *abi.Method, params contract.Params) (b []byte, err error) {
	r := make([]interface{}, len(m.Inputs))
	for i, v := range m.Inputs {
		param, ok := params[v.Name]
		if !ok || param == nil {
			return nil, contract.ErrorCodeInvalidParam.Errorf("required param:%s", v.Name)
		}
		if r[i], err = encode(v.Type, param); err != nil {
			return nil, contract.ErrorCodeInvalidParam.Wrapf(err, "invalid param:%s err:%s", v.Name, err.Error())
		}
		h.l.Tracef("callData index:%d name:%s param:%v encoded:%v type:%T\n",
			i, v.Name, param, r[i], r[i])
	}
	if b, err = h.out.Pack(m.Name, r...); err != nil {
		return nil, errors.Wrapf(err, "fail to Pack err:%s", err.Error())
	}
	return b, nil
}

type InvokeOptions struct {
	From      contract.Address
	Value     contract.Integer
	GasPrice  contract.Integer
	GasLimit  contract.Integer
	GasFeeCap contract.Integer
	GasTipCap contract.Integer
	Nonce     contract.Integer
	Signature contract.Bytes
	Estimate  contract.Boolean
}

type baseTx struct {
	ChainID   *big.Int
	To        *common.Address `rlp:"nil"` // nil means contract creation
	Data      []byte          // contract invocation input data
	Value     *big.Int        // wei amount
	GasLimit  uint64          // gas limit
	Nonce     uint64          // nonce of sender account
	GasPrice  *big.Int        // wei per gas for LegacyTx
	GasTipCap *big.Int        // wei per gas for DynamicFeeTx
	GasFeeCap *big.Int        // wei per gas for DynamicFeeTx
}

func (p *baseTx) TxData() types.TxData {
	if p.GasPrice != nil {
		return &types.LegacyTx{
			Nonce:    p.Nonce,
			Gas:      p.GasLimit,
			Value:    p.Value,
			GasPrice: p.GasPrice,
			To:       p.To,
			Data:     p.Data,
		}
	} else {
		return &types.DynamicFeeTx{
			Nonce:     p.Nonce,
			Gas:       p.GasLimit,
			Value:     p.Value,
			ChainID:   p.ChainID,
			GasFeeCap: p.GasFeeCap,
			GasTipCap: p.GasTipCap,
			To:        p.To,
			Data:      p.Data,
		}
	}
}

func (h *Handler) newBaseTx(opt *InvokeOptions, data []byte) (p *baseTx, err error) {
	p = &baseTx{
		ChainID:  h.a.chainID,
		To:       &h.address,
		Data:     data,
		GasLimit: DefaultGasLimit,
	}
	if len(opt.Value) > 0 {
		if p.Value, err = opt.Value.AsBigInt(); err != nil {
			return nil, contract.ErrorCodeInvalidOption.Wrapf(err, "invalid 'Value' err:%s", err.Error())
		}
	}
	if len(opt.GasLimit) > 0 {
		if p.GasLimit, err = opt.GasLimit.AsUint64(); err != nil {
			return nil, contract.ErrorCodeInvalidOption.Wrapf(err, "invalid 'GasLimit' err:%s", err.Error())
		}
	}
	if len(opt.Nonce) > 0 {
		if p.Nonce, err = opt.Nonce.AsUint64(); err != nil {
			return nil, contract.ErrorCodeInvalidOption.Wrapf(err, "invalid 'Nonce' err:%s", err.Error())
		}
	}
	if len(opt.GasPrice) > 0 {
		if p.GasPrice, err = opt.GasPrice.AsBigInt(); err != nil {
			return nil, contract.ErrorCodeInvalidOption.Wrapf(err, "invalid 'GasPrice' err:%s", err.Error())
		}
	}
	if len(opt.GasFeeCap) > 0 {
		if p.GasFeeCap, err = opt.GasFeeCap.AsBigInt(); err != nil {
			return nil, contract.ErrorCodeInvalidOption.Wrapf(err, "invalid 'GasFeeCap' err:%s", err.Error())
		}
	}
	if len(opt.GasTipCap) > 0 {
		if p.GasTipCap, err = opt.GasTipCap.AsBigInt(); err != nil {
			return nil, contract.ErrorCodeInvalidOption.Wrapf(err, "invalid 'GasTipCap' err:%s", err.Error())
		}
	}
	if p.GasPrice != nil && (p.GasFeeCap != nil || p.GasTipCap != nil) {
		return nil, contract.ErrorCodeInvalidOption.Errorf("both GasPrice and (GasFeeCap or GasTipCap) specified")
	}
	if opt.Estimate {
		gasLimit, err := h.a.EstimateGas(context.Background(), ethereum.CallMsg{
			To:    &h.address,
			Data:  p.Data,
			Value: p.Value,
		})
		if err != nil {
			return nil, err
		}
		if len(opt.GasLimit) == 0 {
			p.GasLimit = gasLimit
			opt.GasLimit, _ = contract.IntegerOf(gasLimit)
		}
	}
	return p, nil
}

func (h *Handler) prepareSign(opt *InvokeOptions, p *baseTx) (optUpdated bool, err error) {
	if len(opt.GasLimit) == 0 {
		//if p.GasLimit, err = h.a.EstimateGas(context.Background(), ethereum.CallMsg{
		//	To:    &h.address,
		//	Data:  p.Data,
		//	Value: p.Value,
		//}); err != nil {
		//	return false, errors.Wrapf(err, "fail to EstimateGas err:%s", err.Error())
		//}
		opt.GasLimit = contract.MustIntegerOf(p.GasLimit)
		optUpdated = true
	}
	if len(opt.Nonce) == 0 {
		if len(opt.From) == 0 {
			return false, contract.ErrorCodeInvalidOption.Errorf("required 'from'")
		}
		if !common.IsHexAddress(string(opt.From)) {
			return false, contract.ErrorCodeInvalidOption.Errorf("invalid 'from'")
		}
		from := common.HexToAddress(string(opt.From))
		if p.Nonce, err = h.a.PendingNonceAt(context.Background(), from); err != nil {
			return false, errors.Wrapf(err, "fail to PendingNonceAt err:%s", err.Error())
		}
		opt.Nonce = contract.MustIntegerOf(p.Nonce)
		optUpdated = true
	}

	if p.GasPrice == nil {
		if p.GasFeeCap == nil || p.GasTipCap == nil {
			var head *types.Header
			if head, err = h.a.HeaderByNumber(context.Background(), nil); err != nil {
				return false, errors.Wrapf(err, "fail to HeaderByNumber err:%s", err.Error())
			}
			if head.BaseFee != nil {
				if p.GasTipCap == nil {
					if p.GasTipCap, err = h.a.SuggestGasTipCap(context.Background()); err != nil {
						return false, errors.Wrapf(err, "fail to SuggestGasTipCap err:%s", err.Error())
					}
					opt.GasTipCap = contract.MustIntegerOf(p.GasTipCap)
					optUpdated = true
				}
				if p.GasFeeCap == nil {
					p.GasFeeCap = new(big.Int).Add(
						p.GasTipCap,
						new(big.Int).Mul(head.BaseFee, big.NewInt(2)),
					)
					opt.GasFeeCap = contract.MustIntegerOf(p.GasFeeCap)
					optUpdated = true
				}
				if p.GasFeeCap.Cmp(p.GasTipCap) < 0 {
					return false, contract.ErrorCodeInvalidOption.Errorf(
						"GasFeeCap (%v) < GasTipCap (%v)", p.GasFeeCap, p.GasTipCap)
				}
			} else {
				if p.GasPrice, err = h.a.SuggestGasPrice(context.Background()); err != nil {
					return false, err
				}
				opt.GasPrice = contract.MustIntegerOf(p.GasPrice)
				optUpdated = true
			}
		}
	}
	return optUpdated, nil
}

func (h *Handler) newTxData(p *baseTx) types.TxData {
	if p.GasPrice != nil {
		return &types.LegacyTx{
			Nonce:    p.Nonce,
			Gas:      p.GasLimit,
			Value:    p.Value,
			GasPrice: p.GasPrice,
			To:       p.To,
			Data:     p.Data,
		}
	} else {
		return &types.DynamicFeeTx{
			Nonce:     p.Nonce,
			Gas:       p.GasLimit,
			Value:     p.Value,
			ChainID:   h.a.chainID,
			GasFeeCap: p.GasFeeCap,
			GasTipCap: p.GasTipCap,
			To:        &h.address,
			Data:      p.Data,
		}
	}
}

func (h *Handler) Invoke(method string, params contract.Params, options contract.Options) (contract.TxID, error) {
	_, out, err := h.method(method, params, false)
	if err != nil {
		return nil, err
	}
	data, err := h.callData(out, params)
	if err != nil {
		return nil, err
	}

	//options convert
	opt := &InvokeOptions{}
	if err = contract.DecodeOptions(options, opt); err != nil {
		return nil, err
	}
	p, err := h.newBaseTx(opt, data)
	if err != nil {
		return nil, err
	}

	if len(opt.Signature) == 0 {
		optUpdated := false
		if optUpdated, err = h.prepareSign(opt, p); err != nil {
			return nil, err
		}
		if optUpdated {
			if options, err = contract.EncodeOptions(opt); err != nil {
				return nil, err
			}
		}
		return nil, contract.NewRequireSignatureError(h.signer.Hash(types.NewTx(p.TxData())).Bytes(), options)
	}

	tx, err := types.NewTx(p.TxData()).WithSignature(h.signer, opt.Signature)
	if err != nil {
		return nil, contract.ErrorCodeInvalidOption.Wrapf(err, "fail to WithSignature err:%s", err.Error())
	}
	if err = h.a.SendTransaction(context.Background(), tx); err != nil {
		return nil, errors.Wrapf(err, "fail to SendTransactions err:%s", err.Error())
	}
	return NewTxID(tx.Hash()), nil
}

type CallOption struct {
	From contract.Address
}

func (h *Handler) Call(method string, params contract.Params, options contract.Options) (contract.ReturnValue, error) {
	in, out, err := h.method(method, params, true)
	if err != nil {
		return nil, err
	}
	data, err := h.callData(out, params)
	if err != nil {
		return nil, err
	}
	p := ethereum.CallMsg{
		To:   &h.address,
		Data: data,
	}
	//Options convert
	opt := &CallOption{}
	if err = contract.DecodeOptions(options, opt); err != nil {
		return nil, err
	}
	//optional fields
	//common.IsHexAddress(string(opt.From))
	p.From = common.HexToAddress(string(opt.From))

	var bs []byte
	if bs, err = h.a.CallContract(context.Background(), p, nil); err != nil {
		return nil, errors.Wrapf(err, "fail to CallContract err:%s", err.Error())
	}
	if in.Output.TypeID == contract.TVoid {
		return nil, nil
	}
	var ret []interface{}
	if ret, err = h.out.Unpack(method, bs); err != nil {
		return nil, errors.Wrapf(err, "fail to Unpack err:%s", err.Error())
	}
	return decode(in.Output, ret[0])
}

func (h *Handler) EventFilter(name string, params contract.Params) (contract.EventFilter, error) {
	in, has := h.in.EventMap[name]
	if !has {
		return nil, contract.ErrorCodeNotFoundEvent.Errorf("not found event:%s in", name)
	}
	out, has := h.out.Events[name]
	if !has {
		return nil, contract.ErrorCodeNotFoundEvent.Errorf("not found event:%s out", name)
	}
	validParams, err := contract.ParamsOfWithSpec(in.InputMap, params)
	if err != nil {
		return nil, err
	}
	outIndexed := getIndexedArguments(out)
	hashedParams := make(map[string]Topic)
	for _, arg := range outIndexed {
		if arg.Indexed {
			if p, ok := validParams[arg.Name]; ok {
				t, err := NewTopic(p)
				if err != nil {
					return nil, contract.ErrorCodeInvalidParam.Wrapf(err, "fail to NewTopic param:%s err:%s", arg.Name, err.Error())
				}
				hashedParams[arg.Name] = t
			}
		}
	}
	return &EventFilter{
		in:           *in,
		out:          out,
		outIndexed:   outIndexed,
		address:      contract.Address(h.address.String()),
		params:       validParams,
		hashedParams: hashedParams,
	}, nil
}

func getIndexedArguments(spec abi.Event) abi.Arguments {
	var outIndexed abi.Arguments
	for _, arg := range spec.Inputs {
		if arg.Indexed {
			outIndexed = append(outIndexed, arg)
		}
	}
	return outIndexed
}

func (h *Handler) Spec() contract.Spec {
	return h.in
}

func (h *Handler) Address() contract.Address {
	return contract.Address(h.address.String())
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

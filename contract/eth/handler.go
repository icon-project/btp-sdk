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
	//FIXME not allowed method override
	//for _, m := range out.Methods {
	//
	//}
	//FIXME len(out.Methods.Outputs) > 0 throw error
	return &Handler{
		in:      NewSpec(out),
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
				return nil, nil, errors.Errorf("mismatch readonly, expected:%v", readonly)
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
		return nil, nil, errors.New("not found method")
	}
}

func (h *Handler) callData(m *abi.Method, params contract.Params) (b []byte, err error) {
	r := make([]interface{}, len(m.Inputs))
	for i, v := range m.Inputs {
		param, ok := params[v.Name]
		if !ok || param == nil {
			return nil, errors.New("required param " + v.Name)
		}
		if r[i], err = encode(v.Type, param); err != nil {
			return nil, err
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
		GasLimit: DefaultStepLimit,
	}
	if len(opt.Value) > 0 {
		if p.Value, err = opt.Value.AsBigInt(); err != nil {
			return nil, err
		}
	}
	if len(opt.GasLimit) > 0 {
		if p.GasLimit, err = opt.GasLimit.AsUint64(); err != nil {
			return nil, err
		}
	}
	if len(opt.Nonce) > 0 {
		if p.Nonce, err = opt.Nonce.AsUint64(); err != nil {
			return nil, err
		}
	}
	if len(opt.GasPrice) > 0 {
		if p.GasPrice, err = opt.GasPrice.AsBigInt(); err != nil {
			return nil, err
		}
	}
	if len(opt.GasFeeCap) > 0 {
		if p.GasFeeCap, err = opt.GasFeeCap.AsBigInt(); err != nil {
			return nil, err
		}
	}
	if len(opt.GasTipCap) > 0 {
		if p.GasTipCap, err = opt.GasTipCap.AsBigInt(); err != nil {
			return nil, err
		}
	}
	if p.GasPrice != nil && (p.GasFeeCap != nil || p.GasTipCap != nil) {
		return nil, errors.New("both GasPrice and (GasFeeCap or GasTipCap) specified")
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
			return false, errors.New("required 'from'")
		}
		//common.IsHexAddress(string(opt.From))
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
					return false, errors.Errorf(
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
		return nil, errors.Wrapf(err, "fail to WithSignature err:%s", err.Error())
	}
	if err = h.a.SendTransaction(context.Background(), tx); err != nil {
		return nil, errors.Wrapf(err, "fail to SendTransactions err:%s", err.Error())
	}
	return tx.Hash().Hex(), nil
}

func (h *Handler) GetResult(id contract.TxID) (contract.TxResult, error) {
	txh, ok := id.(string)
	if !ok {
		return nil, errors.Errorf("fail GetResult, invalid type %T", id)
	}
	txr, err := h.a.TransactionReceipt(context.Background(), common.HexToHash(txh))
	if err != nil {
		return nil, errors.Wrapf(err, "fail to TransactionReceipt err:%s", err.Error())
	}
	return NewTxResult(txr)
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
	var ret []interface{}
	if ret, err = h.out.Unpack(method, bs); err != nil {
		return nil, errors.Wrapf(err, "fail to Unpack err:%s", err.Error())
	}
	return decode(in.Output, ret[0])
}

func (h *Handler) EventFilter(name string, params contract.Params) (contract.EventFilter, error) {
	in, has := h.in.EventMap[name]
	if !has {
		return nil, errors.New("not found event from in")
	}
	out, has := h.out.Events[name]
	if !has {
		return nil, errors.New("not found event from out")
	}
	if err := contract.ParamsTypeCheck(in, params); err != nil {
		return nil, err
	}
	hashedParams := make(map[string]Topic)
	var outIndexed abi.Arguments
	for _, arg := range out.Inputs {
		if arg.Indexed {
			outIndexed = append(outIndexed, arg)
			if p, ok := params[arg.Name]; ok {
				t, err := NewTopic(p)
				if err != nil {
					return nil, err
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
		params:       params,
		hashedParams: hashedParams,
	}, nil
}

func (h *Handler) Spec() contract.Spec {
	return h.in
}

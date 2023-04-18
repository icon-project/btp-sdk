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
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/icon-project/btp-sdk/contract"

	ethLog "github.com/ethereum/go-ethereum/log"
)

const (
	NetworkType              = "eth"
	DefaultGetResultInterval = 2 * time.Second
)

var (
	DefaultStepLimit = uint64(8000000)
)

func init() {
	contract.RegisterAdaptorFactory(NetworkType, NewAdaptor)
}

type Adaptor struct {
	*ethclient.Client
	chainID *big.Int
	l       log.Logger
}

type AdaptorOption struct {
}

func NewAdaptor(endpoint string, options contract.Options, l log.Logger) (contract.Adaptor, error) {
	ethLog.Root().SetHandler(ethLog.FuncHandler(func(r *ethLog.Record) error {
		l.Log(log.Level(r.Lvl+1), r.Msg)
		return nil
	}))
	rpcClient, err := rpc.DialHTTPWithClient(endpoint, contract.NewHttpClient(l))
	//rpcClient, err := rpc.Dial(endpoint)
	if err != nil {
		return nil, err
	}
	a := &Adaptor{
		Client: ethclient.NewClient(rpcClient),
		l:      l,
	}
	if a.chainID, err = a.ChainID(context.Background()); err != nil {
		return nil, errors.Wrapf(err, "fail to ChainID err:%s", err.Error())
	}
	return a, nil
}

func (a *Adaptor) Handler(spec []byte, address contract.Address) (contract.Handler, error) {
	//common.IsHexAddress(string(address))
	addr := common.HexToAddress(string(address))
	return NewHandler(spec, addr, a, a.l)
}

func (a *Adaptor) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	return a.Client.SendTransaction(ctx, tx)
}

//
//func (c *Adaptor) Call(from, to string, method string, params interface{}, opt interface{}) (interface{}, error) {
//	//TODO implement me
//	panic("implement me")
//}
//
//func (c *Adaptor) NewRawTx(from, to string, method string, params interface{}, opt interface{}) ([]byte, error) {
//	tx := types.DynamicFeeTx{
//		ChainID:    nil,
//		Nonce:      0,   //option, generate
//		GasTipCap:  nil, //option
//		GasFeeCap:  nil, //option
//		Gas:        0,
//		To:         nil,
//		Value:      nil,
//		Data:       nil,
//		AccessList: nil,
//		V:          nil,
//		R:          nil,
//		S:          nil,
//	}
//	//TODO implement me
//	panic("implement me")
//}
//
//func (c *Adaptor) Send(signature, rawTx []byte, opt interface{}) (contract.TxID, error) {
//	p := types.Transaction{}
//	if err := json.Unmarshal(rawTx, &p); err != nil {
//		return nil, err
//	}
//	//types.NewTx()
//	return nil, nil
//}
//
//func (c *Adaptor) GetResult(id contract.TxID, opt interface{}) (contract.TxResult, error) {
//	//TODO implement me
//	panic("implement me")
//}

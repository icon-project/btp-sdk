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
	"encoding/json"
	"time"

	"github.com/icon-project/btp2/chain/icon/client"
	"github.com/icon-project/btp2/common/crypto"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/jsonrpc"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
)

const (
	NetworkType              = "icon"
	DefaultGetResultInterval = time.Second
)

var (
	DefaultVersion      = client.NewHexInt(client.JsonrpcApiVersion)
	DefaultStepLimit    = client.NewHexInt(2500000000) //client.HexInt("0x9502f900")
	txSerializeExcludes = map[string]bool{"signature": true}
)

func init() {
	contract.RegisterAdaptorFactory(NetworkType, NewAdaptor)
}

type Adaptor struct {
	*client.Client
	opt AdaptorOption
	l   log.Logger
}

type AdaptorOption struct {
	NetworkID client.HexInt `json:"nid"`
}

func NewAdaptor(endpoint string, options contract.Options, l log.Logger) (contract.Adaptor, error) {
	c := client.NewClient(endpoint, l)
	c.Client = jsonrpc.NewJsonRpcClient(contract.NewHttpClient(l), endpoint)
	a := &Adaptor{
		Client: c,
		l:      l,
	}
	if err := contract.DecodeOptions(options, &a.opt); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *Adaptor) Handler(spec []byte, address contract.Address) (contract.Handler, error) {
	return NewHandler(spec, client.Address(address), a, a.l)
}

func (a *Adaptor) GetTransactionResult(p *client.TransactionHashParam) (*client.TransactionResult, error) {
	for {
		txr, err := a.Client.GetTransactionResult(p)
		if err != nil {
			if je, ok := err.(*jsonrpc.Error); ok {
				switch je.Code {
				case client.JsonrpcErrorCodePending, client.JsonrpcErrorCodeExecuting:
					<-time.After(DefaultGetResultInterval)
					continue
				}
			}
			return nil, err
		}
		return txr, nil
	}
}

func (a *Adaptor) NewTransactionParam(to client.Address, data *client.CallData) *client.TransactionParam {
	return &client.TransactionParam{
		Version:   DefaultVersion,
		NetworkID: a.opt.NetworkID,
		ToAddress: to,
		DataType:  "call",
		Data:      data,
		StepLimit: DefaultStepLimit,
	}
}

func (a *Adaptor) HashForSignature(p *client.TransactionParam) ([]byte, error) {
	js, err := json.Marshal(p)
	if err != nil {
		return nil, errors.Wrapf(err, "fail to HashForSignature marshal err:%s", err.Error())
	}
	var b []byte
	b, err = client.SerializeJSON(js, nil, txSerializeExcludes)
	if err != nil {
		return nil, errors.Wrapf(err, "fail to HashForSignature serialize err:%s", err.Error())
	}
	b = append([]byte("icx_sendTransaction."), b...)
	return crypto.SHA3Sum256(b), nil
}

type HexBool string

func (b HexBool) Value() (bool, error) {
	if b == "0x1" {
		return true, nil
	} else if b == "0x0" {
		return false, nil
	} else {
		return false, errors.Errorf("invalid value %s", b)
	}
}

func NewHexBool(v bool) HexBool {
	if v {
		return HexBool("0x1")
	} else {
		return HexBool("0x0")
	}
}

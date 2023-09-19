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
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
	"github.com/icon-project/btp2/chain/icon/client"
	"github.com/icon-project/btp2/common/crypto"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/jsonrpc"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
)

const (
	NetworkTypeIcon          = "icon"
	DefaultGetResultInterval = time.Second
)

var (
	DefaultVersion          = client.NewHexInt(client.JsonrpcApiVersion)
	DefaultStepLimit        = client.NewHexInt(2500000000) //client.HexInt("0x9502f900")
	txSerializeExcludes     = map[string]bool{"signature": true}
	DefaultProgressInterval = client.NewHexInt(10)
	NetworkTypes            = []string{
		NetworkTypeIcon,
	}
)

func init() {
	contract.RegisterAdaptorFactory(NewAdaptor, NetworkTypes...)
}

type Adaptor struct {
	*client.Client
	fm          contract.FinalityMonitor
	networkType string
	opt         AdaptorOption
	l           log.Logger
}

type AdaptorOption struct {
	NetworkID         client.HexInt     `json:"nid"`
	TransportLogLevel contract.LogLevel `json:"transport_log_level,omitempty"`
	FinalityMonitor   contract.Options  `json:"finality_monitor"`
}

func NewAdaptor(networkType string, endpoint string, options contract.Options, l log.Logger) (contract.Adaptor, error) {
	opt := &AdaptorOption{}
	if err := contract.DecodeOptions(options, &opt); err != nil {
		return nil, err
	}
	opt.TransportLogLevel = contract.LogLevel(contract.EnsureTransportLogLevel(opt.TransportLogLevel.Level()))
	c := client.NewClient(endpoint, l)
	c.Client = jsonrpc.NewJsonRpcClient(contract.NewHttpClient(opt.TransportLogLevel.Level(), l), endpoint)
	var (
		fm  contract.FinalityMonitor
		err error
	)
	switch networkType {
	case NetworkTypeIcon:
		if fm, err = NewFinalityMonitor(opt.FinalityMonitor, c, l); err != nil {
			return nil, err
		}
	default:
		return nil, errors.Errorf("not supported networkType:%s", networkType)
	}
	return &Adaptor{
		Client:      c,
		fm:          fm,
		networkType: networkType,
		opt:         *opt,
		l:           l,
	}, nil
}

func (a *Adaptor) NetworkType() string {
	return a.networkType
}

func (a *Adaptor) SendTransaction(p *client.TransactionParam) (HexBytes, error) {
	var result HexBytes
	if _, err := a.Client.Do("icx_sendTransaction", p, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (a *Adaptor) GetResult(id contract.TxID) (contract.TxResult, error) {
	txh, err := contract.BytesOf(id)
	if err != nil {
		return nil, errors.Wrapf(err, "fail to BytesOf, invalid id err:%s", err.Error())
	}
	p := &client.TransactionHashParam{Hash: client.NewHexBytes(txh)}
	txr, err := a.GetTransactionResult(p)
	if err != nil {
		return nil, errors.Wrapf(err, "fail to GetTransactionResult err:%s", err.Error())
	}
	txBlkHeight, _ := txr.BlockHeight.Value()
	bp := &client.BlockHeightParam{
		Height: client.NewHexInt(txBlkHeight + 1),
	}
	blk, _ := a.GetBlockByHeight(bp)
	bh, _ := hex.DecodeString(blk.BlockHash)
	var txf *TxFailure
	if !IsSuccess(txr) {
		if txf, err = a.TransactionFailureReason(id); err != nil {
			//FIXME if TxFailure is not mandatory, ignore err
			return nil, errors.Wrapf(err, "fail to TransactionFailureReason err:%s", err.Error())
		}
	}
	return NewTxResult(txr, blk.Height, bh, txf)
}

func (a *Adaptor) TransactionFailureReason(id contract.TxID) (*TxFailure, error) {
	txh, err := contract.BytesOf(id)
	if err != nil {
		return nil, errors.Wrapf(err, "fail to BytesOf, invalid id err:%s", err.Error())
	}
	p := &client.TransactionHashParam{Hash: client.NewHexBytes(txh)}
	tp := &client.TransactionParamForEstimate{}
	if _, err = a.Client.Do("icx_getTransactionByHash", p, tp); err != nil {
		return nil, err
	}
	_, err = a.Client.EstimateStep(tp)
	f := NewTxFailure(err)
	if f == nil {
		return nil, err
	}
	return f, nil
}

func (a *Adaptor) Handler(spec []byte, address contract.Address) (contract.Handler, error) {
	return NewHandler(spec, client.Address(address), a, a.l)
}

func (a *Adaptor) FinalityMonitor() contract.FinalityMonitor {
	return a.fm
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

func (a *Adaptor) EstimateStep(p *client.TransactionParamForEstimate) (int64, error) {
	r, err := a.Client.EstimateStep(p)
	if err != nil {
		txf := NewTxFailure(err)
		if txf == nil {
			return r, err
		}
		return r, txf
	}
	return r, nil
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

func (a *Adaptor) GetLastBlock() (*client.Block, error) {
	result := &client.Block{}
	if _, err := a.Client.Do("icx_getLastBlock", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

type Block struct {
	client.Block
	BlockHash string `json:"block_hash"`
}

func (a *Adaptor) GetBlockByHeight(p *client.BlockHeightParam) (*Block, error) {
	result := &Block{}
	if _, err := a.Client.Do("icx_getBlockByHeight", p, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func newEventFilters(sigToAddrs map[string][]contract.Address) []*client.EventFilter {
	efs := make([]*client.EventFilter, 0)
	for s, addrs := range sigToAddrs {
		if len(addrs) == 0 {
			ef := &client.EventFilter{
				Signature: s,
			}
			efs = append(efs, ef)
		} else {
			for _, addr := range addrs {
				ef := &client.EventFilter{
					Addr:      client.Address(addr),
					Signature: s,
				}
				efs = append(efs, ef)
			}
		}
	}
	return efs
}

func (a *Adaptor) MonitorEvent(
	ctx context.Context,
	cb contract.EventCallback,
	efs []contract.EventFilter,
	height int64) error {
	if len(efs) == 0 {
		return errors.New("EventFilter required")
	}
	for i, f := range efs {
		if _, ok := f.(*EventFilter); !ok {
			return errors.Errorf("not support EventFilter idx:%d %T", i, f)
		}
	}
	reqHeight, err := a.monitorHeight(height)
	if err != nil {
		return err
	}
	req := &EventRequest{
		EventRequest: client.EventRequest{
			Height: reqHeight,
		},
		Logs:             NewHexBool(true),
		ProgressInterval: DefaultProgressInterval,
		Filters:          newEventFilters(contract.NewSignatureToAddressesMap(efs)),
	}
	resp := &ProgressOrEventNotification{}
	es := make([]*Event, 0)
	bp := &client.BlockHeightParam{}
	return a.Client.MonitorWithContext(ctx, "/event", req, resp, func(conn *websocket.Conn, v interface{}) {
		switch n := v.(type) {
		case *ProgressOrEventNotification:
			if n.ProgressNotification != nil {
				a.l.Logf(a.opt.TransportLogLevel.Level(), "ProgressNotification:%+v", n.ProgressNotification)
				if len(es) > 0 {
					h, _ := n.ProgressNotification.Progress.Value()
					h = h - 1
					bp.Height = client.NewHexInt(h)
					blk, _ := a.GetBlockByHeight(bp)
					for _, e := range es {
						e.txHash, _ = blk.NormalTransactions[e.txIndex].TxHash.Value()
						if cb(e) != nil {
							conn.Close()
						}
					}
					es = es[:0]
				}
			} else if n.EventNotification != nil {
				a.l.Logf(a.opt.TransportLogLevel.Level(), "EventNotification:%+v", n.EventNotification)
				h, _ := n.EventNotification.Height.Value()
				bh, _ := n.EventNotification.Hash.Value()
				index, _ := n.EventNotification.Index.Int()
				for i, el := range n.EventNotification.Logs {
					indexInTx, _ := n.EventNotification.Events[i].Int()
					be := NewBaseEvent(el, h, bh, nil, index, indexInTx)
					for _, f := range efs {
						if e, _ := f.Filter(be); e != nil {
							es = append(es, e.(*Event))
							continue
						}
					}
				}
			} else {
				a.l.Warnf("empty notification %v", n)
			}
		case client.WSEvent:
			a.l.Debugf("monitorEvent connected conn:%s", conn.LocalAddr().String())
		case error:
			a.l.Warnf("err:%+v", n)
		default:
			a.l.Warnf("err:%+v", errors.Errorf("not supported type %T", n))
		}
	})
}

func (a *Adaptor) MonitorBaseEvent(
	ctx context.Context,
	cb contract.BaseEventCallback,
	sigToAddrs map[string][]contract.Address,
	height int64) error {
	reqHeight, err := a.monitorHeight(height)
	if err != nil {
		return err
	}
	req := &BlockRequest{
		BlockRequest: client.BlockRequest{
			Height:       reqHeight,
			EventFilters: newEventFilters(sigToAddrs),
		},
		Logs: NewHexBool(true),
	}
	resp := &BlockNotification{}
	return a.Client.MonitorWithContext(ctx, "/block", req, resp, func(conn *websocket.Conn, v interface{}) {
		switch n := v.(type) {
		case *BlockNotification:
			a.l.Logf(a.opt.TransportLogLevel.Level(), "BlockNotification:%+v", n)
			h, _ := n.Height.Value()
			bh, _ := n.Hash.Value()
			if len(n.Logs) > 0 {
				blk, _ := a.GetBlockByHeight(&client.BlockHeightParam{Height: client.NewHexInt(h - 1)})
				for i, logsEachFilter := range n.Logs {
					for j, logsOfReceipt := range logsEachFilter {
						indexInBlock, _ := n.Indexes[i][j].Int()
						txh, _ := blk.NormalTransactions[indexInBlock].TxHash.Value()
						for k, el := range logsOfReceipt {
							indexInTx, _ := n.Events[i][j][k].Int()
							be := NewBaseEvent(el, h, bh, txh, indexInBlock, indexInTx)
							if cb(be) != nil {
								conn.Close()
							}
						}
					}
				}
			}
		case client.WSEvent:
			a.l.Debugf("monitorBlock connected conn:%s", conn.LocalAddr().String())
		case error:
			a.l.Infof("err:%+v", n)
		default:
			a.l.Infof("err:%+v", errors.Errorf("not supported type %T", n))
		}
	})
}

func (a *Adaptor) monitorHeight(height int64) (client.HexInt, error) {
	if height == 0 {
		blk, err := a.GetLastBlock()
		if err != nil {
			return "", err
		}
		height = blk.Height
	}
	a.l.Debugf("monitorHeight height:%v", height)
	return client.NewHexInt(height), nil
}

type BlockRequest struct {
	client.BlockRequest
	Logs HexBool `json:"logs,omitempty"`
}

type BlockNotification struct {
	client.BlockNotification
	Logs [][][]struct {
		Addr    client.Address `json:"scoreAddress"`
		Indexed []string       `json:"indexed"`
		Data    []string       `json:"data"`
	} `json:"logs,omitempty"`
}

type EventRequest struct {
	client.EventRequest
	Logs             HexBool       `json:"logs,omitempty"`
	ProgressInterval client.HexInt `json:"progressInterval,omitempty"`

	Filters []*client.EventFilter `json:"eventFilters,omitempty"`
}

type ProgressOrEventNotification struct {
	*ProgressNotification
	*EventNotification
}

type ProgressNotification struct {
	Progress client.HexInt `json:"progress"`
}

type EventNotification struct {
	client.EventNotification
	Logs []struct {
		Addr    client.Address `json:"scoreAddress"`
		Indexed []string       `json:"indexed"`
		Data    []string       `json:"data"`
	} `json:"logs,omitempty"`
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
		return "0x1"
	} else {
		return "0x0"
	}
}

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
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	ethLog "github.com/ethereum/go-ethereum/log"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/icon-project/btp-sdk/contract"
)

const (
	NetworkTypeEth           = "eth"
	NetworkTypeEth2          = "eth2"
	DefaultGetResultInterval = 2 * time.Second
)

var (
	DefaultGasLimit = uint64(8000000)
	NetworkTypes    = []string{
		NetworkTypeEth,
		NetworkTypeEth2,
	}
)

func init() {
	contract.RegisterAdaptorFactory(NewAdaptor, NetworkTypes...)
}

type Adaptor struct {
	*ethclient.Client
	bm          contract.BlockMonitor
	chainID     *big.Int
	networkType string
	opt         AdaptorOption
	l           log.Logger
}

type AdaptorOption struct {
	BlockMonitor      contract.Options  `json:"block-monitor"`
	TransportLogLevel contract.LogLevel `json:"transport-log-level,omitempty"`
}

func NewAdaptor(networkType string, endpoint string, options contract.Options, l log.Logger) (contract.Adaptor, error) {
	opt := &AdaptorOption{}
	if err := contract.DecodeOptions(options, &opt); err != nil {
		return nil, err
	}
	opt.TransportLogLevel = contract.LogLevel(contract.EnsureTransportLogLevel(opt.TransportLogLevel.Level()))
	ethLog.Root().SetHandler(ethLog.FuncHandler(func(r *ethLog.Record) error {
		l.Log(log.Level(r.Lvl+1), r.Msg)
		return nil
	}))
	rc, err := rpc.DialOptions(
		context.Background(),
		endpoint,
		rpc.WithHTTPClient(contract.NewHttpClient(opt.TransportLogLevel.Level(), l)))
	if err != nil {
		return nil, err
	}
	c := ethclient.NewClient(rc)
	chainID, err := c.ChainID(context.Background())
	if err != nil {
		return nil, errors.Wrapf(err, "fail to ChainID err:%s", err.Error())
	}

	var bm contract.BlockMonitor
	switch networkType {
	case NetworkTypeEth:
		if bm, err = NewBlockMonitor(opt.BlockMonitor, c, l); err != nil {
			return nil, err
		}
	case NetworkTypeEth2:
		if bm, err = NewEth2BlockMonitor(opt.BlockMonitor, l); err != nil {
			return nil, err
		}
	default:
		return nil, errors.Errorf("not supported networkType:%s", networkType)
	}
	return &Adaptor{
		Client:      c,
		bm:          bm,
		chainID:     chainID,
		networkType: networkType,
		opt:         *opt,
		l:           l,
	}, nil
}

func (a *Adaptor) Handler(spec []byte, address contract.Address) (contract.Handler, error) {
	//common.IsHexAddress(string(address))
	addr := common.HexToAddress(string(address))
	return NewHandler(spec, addr, a, a.l)
}

func (a *Adaptor) BlockMonitor() contract.BlockMonitor {
	return a.bm
}

func (a *Adaptor) TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	for {
		_, pending, err := a.Client.TransactionByHash(ctx, txHash)
		if err != nil {
			return nil, err
		}
		if pending {
			<-time.After(DefaultGetResultInterval)
			continue
		}
		return a.Client.TransactionReceipt(ctx, txHash)
	}
}

func (a *Adaptor) MonitorEvent(
	cb contract.BaseEventCallback,
	signature string,
	address contract.Address,
	height int64) error {
	fq := &ethereum.FilterQuery{
		Topics: [][]common.Hash{
			{crypto.Keccak256Hash([]byte(signature))},
		},
		Addresses: []common.Address{common.HexToAddress(string(address))},
		FromBlock: big.NewInt(height),
	}
	filterLogsByHeader := func(bh *types.Header) error {
		blkHash := bh.Hash()
		fq.BlockHash = &blkHash
		logs, err := a.Client.FilterLogs(context.Background(), *fq)
		if err != nil {
			return err
		}
		if len(logs) > 0 {
			for _, el := range logs {
				if e := matchAndToBaseEvent(fq, el); e != nil {
					cb(e)
				}
			}
		}
		return nil
	}
	var (
		h *big.Int
	)
	onBlockHeader := func(bh *types.Header) error {
		if h == nil {
			fq.FromBlock = nil
			h = big.NewInt(height)
			for ; h.Cmp(bh.Number) < 0; h = h.Add(h, common.Big1) {
				if tbh, err := a.Client.HeaderByNumber(context.Background(), h); err != nil {
					a.l.Errorf("failure HeaderByNumber(%v) err:%+v", h, err)
					return err
				} else if err = filterLogsByHeader(tbh); err != nil {
					return err
				}
			}
		}
		return filterLogsByHeader(bh)
	}
	if err := a.MonitorBySubscribeFilterLogs(cb, fq); err != nil {
		if err == rpc.ErrNotificationsUnsupported {
			a.l.Debugf("fail to MonitorBySubscribeFilterLogs, try MonitorByPollHead")
			return monitorByPollHead(a.Client, a.l, context.Background(), onBlockHeader)
		}
		return err
	}
	return nil
}

func monitorByPollHead(c *ethclient.Client, l log.Logger, ctx context.Context, cb func(bh *types.Header) error) error {
	n, err := c.BlockNumber(ctx)
	if err != nil {
		return err
	}
	current := new(big.Int).SetUint64(n)
	for {
		select {
		case <-ctx.Done():
			l.Debugf("MonitorByPollHead context done")
			return ctx.Err()
		default:
		}
		var bh *types.Header
		if bh, err = c.HeaderByNumber(ctx, current); err != nil {
			if ethereum.NotFound == err {
				l.Trace("Block not ready, will retry ", current)
			} else {
				l.Warn("Unable to get block ", current, err)
			}
			<-time.After(DefaultPollHeadInterval)
			continue
		}

		if err = cb(bh); err != nil {
			l.Errorf("Poll callback return err:%+v", err)
			return err
		}
		current.Add(current, big.NewInt(1))
	}
}

func (a *Adaptor) MonitorEvents(
	cb contract.BaseEventsCallback,
	filter map[string][]contract.Address,
	height int64) error {
	fl := make(map[common.Hash][]common.Address)
	for signature, addresses := range filter {
		al := make([]common.Address, len(addresses))
		for i, address := range addresses {
			al[i] = common.HexToAddress(string(address))
		}
		sh := crypto.Keccak256Hash([]byte(signature))
		fl[sh] = al
	}
	var current *big.Int
	if height == 0 {
		n, err := a.Client.BlockNumber(context.Background())
		if err != nil {
			return err
		}
		current = new(big.Int).SetUint64(n)
	} else {
		current = new(big.Int).SetUint64(uint64(height))
	}
	for {
		blk, err := a.Client.BlockByNumber(context.Background(), current)
		if err != nil {
			if ethereum.NotFound == err {
				a.l.Trace("Block not ready, will retry ", current)
			} else {
				a.l.Error("Unable to get block ", current, err)
			}
			<-time.After(DefaultGetResultInterval)
			continue
		}
		has := false
		for k := range fl {
			if blk.Bloom().Test(k.Bytes()) {
				has = true
				break
			}
		}
		if has {
			l := make([]contract.BaseEvent, 0)
			for _, tx := range blk.Transactions() {
				var txr *types.Receipt
				txr, err = a.Client.TransactionReceipt(context.Background(), tx.Hash())
				for indexInTx, el := range txr.Logs {
					for k, v := range fl {
						if matchEventLog(k, v, el) {
							e := &BaseEvent{
								indexInTx:  indexInTx,
								Log:        el,
								sigMatcher: SignatureMatcher(el.Topics[0].String()),
								indexed:    len(el.Topics) - 1,
							}
							l = append(l, e)
						}
					}
				}
			}
			if len(l) > 0 {
				cb(l)
			}
		}
		current.Add(current, big.NewInt(1))
	}
}

func (a *Adaptor) MonitorBySubscribeFilterLogs(cb func(e contract.BaseEvent),
	fq *ethereum.FilterQuery) error {
	ch := make(chan types.Log)
	s, err := a.Client.SubscribeFilterLogs(context.Background(), *fq, ch)
	if err != nil {
		return err
	}
	for {
		select {
		case err = <-s.Err():
			return err
		case el := <-ch:
			if e := matchAndToBaseEvent(fq, el); e != nil {
				cb(e)
			}
		}
	}
}

func matchAndToBaseEvent(fq *ethereum.FilterQuery, el types.Log) *BaseEvent {
	if matchEventLog(fq.Topics[0][0], fq.Addresses, &el) {
		return &BaseEvent{
			//indexInTx: indexInTx, //FIXME indexInTx
			Log:        &el,
			sigMatcher: SignatureMatcher(el.Topics[0].String()),
			indexed:    len(el.Topics) - 1,
			//finalized: //FIXME finalized
		}
	}
	return nil
}

func matchEventLog(signature common.Hash, addresses []common.Address, el *types.Log) bool {
	if !bytes.Equal(el.Topics[0].Bytes(), signature.Bytes()) {
		return false
	}
	if len(addresses) > 0 {
		ab := el.Address.Bytes()
		addressMatched := false
		for _, address := range addresses {
			if bytes.Equal(ab, address.Bytes()) {
				addressMatched = true
				break
			}
		}
		if !addressMatched {
			return false
		}
	}
	return true
}

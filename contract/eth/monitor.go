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
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
)

var (
	rpcFinalizedBlockNumber = big.NewInt(int64(rpc.FinalizedBlockNumber))
)

type BlockInfo struct {
	id     common.Hash
	height int64
}

func (b *BlockInfo) ID() contract.BlockID {
	return NewBlockID(b.id)
}

func (b *BlockInfo) Height() int64 {
	return b.height
}

func (b *BlockInfo) String() string {
	return fmt.Sprintf("BlockInfo{ID:%s,Height:%d}",
		hex.EncodeToString(b.id[:]), b.height)
}

func (b *BlockInfo) EqualID(id contract.BlockID) (bool, error) {
	h, err := CommonHashOf(id)
	if err != nil {
		return false, errors.Wrapf(err, "invalid BlockID err:%v", err.Error())
	}
	return bytes.Equal(b.id.Bytes(), h.Bytes()), nil
}

type BlockInfoJson struct {
	ID     common.Hash
	Height int64
}

func (b *BlockInfo) UnmarshalJSON(bytes []byte) error {
	v := &BlockInfoJson{}
	if err := json.Unmarshal(bytes, v); err != nil {
		return err
	}
	b.id = v.ID
	b.height = v.Height
	return nil
}

func (b *BlockInfo) MarshalJSON() ([]byte, error) {
	v := BlockInfoJson{
		ID:     b.id,
		Height: b.height,
	}
	return json.Marshal(v)
}

type FinalityMonitorOptions struct {
	contract.DefaultFinalityMonitorOptions
	PollingPeriodSec uint `json:"polling_period_sec"`
}

type FinalitySupplier struct {
	*ethclient.Client
	opt FinalityMonitorOptions
	l   log.Logger
}

func NewFinalitySupplier(options contract.Options, c *ethclient.Client, l log.Logger) (*FinalitySupplier, error) {
	opt := FinalityMonitorOptions{}
	if err := contract.DecodeOptions(options, &opt); err != nil {
		return nil, err
	}
	return &FinalitySupplier{
		Client: c,
		opt:    opt,
		l:      l,
	}, nil
}

func (f *FinalitySupplier) Latest() (contract.BlockInfo, error) {
	bh, err := f.HeaderByNumber(context.Background(), rpcFinalizedBlockNumber)
	if err != nil {
		return nil, err
	}
	return &BlockInfo{
		id:     bh.Hash(),
		height: bh.Number.Int64(),
	}, nil
}

func (f *FinalitySupplier) HeightByID(id contract.BlockID) (int64, error) {
	h, err := CommonHashOf(id)
	if err != nil {
		return 0, errors.Wrapf(err, "invalid BlockID err:%v", err.Error())
	}
	bh, err := f.Client.HeaderByHash(context.Background(), h)
	if err != nil {
		if err == ethereum.NotFound {
			return 0, contract.ErrorCodeNotFoundBlock.Errorf("not found block:%s", id)
		}
		return 0, err
	}
	return bh.Number.Int64(), nil
}

func (f *FinalitySupplier) Serve(ctx context.Context, last contract.BlockInfo, cb func(contract.BlockInfo)) error {
	height := last.Height()
	period := time.Duration(f.opt.PollingPeriodSec) * time.Second
	for {
		select {
		case <-ctx.Done():
			f.l.Debugf("context done")
			return ctx.Err()
		default:
		}
		bi, err := f.Latest()
		if err != nil {
			return err
		}
		if height < bi.Height() {
			cb(bi)
			height = bi.Height()
		}
		<-time.After(period)
	}
}

func NewFinalityMonitor(options contract.Options, c *ethclient.Client, l log.Logger) (contract.FinalityMonitor, error) {
	fs, err := NewFinalitySupplier(options, c, l)
	if err != nil {
		return nil, err
	}
	return contract.NewDefaultFinalityMonitor(options, fs, l)
}

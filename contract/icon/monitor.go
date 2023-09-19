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
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/gorilla/websocket"
	"github.com/icon-project/btp2/chain/icon/client"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
)

type BlockInfo struct {
	id     HexBytes
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
		b.id.String(), b.height)
}

func (b *BlockInfo) EqualID(id contract.BlockID) (bool, error) {
	hb, err := HexBytesOf(id)
	if err != nil {
		return false, errors.Wrapf(err, "invalid BlockID err:%v", err.Error())
	}
	return bytes.Equal(b.id, hb), nil
}

type BlockInfoJson struct {
	ID     HexBytes
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

type FinalitySupplier struct {
	*client.Client
	l log.Logger
}

func NewFinalitySupplier(c *client.Client, l log.Logger) *FinalitySupplier {
	return &FinalitySupplier{
		Client: c,
		l:      l,
	}
}

func (f *FinalitySupplier) Latest() (contract.BlockInfo, error) {
	blk, err := f.Client.GetLastBlock()
	if err != nil {
		return nil, err
	}
	bi := &BlockInfo{
		height: blk.Height,
	}
	if bi.id, err = blk.BlockHash.Value(); err != nil {
		return nil, err
	}
	return bi, nil
}

func (f *FinalitySupplier) HeightByID(id contract.BlockID) (int64, error) {
	hb, err := HexBytesOf(id)
	if err != nil {
		return 0, errors.Wrapf(err, "invalid BlockID err:%v", err.Error())
	}
	p := &client.BlockHashParam{
		Hash: client.NewHexBytes(hb),
	}
	blk, err := f.Client.GetBlockByHash(p)
	if err != nil {
		return 0, err
	}
	return blk.Height, nil
}

func (f *FinalitySupplier) Serve(ctx context.Context, last contract.BlockInfo, cb func(contract.BlockInfo)) error {
	var height int64
	if last != nil {
		height = last.Height()
	}
	req := &client.BlockRequest{
		Height: client.NewHexInt(height),
	}
	resp := &BlockNotification{}
	return f.Client.MonitorWithContext(ctx, "/block", req, resp, func(conn *websocket.Conn, v interface{}) {
		switch n := v.(type) {
		case *BlockNotification:
			id, _ := n.Hash.Value()
			height, _ := n.Height.Value()
			bi := &BlockInfo{
				id:     id,
				height: height,
			}
			cb(bi)
		case client.WSEvent:
			f.l.Debugf("monitorBlock connected conn:%s", conn.LocalAddr().String())
		case error:
			f.l.Infof("err:%+v", n)
		default:
			f.l.Infof("err:%+v", errors.Errorf("not supported type %T", n))
		}
	})
}

func NewFinalityMonitor(options contract.Options, c *client.Client, l log.Logger) (contract.FinalityMonitor, error) {
	return contract.NewDefaultFinalityMonitor(options, NewFinalitySupplier(c, l), l)
}

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
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/icon-project/btp2/common/log"
	"github.com/icon-project/btp2/common/types"
	"github.com/icon-project/btp2/common/wallet"
	"github.com/stretchr/testify/assert"

	"github.com/icon-project/btp-sdk/contract"
)

const (
	endpoint       = "http://localhost:8545"
	addr           = "0x09635F643e140090A9A8Dcd712eD6285858ceBef"
	keystoreFile   = "../../example/solidity/test/keystore.json"
	keystoreSecret = "../../example/solidity/test/keysecret"
)

var (
	adaptorOpts = map[string]AdaptorOption{
		NetworkTypeEth: {
			FinalityMonitor: MustEncodeOptions(FinalityMonitorOptions{
				PollingPeriodSec: 2,
			}),
		},
		NetworkTypeEth2: {
			FinalityMonitor: MustEncodeOptions(FinalityMonitorOptions{
				PollingPeriodSec: 3,
			}),
		},
		NetworkTypeBSC: {
			FinalityMonitor: MustEncodeOptions(FinalityMonitorOptions{
				PollingPeriodSec: 3,
			}),
		},
	}
	w = MustLoadWallet(keystoreFile, keystoreSecret)
)

func MustEncodeOptions(v interface{}) contract.Options {
	opt, err := contract.EncodeOptions(v)
	if err != nil {
		log.Panicf("%+v", err)
	}
	return opt
}

func MustLoadWallet(keyStoreFile, keyStoreSecret string) types.Wallet {
	w, err := wallet.DecryptKeyStore(MustReadFile(keyStoreFile), MustReadFile(keyStoreSecret))
	if err != nil {
		log.Panicf("keyStoreFile:%s, keyStoreSecret:%s, %+v",
			keyStoreFile, keyStoreSecret, err)
	}
	return w
}

func MustReadFile(f string) []byte {
	b, err := os.ReadFile(f)
	if err != nil {
		log.Panicf("fail to ReadFile err:%+v", err)
	}
	return b
}

func adaptor(t *testing.T, networkType string) *Adaptor {
	l := log.GlobalLogger()
	opt, err := contract.EncodeOptions(adaptorOpts[networkType])
	if err != nil {
		assert.FailNow(t, "fail to EncodeOptions", err)
	}
	a, err := contract.NewAdaptor(networkType, endpoint, opt, l)
	if err != nil {
		assert.FailNow(t, "fail to NewAdaptor", err)
	}
	return a.(*Adaptor)
}

func Test_MonitorEvents(t *testing.T) {
	a := adaptor(t, NetworkTypeEth2)
	name := "hello"
	arg := abi.Argument{
		Name: "name",
	}
	arg.Type, _ = abi.NewType("string", "", nil)
	args := abi.Arguments{arg}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		err := a.MonitorBaseEvent(ctx, func(be contract.BaseEvent) error {
			t.Logf("%+v", be)
			if vl, err := args.UnpackValues(be.(*BaseEvent).Data); err != nil {
				assert.NoError(t, err, "fail to UnpackValues")
				return err
			} else {
				t.Logf("paramValue:%+v", vl[0])
				return nil
			}
		}, map[string][]contract.Address{
			"HelloEvent(string)": {addr},
		},
			0)
		assert.Equal(t, ctx.Err(), err)
	}()

	b, err := args.Pack(name)
	if err != nil {
		assert.FailNow(t, "fail to Pack", err)
	}
	sig := fmt.Sprintf("%v(%v)", "setName", arg.Type.String())
	id := crypto.Keccak256([]byte(sig))[:4]
	to := common.HexToAddress(addr)
	p := &ethTypes.LegacyTx{
		Gas:  DefaultGasLimit,
		To:   &to,
		Data: append(id, b...),
	}
	p.Nonce, err = a.PendingNonceAt(context.Background(), common.HexToAddress(w.Address()))
	if err != nil {
		assert.FailNow(t, "fail to PendingNonceAt", err)
	}
	p.GasPrice, err = a.SuggestGasPrice(context.Background())
	if err != nil {
		assert.FailNow(t, "fail to SuggestGasPrice", err)
	}

	tx := ethTypes.NewTx(p)
	signer := ethTypes.LatestSignerForChainID(a.chainID)
	b, err = w.Sign(signer.Hash(tx).Bytes())
	if err != nil {
		assert.FailNow(t, "fail to Sign", err)
	}
	tx, err = tx.WithSignature(signer, b)
	if err != nil {
		assert.FailNow(t, "fail to WithSignature", err)
	}

	err = a.SendTransaction(context.Background(), tx)
	if err != nil {
		assert.FailNow(t, "fail to SendTransaction", err)
	}

	txr, err := a.TransactionReceipt(context.Background(), tx.Hash())
	assert.NoError(t, err)
	t.Logf("%+v", txr)
	<-time.After(5 * time.Second)
	cancel()
}

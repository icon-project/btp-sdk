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
	"crypto/ecdsa"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"
	"github.com/icon-project/btp2/common/wallet"
	"github.com/stretchr/testify/assert"

	"github.com/icon-project/btp-sdk/contract"
)

const (
	endpoint = "http://localhost:8545"
	//cd ../../example/solidity; npx hardhat --network ether2_test run ./scripts/deploy.ts
	addr                              = "0x09635F643e140090A9A8Dcd712eD6285858ceBef"
	keystoreFile                      = "../../example/solidity/test/keystore.json"
	keystoreSecret                    = "../../example/solidity/test/keysecret"
	ethBlockMonitorFinalizeBlockCount = 3
	eth2BlockMonitorEndpoint          = "http://localhost:9596"
)

var (
	adaptorOpts = map[string]AdaptorOption{
		NetworkTypeEth: {
			BlockMonitor: MustEncodeOptions(BlockMonitorOptions{
				FinalizeBlockCount: ethBlockMonitorFinalizeBlockCount,
			}),
		},
		NetworkTypeEth2: {
			BlockMonitor: MustEncodeOptions(Eth2BlockMonitorOptions{
				Endpoint: eth2BlockMonitorEndpoint,
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

type Wallet struct {
	*keystore.Key
	PubKey *ecdsa.PublicKey
}

func (w *Wallet) Address() string {
	return w.Key.Address.String()
}

func (w *Wallet) Sign(data []byte) ([]byte, error) {
	return crypto.Sign(data, w.PrivateKey)
}

func (w *Wallet) PublicKey() []byte {
	return crypto.FromECDSAPub(w.PubKey)
}

func (w *Wallet) ECDH(pubKey []byte) ([]byte, error) {
	return nil, errors.New("not implemented")
}

func MustLoadWallet(keyStoreFile, keyStoreSecret string) wallet.Wallet {
	key, err := keystore.DecryptKey(MustReadFile(keyStoreFile), string(MustReadFile(keyStoreSecret)))
	if err != nil {
		log.Panicf("keyStoreFile:%s, keyStoreSecret:%s, %+v",
			keyStoreFile, keyStoreSecret, err)
	}
	return &Wallet{
		Key:    key,
		PubKey: &key.PrivateKey.PublicKey,
	}
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

	go func() {
		err := a.MonitorBaseEvent(func(be contract.BaseEvent) error {
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
		assert.NoError(t, err)
	}()

	b, err := args.Pack(name)
	if err != nil {
		assert.FailNow(t, "fail to Pack", err)
	}
	sig := fmt.Sprintf("%v(%v)", "setName", arg.Type.String())
	id := crypto.Keccak256([]byte(sig))[:4]
	to := common.HexToAddress(addr)
	p := &types.LegacyTx{
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

	tx := types.NewTx(p)
	signer := types.LatestSignerForChainID(a.chainID)
	b, err = crypto.Sign(signer.Hash(tx).Bytes(), w.(*Wallet).Key.PrivateKey)
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
}

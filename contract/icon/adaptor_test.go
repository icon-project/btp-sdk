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
	"os"
	"testing"
	"time"

	"github.com/icon-project/btp2/chain/icon/client"
	"github.com/icon-project/btp2/common/log"
	"github.com/icon-project/btp2/common/wallet"
	"github.com/stretchr/testify/assert"

	"github.com/icon-project/btp-sdk/contract"
)

const (
	endpoint = "http://localhost:19080/api/v3/icon"
	nid      = "0x3"
	//cd ../../example/javascore; ./gradlew deployToTest
	addr           = "cxbdb2fac53eaf445f9f0d0c33306d6b2a1a25353d"
	keystoreFile   = "../../example/javascore/src/test/resources/keystore.json"
	keystoreSecret = "../../example/javascore/src/test/resources/keysecret"
)

var (
	adaptorOpt = AdaptorOption{
		NetworkID: client.HexInt(nid),
	}

	w = MustLoadWallet(keystoreFile, keystoreSecret)
)

func MustLoadWallet(keyStoreFile, keyStoreSecret string) wallet.Wallet {
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
	opt, err := contract.EncodeOptions(adaptorOpt)
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
	a := adaptor(t, NetworkTypeIcon)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		err := a.MonitorBaseEvent(ctx, func(be contract.BaseEvent) error {
			t.Logf("%+v", be)
			if paramValue, err := decodePrimitive(contract.TString, be.(*BaseEvent).Data[0]); err != nil {
				assert.NoError(t, err, "fail to decodePrimitive")
				return err
			} else {
				t.Logf("paramValue:%v", paramValue)
				return nil
			}
		}, map[string][]contract.Address{
			"HelloEvent(str)": {addr},
		},
			0)
		assert.Equal(t, ctx.Err(), err)
	}()

	name := "hello"
	p := a.NewTransactionParam(addr, &client.CallData{
		Method: "setName",
		Params: struct {
			Name string `json:"name"`
		}{name},
	})
	p.FromAddress = client.Address(w.Address())
	err := a.SignTransaction(w, p)
	if err != nil {
		assert.FailNow(t, "fail to SignTransaction", err)
	}
	txh, err := a.SendTransaction(p)
	if err != nil {
		assert.FailNow(t, "fail to SendTransaction", err)
	}
	txr, err := a.GetTransactionResult(&client.TransactionHashParam{
		Hash: client.NewHexBytes(txh),
	})
	assert.NoError(t, err)
	t.Logf("%+v", txr)
	<-time.After(5 * time.Second)
	cancel()
}

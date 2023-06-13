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
	"io/ioutil"
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
	keystoreName   = "../../example/javascore/src/test/resources/keystore.json"
	keystoreSecret = "../../example/javascore/src/test/resources/keysecret"
)

var (
	adaptorOpt = AdaptorOption{
		NetworkID: client.HexInt(nid),
	}

	w = loadWallet(keystoreName, keystoreSecret)
)

func loadWallet(keyStoreFile, keyStoreSecret string) wallet.Wallet {
	ks, err := ioutil.ReadFile(keyStoreFile)
	if err != nil {
		panic(err)
	}
	ksp, err := ioutil.ReadFile(keyStoreSecret)
	if err != nil {
		panic(err)
	}
	w, err := wallet.DecryptKeyStore(ks, ksp)
	if err != nil {
		panic(err)
	}
	return w
}

func adaptor(t *testing.T) *Adaptor {
	l := log.GlobalLogger()
	opt, err := contract.EncodeOptions(adaptorOpt)
	if err != nil {
		assert.FailNow(t, "fail to EncodeOptions", err)
	}
	a, err := contract.NewAdaptor(NetworkTypeIcon, endpoint, opt, l)
	if err != nil {
		assert.FailNow(t, "fail to NewAdaptor", err)
	}
	return a.(*Adaptor)
}

func Test_MonitorEvents(t *testing.T) {
	a := adaptor(t)
	go func() {
		err := a.MonitorEvents(func(l []contract.BaseEvent) {
			for _, el := range l {
				t.Logf("%+v", el)
				if v, err := decodePrimitive(contract.TString, el.(*BaseEvent).values[0]); err != nil {
					assert.NoError(t, err, "fail to decodePrimitive")
				} else {
					t.Logf("%v", v)
				}
			}
		}, map[string][]contract.Address{
			"HelloEvent(str)": {addr},
		},
			0)
		assert.NoError(t, err)
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
		Hash: *txh,
	})
	assert.NoError(t, err)
	t.Logf("%+v", txr)
	<-time.After(5 * time.Second)
}

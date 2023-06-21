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

package service

import (
	"encoding/hex"

	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"
	"github.com/icon-project/btp2/common/wallet"

	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/contract/eth"
	"github.com/icon-project/btp-sdk/contract/icon"
)

type Signer struct {
	wallet.Wallet
	NetworkType string
}

type SignerService struct {
	s    Service
	wMap map[string]Signer
	l    log.Logger
}

func (s *SignerService) Name() string {
	return s.s.Name()
}

func (s *SignerService) prepare(network string, options contract.Options) (contract.Options, error) {
	w, ok := s.wMap[network]
	if !ok {
		return nil, errors.Errorf("not found wallet network:%s", network)
	}
	s.l.Debugf("prepare network:%s, address:%s", network, w.Address())
	switch w.NetworkType {
	case icon.NetworkTypeIcon:
		opt := &icon.InvokeOptions{}
		if err := contract.DecodeOptions(options, opt); err != nil {
			return nil, err
		}
		if len(opt.From) == 0 {
			opt.From = contract.Address(w.Address())
		}
		return contract.EncodeOptions(opt)
	case eth.NetworkTypeEth, eth.NetworkTypeEth2:
		opt := &eth.InvokeOptions{}
		if err := contract.DecodeOptions(options, opt); err != nil {
			return nil, err
		}
		if len(opt.From) == 0 {
			opt.From = contract.Address(w.Address())
		}
		return contract.EncodeOptions(opt)
	default:
		return nil, errors.Errorf("not support network type:%s", w.NetworkType)
	}
}

func (s *SignerService) sign(network string, rse contract.RequireSignatureError) (contract.Options, error) {
	w, ok := s.wMap[network]
	if !ok {
		return nil, errors.Errorf("not found wallet network:%s", network)
	}
	sig, err := w.Sign(rse.Data())
	if err != nil {
		return nil, errors.Wrapf(err, "fail to Sign err:%s", err.Error())
	}
	s.l.Debugf("sign network:%s, data:%s, sig:%s", network, hex.EncodeToString(rse.Data()), hex.EncodeToString(sig))
	switch w.NetworkType {
	case icon.NetworkTypeIcon:
		opt := &icon.InvokeOptions{}
		if err = contract.DecodeOptions(rse.Options(), opt); err != nil {
			return nil, err
		}
		if opt.From != contract.Address(w.Address()) {
			//ignore
			return nil, rse
		}
		opt.Signature = sig
		return contract.EncodeOptions(opt)
	case eth.NetworkTypeEth, eth.NetworkTypeEth2:
		opt := &eth.InvokeOptions{}
		if err = contract.DecodeOptions(rse.Options(), opt); err != nil {
			return nil, err
		}
		if opt.From != contract.Address(w.Address()) {
			//ignore
			return nil, rse
		}
		opt.Signature = sig
		return contract.EncodeOptions(opt)
	default:
		return nil, errors.Errorf("not support network type:%s", w.NetworkType)
	}

}

func (s *SignerService) Invoke(network, method string, params contract.Params, options contract.Options) (contract.TxID, error) {
	opt, err := s.prepare(network, options)
	txId, err := s.s.Invoke(network, method, params, opt)
	if err != nil {
		if rse, ok := err.(contract.RequireSignatureError); ok {
			if opt, err = s.sign(network, rse); err != nil {
				return nil, err
			}
			return s.s.Invoke(network, method, params, opt)
		}
		return nil, err
	}
	return txId, err
}

func (s *SignerService) Call(network, method string, params contract.Params, options contract.Options) (contract.ReturnValue, error) {
	return s.s.Call(network, method, params, options)
}

func NewSignerService(s Service, signers map[string]Signer, l log.Logger) (*SignerService, error) {
	return &SignerService{
		s:    s,
		wMap: signers,
		l:    l,
	}, nil
}

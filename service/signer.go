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
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"
	"github.com/icon-project/btp2/common/wallet"

	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/contract/eth"
	"github.com/icon-project/btp-sdk/contract/icon"
)

type Signer interface {
	wallet.Wallet
	NetworkType() string
}

type SignerService struct {
	Service
	m map[string]Signer
	l log.Logger
}

func (s *SignerService) Name() string {
	return s.Service.Name()
}

func PrepareToSign(options contract.Options, s Signer, force bool) (contract.Options, error) {
	switch s.NetworkType() {
	case icon.NetworkTypeIcon:
		opt := &icon.InvokeOptions{}
		if err := contract.DecodeOptions(options, opt); err != nil {
			return nil, err
		}
		if force || len(opt.From) == 0 {
			opt.From = contract.Address(s.Address())
		}
		if force && len(opt.Signature) > 0 {
			opt.Signature = opt.Signature[:0]
		}
		return contract.EncodeOptions(opt)
	case eth.NetworkTypeEth, eth.NetworkTypeEth2:
		opt := &eth.InvokeOptions{}
		if err := contract.DecodeOptions(options, opt); err != nil {
			return nil, err
		}
		if force || len(opt.From) == 0 {
			opt.From = contract.Address(s.Address())
		}
		if force && len(opt.Signature) > 0 {
			opt.Signature = opt.Signature[:0]
		}
		return contract.EncodeOptions(opt)
	default:
		return nil, errors.Errorf("not support network type:%s", s.NetworkType)
	}
}

func Sign(data []byte, options contract.Options, s Signer) (contract.Options, error) {
	sig, err := s.Sign(data)
	if err != nil {
		return nil, errors.Wrapf(err, "fail to Sign err:%s", err.Error())
	}
	switch s.NetworkType() {
	case icon.NetworkTypeIcon:
		opt := &icon.InvokeOptions{}
		if err = contract.DecodeOptions(options, opt); err != nil {
			return nil, err
		}
		if opt.From != contract.Address(s.Address()) {
			return nil, MismatchSignerErrorCode.Errorf("mismatch from expected:%s, actual:%s", s.Address(), opt.From)
		}
		opt.Signature = sig
		return contract.EncodeOptions(opt)
	case eth.NetworkTypeEth, eth.NetworkTypeEth2:
		opt := &eth.InvokeOptions{}
		if err = contract.DecodeOptions(options, opt); err != nil {
			return nil, err
		}
		if opt.From != contract.Address(s.Address()) {
			return nil, MismatchSignerErrorCode.Errorf("mismatch from expected:%s, actual:%s", s.Address(), opt.From)
		}
		opt.Signature = sig
		return contract.EncodeOptions(opt)
	default:
		return nil, errors.Errorf("not support network type:%s", s.NetworkType)
	}

}

func (s *SignerService) Signer(network string) (Signer, error) {
	signer, ok := s.m[network]
	if !ok {
		return nil, errors.Errorf("not found signer network:%s", network)
	}
	return signer, nil
}

func (s *SignerService) Invoke(network, method string, params contract.Params, options contract.Options) (contract.TxID, error) {
	signer, err := s.Signer(network)
	if err != nil {
		return nil, err
	}
	opt, err := PrepareToSign(options, signer, false)
	if err != nil {
		return nil, err
	}
	txId, err := s.Service.Invoke(network, method, params, opt)
	if err != nil {
		if rse, ok := err.(contract.RequireSignatureError); ok {
			if opt, err = Sign(rse.Data(), rse.Options(), signer); err != nil {
				if MismatchSignerErrorCode.Equals(err) {
					return nil, rse
				}
				return nil, err
			}
			return s.Service.Invoke(network, method, params, opt)
		}
		return nil, err
	}
	return txId, err
}

func (s *SignerService) Call(network, method string, params contract.Params, options contract.Options) (contract.ReturnValue, error) {
	return s.Service.Call(network, method, params, options)
}

func NewSignerService(s Service, signers map[string]Signer, l log.Logger) (*SignerService, error) {
	return &SignerService{
		Service: s,
		m:       signers,
		l:       l,
	}, nil
}

type DefaultSigner struct {
	wallet.Wallet
	networkType string
}

func (s *DefaultSigner) NetworkType() string {
	return s.networkType
}

func NewDefaultSigner(w wallet.Wallet, networkType string) Signer {
	return &DefaultSigner{
		Wallet:      w,
		networkType: networkType,
	}
}

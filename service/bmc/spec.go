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

package bmc

import (
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/contract/eth"
	"github.com/icon-project/btp-sdk/contract/icon"
	"github.com/icon-project/btp-sdk/service"
)

const (
	iconContractSpec    = `{"name":"foundation.icon.btp.bmc.BTPMessageCenter","methods":[{"name":"<init>","inputs":[{"name":"_net","type":{"name":"String"}}],"output":{"name":"Void"}},{"name":"getBtpAddress","inputs":[],"output":{"name":"String"},"readOnly":true},{"name":"getNetworkAddress","inputs":[],"output":{"name":"String"},"readOnly":true},{"name":"addVerifier","inputs":[{"name":"_net","type":{"name":"String"}},{"name":"_addr","type":{"name":"Address"}}],"output":{"name":"Void"}},{"name":"removeVerifier","inputs":[{"name":"_net","type":{"name":"String"}}],"output":{"name":"Void"}},{"name":"getVerifiers","inputs":[],"output":{"name":"java.util.Map"},"readOnly":true},{"name":"addService","inputs":[{"name":"_svc","type":{"name":"String"}},{"name":"_addr","type":{"name":"Address"}}],"output":{"name":"Void"}},{"name":"removeService","inputs":[{"name":"_svc","type":{"name":"String"}}],"output":{"name":"Void"}},{"name":"getServices","inputs":[],"output":{"name":"java.util.Map"},"readOnly":true},{"name":"addLink","inputs":[{"name":"_link","type":{"name":"String"}}],"output":{"name":"Void"}},{"name":"removeLink","inputs":[{"name":"_link","type":{"name":"String"}}],"output":{"name":"Void"}},{"name":"getStatus","inputs":[{"name":"_link","type":{"name":"String"}}],"output":{"name":"foundation.icon.btp.lib.BMCStatus"},"readOnly":true},{"name":"getLinks","inputs":[],"output":{"dimension":1,"name":"String"},"readOnly":true},{"name":"addRoute","inputs":[{"name":"_dst","type":{"name":"String"}},{"name":"_link","type":{"name":"String"}}],"output":{"name":"Void"}},{"name":"removeRoute","inputs":[{"name":"_dst","type":{"name":"String"}}],"output":{"name":"Void"}},{"name":"getRoutes","inputs":[],"output":{"name":"java.util.Map"},"readOnly":true},{"name":"setFeeTable","inputs":[{"name":"_dst","type":{"dimension":1,"name":"String"}},{"name":"_value","type":{"dimension":2,"name":"Integer"}}],"output":{"name":"Void"}},{"name":"getFee","inputs":[{"name":"_to","type":{"name":"String"}},{"name":"_response","type":{"name":"Boolean"}}],"output":{"name":"Integer"},"readOnly":true},{"name":"getFeeTable","inputs":[{"name":"_dst","type":{"dimension":1,"name":"String"}}],"output":{"dimension":2,"name":"Integer"},"readOnly":true},{"name":"claimReward","inputs":[{"name":"_network","type":{"name":"String"}},{"name":"_receiver","type":{"name":"String"}}],"output":{"name":"Void"},"payable":true},{"name":"getReward","inputs":[{"name":"_network","type":{"name":"String"}},{"name":"_addr","type":{"name":"Address"}}],"output":{"name":"Integer"},"readOnly":true},{"name":"setFeeHandler","inputs":[{"name":"_addr","type":{"name":"Address"}}],"output":{"name":"Void"}},{"name":"getFeeHandler","inputs":[],"output":{"name":"Address"},"readOnly":true},{"name":"handleRelayMessage","inputs":[{"name":"_prev","type":{"name":"String"}},{"name":"_msg","type":{"name":"String"}}],"output":{"name":"Void"}},{"name":"getNetworkSn","inputs":[],"output":{"name":"Integer"},"readOnly":true},{"name":"sendMessage","inputs":[{"name":"_to","type":{"name":"String"}},{"name":"_svc","type":{"name":"String"}},{"name":"_sn","type":{"name":"Integer"}},{"name":"_msg","type":{"name":"Bytes"}}],"output":{"name":"Integer"},"payable":true},{"name":"handleFragment","inputs":[{"name":"_prev","type":{"name":"String"}},{"name":"_msg","type":{"name":"String"}},{"name":"_idx","type":{"name":"Integer"}}],"output":{"name":"Void"}},{"name":"dropMessage","inputs":[{"name":"_src","type":{"name":"String"}},{"name":"_seq","type":{"name":"Integer"}},{"name":"_svc","type":{"name":"String"}},{"name":"_sn","type":{"name":"Integer"}},{"name":"_nsn","type":{"name":"Integer"}},{"name":"_feeNetwork","type":{"name":"String"}},{"name":"_feeValues","type":{"dimension":1,"name":"Integer"}}],"output":{"name":"Void"}},{"name":"addRelay","inputs":[{"name":"_link","type":{"name":"String"}},{"name":"_addr","type":{"name":"Address"}}],"output":{"name":"Void"}},{"name":"removeRelay","inputs":[{"name":"_link","type":{"name":"String"}},{"name":"_addr","type":{"name":"Address"}}],"output":{"name":"Void"}},{"name":"getRelays","inputs":[{"name":"_link","type":{"name":"String"}}],"output":{"dimension":1,"name":"Address"},"readOnly":true},{"name":"addOwner","inputs":[{"name":"_addr","type":{"name":"Address"}}],"output":{"name":"Void"}},{"name":"removeOwner","inputs":[{"name":"_addr","type":{"name":"Address"}}],"output":{"name":"Void"}},{"name":"getOwners","inputs":[],"output":{"dimension":1,"name":"Address"},"readOnly":true},{"name":"isOwner","inputs":[{"name":"_addr","type":{"name":"Address"}}],"output":{"name":"Boolean"},"readOnly":true},{"name":"addBTPLink","inputs":[{"name":"_link","type":{"name":"String"}},{"name":"_networkId","type":{"name":"Integer"}}],"output":{"name":"Void"}},{"name":"setBTPLinkNetworkId","inputs":[{"name":"_link","type":{"name":"String"}},{"name":"_networkId","type":{"name":"Integer"}}],"output":{"name":"Void"}},{"name":"getBTPLinkNetworkId","inputs":[{"name":"_link","type":{"name":"String"}}],"output":{"name":"Integer"},"readOnly":true},{"name":"getBTPLinkOffset","inputs":[{"name":"_link","type":{"name":"String"}}],"output":{"name":"Integer"},"readOnly":true}],"events":[{"name":"ClaimReward","indexed":2,"inputs":[{"name":"_sender","type":{"name":"Address"}},{"name":"_network","type":{"name":"String"}},{"name":"_receiver","type":{"name":"String"}},{"name":"_amount","type":{"name":"Integer"}},{"name":"_nsn","type":{"name":"Integer"}}]},{"name":"ClaimRewardResult","indexed":2,"inputs":[{"name":"_sender","type":{"name":"Address"}},{"name":"_network","type":{"name":"String"}},{"name":"_nsn","type":{"name":"Integer"}},{"name":"_result","type":{"name":"Integer"}}]},{"name":"Message","indexed":2,"inputs":[{"name":"_next","type":{"name":"String"}},{"name":"_seq","type":{"name":"Integer"}},{"name":"_msg","type":{"name":"Bytes"}}]},{"name":"BTPEvent","indexed":2,"inputs":[{"name":"_src","type":{"name":"String"}},{"name":"_nsn","type":{"name":"Integer"}},{"name":"_next","type":{"name":"String"}},{"name":"_event","type":{"name":"String"}}]},{"name":"MessageDropped","indexed":2,"inputs":[{"name":"_prev","type":{"name":"String"}},{"name":"_seq","type":{"name":"Integer"}},{"name":"_msg","type":{"name":"Bytes"}},{"name":"_ecode","type":{"name":"Integer"}},{"name":"_emsg","type":{"name":"String"}}]}],"structs":[{"name":"foundation.icon.btp.lib.BMVStatus","fields":[{"name":"height","type":{"name":"Integer"}},{"name":"extra","type":{"name":"Bytes"}}]},{"name":"foundation.icon.btp.lib.BMCStatus","fields":[{"name":"rx_seq","type":{"name":"Integer"}},{"name":"tx_seq","type":{"name":"Integer"}},{"name":"verifier","type":{"name":"foundation.icon.btp.lib.BMVStatus"}},{"name":"cur_height","type":{"name":"Integer"}}]}]}`
	ethContractSpec     = `[{"anonymous":false,"inputs":[{"indexed":true,"internalType":"string","name":"_src","type":"string"},{"indexed":true,"internalType":"int256","name":"_nsn","type":"int256"},{"indexed":false,"internalType":"string","name":"_next","type":"string"},{"indexed":false,"internalType":"string","name":"_event","type":"string"}],"name":"BTPEvent","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"_sender","type":"address"},{"indexed":true,"internalType":"string","name":"_network","type":"string"},{"indexed":false,"internalType":"string","name":"_receiver","type":"string"},{"indexed":false,"internalType":"uint256","name":"_amount","type":"uint256"},{"indexed":false,"internalType":"int256","name":"_nsn","type":"int256"}],"name":"ClaimReward","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"_sender","type":"address"},{"indexed":true,"internalType":"string","name":"_network","type":"string"},{"indexed":false,"internalType":"int256","name":"_nsn","type":"int256"},{"indexed":false,"internalType":"uint256","name":"_result","type":"uint256"}],"name":"ClaimRewardResult","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"string","name":"_next","type":"string"},{"indexed":true,"internalType":"uint256","name":"_seq","type":"uint256"},{"indexed":false,"internalType":"bytes","name":"_msg","type":"bytes"}],"name":"Message","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"string","name":"_prev","type":"string"},{"indexed":true,"internalType":"uint256","name":"_seq","type":"uint256"},{"indexed":false,"internalType":"bytes","name":"_msg","type":"bytes"},{"indexed":false,"internalType":"uint256","name":"_ecode","type":"uint256"},{"indexed":false,"internalType":"string","name":"_emsg","type":"string"}],"name":"MessageDropped","type":"event"},{"inputs":[{"internalType":"string","name":"_network","type":"string"},{"internalType":"string","name":"_receiver","type":"string"}],"name":"claimReward","outputs":[],"stateMutability":"payable","type":"function"},{"inputs":[],"name":"getBtpAddress","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"string","name":"_to","type":"string"},{"internalType":"bool","name":"_response","type":"bool"}],"name":"getFee","outputs":[{"internalType":"uint256","name":"_fee","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"getNetworkAddress","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"getNetworkSn","outputs":[{"internalType":"int256","name":"","type":"int256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"string","name":"_network","type":"string"},{"internalType":"address","name":"_addr","type":"address"}],"name":"getReward","outputs":[{"internalType":"uint256","name":"_reward","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"string","name":"_link","type":"string"}],"name":"getStatus","outputs":[{"components":[{"internalType":"uint256","name":"rxSeq","type":"uint256"},{"internalType":"uint256","name":"txSeq","type":"uint256"},{"components":[{"internalType":"uint256","name":"height","type":"uint256"},{"internalType":"bytes","name":"extra","type":"bytes"}],"internalType":"struct IBMV.VerifierStatus","name":"verifier","type":"tuple"},{"internalType":"uint256","name":"currentHeight","type":"uint256"}],"internalType":"struct Types.LinkStatus","name":"_status","type":"tuple"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"string","name":"_prev","type":"string"},{"internalType":"bytes","name":"_msg","type":"bytes"}],"name":"handleRelayMessage","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"string","name":"_to","type":"string"},{"internalType":"string","name":"_svc","type":"string"},{"internalType":"int256","name":"_sn","type":"int256"},{"internalType":"bytes","name":"_msg","type":"bytes"}],"name":"sendMessage","outputs":[{"internalType":"int256","name":"nsn","type":"int256"}],"stateMutability":"payable","type":"function"}]`
	ethBMCMContractSpec = `[{"inputs":[{"internalType":"string","name":"_link","type":"string"}],"name":"addLink","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"string","name":"_link","type":"string"},{"internalType":"address","name":"_addr","type":"address"}],"name":"addRelay","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"string","name":"_dst","type":"string"},{"internalType":"string","name":"_link","type":"string"}],"name":"addRoute","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"string","name":"_svc","type":"string"},{"internalType":"address","name":"_addr","type":"address"}],"name":"addService","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"string","name":"_net","type":"string"},{"internalType":"address","name":"_addr","type":"address"}],"name":"addVerifier","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"string","name":"_src","type":"string"},{"internalType":"uint256","name":"_seq","type":"uint256"},{"internalType":"string","name":"_svc","type":"string"},{"internalType":"int256","name":"_sn","type":"int256"},{"internalType":"int256","name":"_nsn","type":"int256"},{"internalType":"string","name":"_feeNetwork","type":"string"},{"internalType":"uint256[]","name":"_feeValues","type":"uint256[]"}],"name":"dropMessage","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"getFeeHandler","outputs":[{"internalType":"address","name":"_addr","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"string[]","name":"_dst","type":"string[]"}],"name":"getFeeTable","outputs":[{"internalType":"uint256[][]","name":"_feeTable","type":"uint256[][]"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"getLinks","outputs":[{"internalType":"string[]","name":"_links","type":"string[]"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"string","name":"_link","type":"string"}],"name":"getRelays","outputs":[{"internalType":"address[]","name":"_relays","type":"address[]"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"getRoutes","outputs":[{"components":[{"internalType":"string","name":"dst","type":"string"},{"internalType":"string","name":"next","type":"string"}],"internalType":"struct Types.Route[]","name":"_routes","type":"tuple[]"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"getServices","outputs":[{"components":[{"internalType":"string","name":"svc","type":"string"},{"internalType":"address","name":"addr","type":"address"}],"internalType":"struct Types.Service[]","name":"_servicers","type":"tuple[]"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"getVerifiers","outputs":[{"components":[{"internalType":"string","name":"net","type":"string"},{"internalType":"address","name":"addr","type":"address"}],"internalType":"struct Types.Verifier[]","name":"_verifiers","type":"tuple[]"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"string","name":"_link","type":"string"}],"name":"removeLink","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"string","name":"_link","type":"string"},{"internalType":"address","name":"_addr","type":"address"}],"name":"removeRelay","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"string","name":"_dst","type":"string"}],"name":"removeRoute","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"string","name":"_svc","type":"string"}],"name":"removeService","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"string","name":"_net","type":"string"}],"name":"removeVerifier","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"_addr","type":"address"}],"name":"setFeeHandler","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"string[]","name":"_dst","type":"string[]"},{"internalType":"uint256[][]","name":"_value","type":"uint256[][]"}],"name":"setFeeTable","outputs":[],"stateMutability":"nonpayable","type":"function"}]`
)

var (
	iconSpec = MustNewContractSpec(icon.NetworkTypeIcon, []byte(iconContractSpec))
	ethSpec  = MustNewContractSpec(eth.NetworkTypeEth, []byte(ethContractSpec))

	mergeInfos = map[string]mergeInfo{
		"getServices": {
			Flag: service.MethodOverloadOutput,
			Spec: iconSpec,
		},
		"getVerifiers": {
			Flag: service.MethodOverloadOutput,
			Spec: iconSpec,
		},
		"getRoutes": {
			Flag: service.MethodOverloadOutput,
			Spec: iconSpec,
		},
		"getStatus": {
			Flag: service.MethodOverloadOutput,
			Spec: iconSpec,
		},
		"handleRelayMessage": {
			Flag: service.MethodOverloadInputs,
			Spec: ethSpec,
		},
	}
)

type mergeInfo struct {
	Flag interface{}
	Spec *contract.Spec
}

func MustNewContractSpec(networkType string, b []byte) *contract.Spec {
	var (
		s   *contract.Spec
		err error
	)
	switch networkType {
	case icon.NetworkTypeIcon:
		s, err = icon.NewSpec(b)
	case eth.NetworkTypeEth, eth.NetworkTypeEth2:
		s, err = eth.NewSpec(b)
	default:
		err = errors.Errorf("not supported network type:%s", networkType)
	}
	if err != nil {
		log.Panicf("fail to NewSpec err:%+v", err)
	}
	return s
}

func specForceMerge(ss *service.Spec, infos map[string]mergeInfo) error {
	for name, info := range infos {
		switch f := info.Flag.(type) {
		case service.MethodOverloadFlag:
			sm := ss.Methods[name]
			if sm == nil {
				return errors.Errorf("not found method:%s in service spec", name)
			}
			cm := info.Spec.MethodMap[name]
			if cm == nil {
				return errors.Errorf("not found method:%s in contract spec", name)
			}
			if err := sm.MergeOverloads(cm, f); err != nil {
				return err
			}
		case service.EventOverloadFlag:
			se := ss.Events[name]
			if se == nil {
				return errors.Errorf("not found event:%s in service spec", name)
			}
			ce := info.Spec.EventMap[name]
			if ce == nil {
				return errors.Errorf("not found event:%s in contract spec", name)
			}
			if err := se.MergeOverloads(ce, f); err != nil {
				return err
			}
		default:
			return errors.Errorf("not support flag type %T", info.Flag)
		}
	}
	return nil
}

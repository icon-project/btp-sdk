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

package dappsample

const (
	iconContractSpec = `{"name":"foundation.icon.btp.example.DAppSample","methods":[{"name":"<init>","inputs":[],"output":{"name":"Void"}},{"name":"setXCall","inputs":[{"name":"_addr","type":{"name":"Address"}}],"output":{"name":"Void"}},{"name":"getXCall","inputs":[],"output":{"name":"Address"},"readOnly":true},{"name":"getLastSn","inputs":[],"output":{"name":"Integer"},"readOnly":true},{"name":"sendMessage","inputs":[{"name":"_to","type":{"name":"String"}},{"name":"_data","type":{"name":"Bytes"}},{"name":"_rollback","type":{"name":"Bytes"},"optional":true}],"output":{"name":"Void"},"payable":true},{"name":"handleCallMessage","inputs":[{"name":"_from","type":{"name":"String"}},{"name":"_data","type":{"name":"Bytes"}}],"output":{"name":"Void"}}],"events":[{"name":"Sent","indexed":0,"inputs":[{"name":"_to","type":{"name":"String"}},{"name":"_sn","type":{"name":"Integer"}},{"name":"_rollback","type":{"name":"Integer"}},{"name":"_xcallSn","type":{"name":"Integer"}}]},{"name":"MessageReceived","indexed":0,"inputs":[{"name":"_from","type":{"name":"String"}},{"name":"_sn","type":{"name":"Integer"}},{"name":"_data","type":{"name":"Bytes"}}]},{"name":"RollbackDataReceived","indexed":0,"inputs":[{"name":"_sn","type":{"name":"Integer"}},{"name":"_data","type":{"name":"Bytes"}},{"name":"_xcallSn","type":{"name":"Integer"}}]}],"structs":[]}`
	ethContractSpec  = `[{"anonymous":false,"inputs":[{"indexed":false,"internalType":"uint8","name":"version","type":"uint8"}],"name":"Initialized","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"string","name":"_from","type":"string"},{"indexed":false,"internalType":"uint256","name":"_sn","type":"uint256"},{"indexed":false,"internalType":"bytes","name":"_data","type":"bytes"}],"name":"MessageReceived","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"uint256","name":"_sn","type":"uint256"},{"indexed":false,"internalType":"bytes","name":"_data","type":"bytes"},{"indexed":false,"internalType":"uint256","name":"_xcallSn","type":"uint256"}],"name":"RollbackDataReceived","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"string","name":"_to","type":"string"},{"indexed":false,"internalType":"uint256","name":"_sn","type":"uint256"},{"indexed":false,"internalType":"uint256","name":"_rollback","type":"uint256"},{"indexed":false,"internalType":"uint256","name":"_xcallSn","type":"uint256"}],"name":"Sent","type":"event"},{"inputs":[],"name":"getLastSn","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"getXCall","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"string","name":"_from","type":"string"},{"internalType":"bytes","name":"_data","type":"bytes"}],"name":"handleCallMessage","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"initialize","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"string","name":"_to","type":"string"},{"internalType":"bytes","name":"_data","type":"bytes"},{"internalType":"bytes","name":"_rollback","type":"bytes"}],"name":"sendMessage","outputs":[],"stateMutability":"payable","type":"function"},{"inputs":[{"internalType":"string","name":"_to","type":"string"},{"internalType":"bytes","name":"_data","type":"bytes"}],"name":"sendMessage","outputs":[],"stateMutability":"payable","type":"function"},{"inputs":[{"internalType":"address","name":"_addr","type":"address"}],"name":"setXCall","outputs":[],"stateMutability":"nonpayable","type":"function"}]`
)

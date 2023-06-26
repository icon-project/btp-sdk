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

package xcall

const (
	/**
	add belows to ../../example/javascore/build.gradle
	dependencies {
	 implementation 'foundation.icon:btp2-xcall:0.6.1'
	 ...
	}

	task xcallJar(type: Jar) {
		manifest {
			attributes(
			'Main-Class' : 'foundation.icon.btp.xcall.CallServiceImpl'
		)
	}
		from {
			configurations.runtimeClasspath.collect { it.isDirectory() ? it : zipTree(it) }
		} { exclude "score/*" }
	}

	task xcallGenerateContractSpec(type: GenerateContractSpecTask) {
		dependsOn xcallJar
		jarFile = xcallJar.archiveFile
		contractSpecFile = file("$buildDir/generated/xcallContractSpec.json")
	}
	cd ../../example/javascore
	./gradlew xcallGenerateSpec
	mv build/generated/xcallContractSpec.json ./
	*/
	iconContractSpec = `{"name":"foundation.icon.btp.xcall.CallServiceImpl","methods":[{"name":"<init>","inputs":[{"name":"_bmc","type":{"name":"Address"}}],"output":{"name":"Void"}},{"name":"getBtpAddress","inputs":[],"output":{"name":"String"},"readOnly":true},{"name":"sendCallMessage","inputs":[{"name":"_to","type":{"name":"String"}},{"name":"_data","type":{"name":"Bytes"}},{"name":"_rollback","optional":true,"type":{"name":"Bytes"}}],"output":{"name":"Integer"},"payable":true},{"name":"executeCall","inputs":[{"name":"_reqId","type":{"name":"Integer"}}],"output":{"name":"Void"}},{"name":"executeRollback","inputs":[{"name":"_sn","type":{"name":"Integer"}}],"output":{"name":"Void"}},{"name":"handleBTPMessage","inputs":[{"name":"_from","type":{"name":"String"}},{"name":"_svc","type":{"name":"String"}},{"name":"_sn","type":{"name":"Integer"}},{"name":"_msg","type":{"name":"Bytes"}}],"output":{"name":"Void"}},{"name":"handleBTPError","inputs":[{"name":"_src","type":{"name":"String"}},{"name":"_svc","type":{"name":"String"}},{"name":"_sn","type":{"name":"Integer"}},{"name":"_code","type":{"name":"Integer"}},{"name":"_msg","type":{"name":"String"}}],"output":{"name":"Void"}},{"name":"admin","inputs":[],"output":{"name":"Address"},"readOnly":true},{"name":"setAdmin","inputs":[{"name":"_address","type":{"name":"Address"}}],"output":{"name":"Void"}},{"name":"setProtocolFeeHandler","inputs":[{"name":"_addr","optional":true,"type":{"name":"Address"}}],"output":{"name":"Void"}},{"name":"getProtocolFeeHandler","inputs":[],"output":{"name":"Address"},"readOnly":true},{"name":"setProtocolFee","inputs":[{"name":"_value","type":{"name":"Integer"}}],"output":{"name":"Void"}},{"name":"getProtocolFee","inputs":[],"output":{"name":"Integer"},"readOnly":true},{"name":"getFee","inputs":[{"name":"_net","type":{"name":"String"}},{"name":"_rollback","type":{"name":"Boolean"}}],"output":{"name":"Integer"},"readOnly":true}],"events":[{"name":"CallMessage","indexed":3,"inputs":[{"name":"_from","type":{"name":"String"}},{"name":"_to","type":{"name":"String"}},{"name":"_sn","type":{"name":"Integer"}},{"name":"_reqId","type":{"name":"Integer"}}]},{"name":"CallExecuted","indexed":1,"inputs":[{"name":"_reqId","type":{"name":"Integer"}},{"name":"_code","type":{"name":"Integer"}},{"name":"_msg","type":{"name":"String"}}]},{"name":"ResponseMessage","indexed":1,"inputs":[{"name":"_sn","type":{"name":"Integer"}},{"name":"_code","type":{"name":"Integer"}},{"name":"_msg","type":{"name":"String"}}]},{"name":"RollbackMessage","indexed":1,"inputs":[{"name":"_sn","type":{"name":"Integer"}}]},{"name":"RollbackExecuted","indexed":1,"inputs":[{"name":"_sn","type":{"name":"Integer"}},{"name":"_code","type":{"name":"Integer"}},{"name":"_msg","type":{"name":"String"}}]},{"name":"CallMessageSent","indexed":3,"inputs":[{"name":"_from","type":{"name":"Address"}},{"name":"_to","type":{"name":"String"}},{"name":"_sn","type":{"name":"Integer"}},{"name":"_nsn","type":{"name":"Integer"}}]}],"structs":[]}`

	/**
	git clone https://github.com/icon-project/btp2-solidity
	cd btp2-solidity/xcall
	yarn install
	npx hardhat compile
	cp build/hardhat/artifacts/contracts/CallService.sol/CallService.json ./
	jq .abi CallService.json
	*/
	ethContractSpec = `[{"anonymous":false,"inputs":[{"indexed":true,"internalType":"uint256","name":"_reqId","type":"uint256"},{"indexed":false,"internalType":"int256","name":"_code","type":"int256"},{"indexed":false,"internalType":"string","name":"_msg","type":"string"}],"name":"CallExecuted","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"string","name":"_from","type":"string"},{"indexed":true,"internalType":"string","name":"_to","type":"string"},{"indexed":true,"internalType":"uint256","name":"_sn","type":"uint256"},{"indexed":false,"internalType":"uint256","name":"_reqId","type":"uint256"}],"name":"CallMessage","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"_from","type":"address"},{"indexed":true,"internalType":"string","name":"_to","type":"string"},{"indexed":true,"internalType":"uint256","name":"_sn","type":"uint256"},{"indexed":false,"internalType":"int256","name":"_nsn","type":"int256"}],"name":"CallMessageSent","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"uint8","name":"version","type":"uint8"}],"name":"Initialized","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"uint256","name":"_sn","type":"uint256"},{"indexed":false,"internalType":"int256","name":"_code","type":"int256"},{"indexed":false,"internalType":"string","name":"_msg","type":"string"}],"name":"ResponseMessage","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"uint256","name":"_sn","type":"uint256"},{"indexed":false,"internalType":"int256","name":"_code","type":"int256"},{"indexed":false,"internalType":"string","name":"_msg","type":"string"}],"name":"RollbackExecuted","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"uint256","name":"_sn","type":"uint256"}],"name":"RollbackMessage","type":"event"},{"inputs":[],"name":"admin","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"_reqId","type":"uint256"}],"name":"executeCall","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"_sn","type":"uint256"}],"name":"executeRollback","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"getBtpAddress","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"string","name":"_net","type":"string"},{"internalType":"bool","name":"_rollback","type":"bool"}],"name":"getFee","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"getProtocolFee","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"getProtocolFeeHandler","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"string","name":"_src","type":"string"},{"internalType":"string","name":"_svc","type":"string"},{"internalType":"uint256","name":"_sn","type":"uint256"},{"internalType":"uint256","name":"_code","type":"uint256"},{"internalType":"string","name":"_msg","type":"string"}],"name":"handleBTPError","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"string","name":"_from","type":"string"},{"internalType":"string","name":"_svc","type":"string"},{"internalType":"uint256","name":"_sn","type":"uint256"},{"internalType":"bytes","name":"_msg","type":"bytes"}],"name":"handleBTPMessage","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"_bmc","type":"address"}],"name":"initialize","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"string","name":"_to","type":"string"},{"internalType":"bytes","name":"_data","type":"bytes"},{"internalType":"bytes","name":"_rollback","type":"bytes"}],"name":"sendCallMessage","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"payable","type":"function"},{"inputs":[{"internalType":"address","name":"_address","type":"address"}],"name":"setAdmin","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"_value","type":"uint256"}],"name":"setProtocolFee","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"_addr","type":"address"}],"name":"setProtocolFeeHandler","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"toAddr","type":"address"},{"internalType":"string","name":"to","type":"string"},{"internalType":"string","name":"from","type":"string"},{"internalType":"bytes","name":"data","type":"bytes"}],"name":"tryHandleCallMessage","outputs":[],"stateMutability":"nonpayable","type":"function"}]`
)
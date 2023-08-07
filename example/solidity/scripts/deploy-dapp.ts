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

import { ethers, upgrades } from "hardhat";

async function main() {
    const DAppSample = await ethers.getContractFactory("DAppSample");
    const dAppSample = await upgrades.deployProxy(DAppSample);
    await dAppSample.deployed();
    await dAppSample.setXCall("0x7B4f352Cd40114f12e82fC675b5BA8C7582FC513");
    console.log(
        `DAppSample deployed to ${dAppSample.address} txHash:${dAppSample.deployTransaction.hash}`
    );

    // Upgrading
    // const dAppSample = upgrades.upgradeProxy(address, DAppSample);
}

// We recommend this pattern to be able to use async/await everywhere
// and properly handle errors.
main().catch((error) => {
    console.error(error);
    process.exitCode = 1;
});

import { HardhatUserConfig } from "hardhat/config";
import "@nomicfoundation/hardhat-toolbox";
import "solidity-coverage";
import "hardhat-contract-sizer";

const config: HardhatUserConfig = {
  defaultNetwork: "hardhat",
  networks: {
    hardhat: {
      // mining: {
      //   auto: false,
      //   interval: 3000
      // },
      allowUnlimitedContractSize: true,
      // blockGasLimit: 0x1fffffffffffff,
      // gas:  0xffffffffff,
      // gasPrice: 0x01,
      initialBaseFeePerGas: 0,
      accounts: [
        {
          privateKey: "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
          balance: "10000000000000000000000"
        },
        {
          privateKey: "0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d",
          balance: "10000000000000000000000"
        },
        {
          privateKey: "0x8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63",
          balance: "10000000000000000000000"
        },
        {
          privateKey: "0xa6d23a0b704b649a92dd56bdff0f9874eeccc9746f10d78b683159af1617e08f",
          balance: "10000000000000000000000"
        }
      ]
    }
  },
  solidity: {
    version: "0.8.12",
    settings: {
      optimizer: {
        enabled: true,
        runs: 10
      }
    }
  },
  mocha: {
    timeout: 600000
  },
  contractSizer: {
    only:[
        "HelloWorld"
    ],
    except:[
    ]
  }
};

export default config;

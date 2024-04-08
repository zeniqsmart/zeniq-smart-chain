require("@nomiclabs/hardhat-truffle5");

module.exports = {
  defaultNetwork: "smartzeniq_local",
  networks: {
    smartzeniq_local: {
      url: "172.18.188.10:8545", // docker-compose.yml
      chainId: 0x59454E4951,
    },
  },
  solidity: {
    version: "0.8.0",
    settings: {
      optimizer: {
        enabled: true,
        runs: 200,
      },
    },
  },
  paths: {
  sources: "./contracts",
    tests: "./test",
    cache: "./cache",
    artifacts: "./artifacts",
  },
  mocha: {
    timeout: 40000
  }
}

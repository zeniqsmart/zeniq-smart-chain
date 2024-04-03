const HDWalletProvider = require('@truffle/hdwallet-provider');
module.exports = {

/*
truffle test --network smartzeniq_local
*/

  networks: {
    smartzeniq_local: {
      host: "172.18.188.10", // docker-compose.yml
      port: 8545,
      network_id: "*",
      gasPrice: 10000000000,
      accounts: ["0xf96ae03f3637e3195ebcb85f0043052338196e57",],
      from: "0xf96ae03f3637e3195ebcb85f0043052338196e57",
    },
  },

  // Set default mocha options here, use special reporters etc.
  mocha: {
    timeout: 1000000
  },

  // Configure your compilers
  compilers: {
    solc: {
      version: "0.8.0",    // Fetch exact version from solc-bin (default: truffle's version)
    },
  },
};

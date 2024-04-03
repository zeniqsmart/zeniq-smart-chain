#!/usr/bin/env bash

set -e

if ! type truffle; then
  echo First do:
  echo '
npm install -g truffle
npm install -g ganache
'
exit 0
fi

curl -X POST --data '{"jsonrpc":"2.0","method":"net_version","params":[],"id":67}' \
  -H "Content-Type: application/json" http://172.18.188.10:8545

cd testdata/sol

npm install

truffle test --network=smartzeniq_local

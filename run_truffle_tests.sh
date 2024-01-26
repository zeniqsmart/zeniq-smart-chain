#!/usr/bin/env bash

set -e

if ! type truffle; then
  echo You need to first:
  echo '
npm install -g truffle
npm install -g ganache
'
exit 0
fi

curl -X POST --data '{"jsonrpc":"2.0","method":"net_version","params":[],"id":67}' \
  -H "Content-Type: application/json" http://localhost:8545

cd testdata/sol

npm install

truffle test --network=sbch_local

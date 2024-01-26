#!/usr/bin/env bash

: '

Usage:

sudo rm -rf build
sudo rm -rf testnodes
./docker_build.sh
./testnettest.sh


Variables: 

- $ZENIQRPCPORT: default 57319

Make sure zeniq.conf allows connection from docker:

rpcbind=0.0.0.0
rpcallowip=127.0.0.1
rpcallowip=172.17.0.1/16
rpcallowip=172.18.0.1/16
rpcallowip=192.168.0.0/16


test:
build/zeniqsmartd testnet --v 4 --o testnodes --populate-persistent-peers --starting-ip-address 172.18.188.10
docker run -it -v ${PWD%/*}:/zeniq_smart -v ${PWD}/testnodes/node0:/root/.zeniqsmartd zeniqsmart bash
BC=$(http --auth zeniq:zeniq123 http://172.17.0.1:57319 method=getblockcount params:="[]" | grep result | cut -d ':' -f 2 | cut -d ',' -f 1)
echo "$BC will be first mainnet block of ccrpc epoch 0"
sed -i "s/cc-rpc-epochs.*/cc-rpc-epochs = [[$BC,6]]/g" testnodes/node0/config/app.toml
sed -i "s/cc-rpc-fork-block.*/cc-rpc-fork-block = 0/g" testnodes/node0/config/app.toml
sed -i "s/mainnet-rpc-url.*/mainnet-rpc-url = \"http:\/\/172.17.0.1:$zeniqrpcport\"/g" testnodes/node0/config/app.toml
build/zeniqsmartd start
'


THS=$PWD
if [[ "${THS##*/}" != "zeniq-smart-chain" ]]; then
    cd "zeniq-smart-chain"
    THS=$THS/zeniq-smart-chain
fi
BD=$THS/build

$BD/zeniqsmartd testnet --v 4 --o testnodes --populate-persistent-peers --starting-ip-address 172.18.188.10

# sed -i 's/log_level.*/log_level = "debug"/g'  testnodes/node0/config/config.toml
# sed -i 's/log_level.*/log_level = "debug"/g'  testnodes/node1/config/config.toml
# sed -i 's/log_level.*/log_level = "debug"/g'  testnodes/node2/config/config.toml
# sed -i 's/log_level.*/log_level = "debug"/g'  testnodes/node3/config/config.toml

zeniqrpcport=${ZENIQRPCPORT:-57319}
BC_1="$(http --auth zeniq:zeniq123 http://172.17.0.1:$zeniqrpcport method=getblockcount params:='[]' | grep result | jq -r .result)"
BC=$(echo 1+"$BC_1" | bc)

if [[ "empty$BC" == "empty" ]]; then
    echo "ERROR: For this test zeniqd must run locally!"
    exit 1
fi
echo "$BC will be first mainnet block of ccrpc epoch 0"

sed -i "s/cc-rpc-epochs.*/cc-rpc-epochs = [[$BC,6]]/g" testnodes/node0/config/app.toml
sed -i "s/cc-rpc-fork-block.*/cc-rpc-fork-block = 0/g" testnodes/node0/config/app.toml
sed -i "s/mainnet-rpc-url.*/mainnet-rpc-url = \"http:\/\/172.17.0.1:$zeniqrpcport\"/g" testnodes/node0/config/app.toml
sed -i "s/cc-rpc-epochs.*/cc-rpc-epochs = [[$BC,6]]/g" testnodes/node1/config/app.toml
sed -i "s/cc-rpc-fork-block.*/cc-rpc-fork-block = 0/g" testnodes/node1/config/app.toml
sed -i "s/mainnet-rpc-url.*/mainnet-rpc-url = \"http:\/\/172.17.0.1:$zeniqrpcport\"/g" testnodes/node1/config/app.toml
sed -i "s/cc-rpc-epochs.*/cc-rpc-epochs = [[$BC,6]]/g" testnodes/node2/config/app.toml
sed -i "s/cc-rpc-fork-block.*/cc-rpc-fork-block = 0/g" testnodes/node2/config/app.toml
sed -i "s/mainnet-rpc-url.*/mainnet-rpc-url = \"http:\/\/172.17.0.1:$zeniqrpcport\"/g" testnodes/node2/config/app.toml
sed -i "s/cc-rpc-epochs.*/cc-rpc-epochs = [[$BC,6]]/g" testnodes/node3/config/app.toml
sed -i "s/cc-rpc-fork-block.*/cc-rpc-fork-block = 0/g" testnodes/node3/config/app.toml
sed -i "s/mainnet-rpc-url.*/mainnet-rpc-url = \"http:\/\/172.17.0.1:$zeniqrpcport\"/g" testnodes/node3/config/app.toml

docker compose up


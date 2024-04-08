#!/usr/bin/env bash

: '

Usage:

sudo rm -rf build
sudo ./docker_build.sh
sudo ./docker_testnet.sh


Make sure zeniq.conf allows connection from docker:

rpcbind=0.0.0.0
rpcallowip=127.0.0.1
rpcallowip=172.17.0.1/16
rpcallowip=172.18.0.1/16
rpcallowip=192.168.0.0/16


test:
build/zeniqsmartd testnet --v 4 --o build/testnodes --populate-persistent-peers --starting-ip-address 172.18.188.10
docker run -it -v ${PWD%/*}:/zeniq_smart -v ${PWD}/build/testnodes/node0:/root/.zeniqsmartd zeniqsmart bash
BC=$(curl -X POST --data-binary '{"jsonrpc":"1.0","method":"getblockcount","params":[],"id":9999}' -H "Content-Type: application/json" http://zeniq:zeniq123@127.0.0.1:57319 | jq -r '.result')
echo "$BC will be first mainnet block of ccrpc epoch 0"
sed -i "s/cc-rpc-epochs.*/cc-rpc-epochs = [[$BC,6,2400]]/g" build/testnodes/node0/config/app.toml
sed -i "s/cc-rpc-fork-block.*/cc-rpc-fork-block = 0/g" build/testnodes/node0/config/app.toml
sed -i "s/mainnet-rpc-url.*/mainnet-rpc-url = \"http:\/\/172.17.0.1:57319\"/g" build/testnodes/node0/config/app.toml
build/zeniqsmartd start --testing


#keys from testval.sh
zsmartgenesiskey="0xe127f1fddebd3218eabb5b3e41ffc55db9a526555a8d99b263fb73c9c5deaf2c"
zsmartgenesis="0x53CB74974D4CddEF438DE77B13F18Eb3FA6309E8"
sudo ./docker_testnet.sh 0 $zsmartgenesis
'


THS="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
cd $THS

BD=$THS/build

rm -rf $BD/testnodes # else InitChain is not called

$BD/zeniqsmartd testnet --v 4 --o $BD/testnodes --populate-persistent-peers --starting-ip-address 172.18.188.10

# sed -i 's/log_level.*/log_level = "debug"/g'  $BD/testnodes/node0/config/config.toml
# sed -i 's/log_level.*/log_level = "debug"/g'  $BD/testnodes/node1/config/config.toml
# sed -i 's/log_level.*/log_level = "debug"/g'  $BD/testnodes/node2/config/config.toml
# sed -i 's/log_level.*/log_level = "debug"/g'  $BD/testnodes/node3/config/config.toml

BC_1=$(curl -X POST --data-binary '{"jsonrpc":"1.0","method":"getblockcount","params":[],"id":9999}' -H "Content-Type: application/json" http://zeniq:zeniq123@127.0.0.1:57319 | jq -r '.result')

BC=0
if [[ "$1" != "0" ]] ; then
    BC=$((1+BC_1))

    if [[ "empty$BC" == "empty" ]]; then
        echo "ERROR: For this test zeniqd must run locally!"
        exit 1
    fi
fi

echo "$BC will be first mainnet block of ccrpc epoch 0"

if [[ "$2" != "" ]] ; then
    sed -i "s/0xf96ae03f3637e3195ebcb85f0043052338196e57/$2/g" $BD/testnodes/node0/config/genesis.json
    sed -i "s/0xf96ae03f3637e3195ebcb85f0043052338196e57/$2/g" $BD/testnodes/node1/config/genesis.json
    sed -i "s/0xf96ae03f3637e3195ebcb85f0043052338196e57/$2/g" $BD/testnodes/node2/config/genesis.json
    sed -i "s/0xf96ae03f3637e3195ebcb85f0043052338196e57/$2/g" $BD/testnodes/node3/config/genesis.json
fi

patch_node(){
sed -i "s/cc-rpc-epochs.*/cc-rpc-epochs = [${2}]/g" $BD/testnodes/node${1}/config/app.toml
sed -i "s/cc-rpc-fork-block.*/cc-rpc-fork-block = 0/g" $BD/testnodes/node${1}/config/app.toml
sed -i "s/mainnet-rpc-url.*/mainnet-rpc-url = \"http:\/\/172.17.0.1:57319\"/g" $BD/testnodes/node${1}/config/app.toml
}

# 3600 instead of 2400 because crosschaintest.sh has 100 fast blocks at the beginning to reach maturity
patch_node 0 "[$BC,6,3600]"
patch_node 1 "[$BC,6,3600]"
patch_node 2 "[$BC,6,3600]"
patch_node 3 "[$BC,6,3600]"

(cd $THS && docker compose up)


#!/usr/bin/env bash

: '
Make sure zeniq.conf allows connection from docker:

rpcbind=0.0.0.0
rpcallowip=127.0.0.1
rpcallowip=172.17.0.1/16
rpcallowip=172.18.0.1/16
rpcallowip=192.168.0.0/16

sudo rm -rf build
./docker_build.sh

testing:

docker run -it -v ${PWD%/*}:/zeniq_smart zeniqsmart bash
http --auth zeniq:zeniq123 http://172.17.0.1:57319 method=getblockcount params:='[]'
cd /zeniq_smart/zeniq-smart-chain
./build/zeniqsmartd init dockerzeniqsmartd --chain-id 0x59454E4951
\cp -rf config/* $HOME/.zeniqsmartd/config/
cat $HOME/.zeniqsmartd/config/app.toml
sed -i "s/mainnet-rpc-url.*/mainnet-rpc-url = \"http:\/\/172.17.0.1:57319\"/g" $HOME/.zeniqsmartd/config/app.toml
# ... too slow startup
# sed -i "s/zeniqsmart-rpc-url.*/#\0/g" $HOME/.zeniqsmartd/config/app.toml
./build/zeniqsmartd start --testing
exit

docker run --network="host" -it -v ${PWD%/*}:/zeniq_smart zeniqsmart bash
http --auth zeniq:zeniq123 http://127.0.0.1:57319 method=getblockcount params:='[]'
cd /zeniq_smart/zeniq-smart-chain
./build/zeniqsmartd init dockerzeniqsmartd --chain-id 0x59454E4951
\cp -rf config/* $HOME/.zeniqsmartd/config/
./build/zeniqsmartd start --testing
exit

'

THS="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
cd $THS

docker run --network="host" -t -v ${THS%/*}:/zeniq_smart zeniqsmart \
bash -c "cd /root/                                                                              && \
/zeniq_smart/zeniq-smart-chain/build/zeniqsmartd init dockerzeniqsmartd --chain-id 0x59454E4951 && \
cp /zeniq_smart/zeniq-smart-chain/config/* .zeniqsmartd/config/                                 && \
/zeniq_smart/zeniq-smart-chain/build/zeniqsmartd start --testing"


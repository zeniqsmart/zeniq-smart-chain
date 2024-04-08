#!/usr/bin/env bash

: '

docker_testnet.sh uses a 6-block-epoch, ie one hour
crosschain txs made in the first hour will be accounted at the end of the 3rd hour

Build:

sudo rm -rf build
sudo ./docker_build.sh

Manual testing:

zeniqd=$HOME/zeniq-core_build/src/zeniqd
echo $zeniqd

. ./crosschaintest.sh

z_mainnet
z_smartnet  # needs sudo then blocks

# in other terminal
. /tmp/tmp..../zinfo.sh
. ./crosschaintest.sh

z_crss_at_height 2
h2=$(z_main_height)
echo $h2
z_main_crss_from_to $h2 $h2

# NOW wait until 3 epochs (= 3 hours) are over

# miner address on mainnet
echo $zaddr
echo $zaddrkey
# accoring addres on smartnet
echo $zsmartaddr
echo $zsmartaddrkey

# after 3 hours this should not be 0
z_smart_balance $zsmartaddr
z_smart_height

# spend the ZENIQ from crosschain
zsmartaddrnewkey=$(z_smart_new)
zsmartaddrnew=$(z_smart_addr $zsmartaddrnewkey)
z_smart_spend 20000000000000000000 $zsmartaddrnew
z_smart_balance $zsmartaddrnew

# spend from genesis alloc account
zsmartaddrnew1key=$(z_smart_new)
echo $zsmartaddrnew1key
zsmartaddrnew1=$(z_smart_addr $zsmartaddrnew1key)
echo $zsmartaddrnew1
echo $zsmartgenesis
echo $zsmartgenesiskey
zsenttx=$(z_smart_spend 20000000000000000000 $zsmartaddrnew1 $zsmartgenesis $zsmartgenesiskey)
echo $zsenttx
z_smart_balance $zsmartaddrnew1

z_smart_height

# To enter a container
docker exec -it zeniq-smart-chain-node1-1 bash
ps fax
cd ~/.zeniqsmartd

# To stop a container
docker ps
docker stop zeniq-smart-chain-node1-1
sudo rm -rf build/testnodes/node1/data
ll build/testnodes/node1/
docker start zeniq-smart-chain-node1-1
docker ps

# To stop all containers and processes
# does not remove the bash scripts, though.
z_cleanup

# manually remove the sleep and parent script
ps fax
kill -9 ...

'

zsmartdir="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"

if [[ "$zeniqd" == "" ]]; then
   echo "zeniqd needs to point to the executable"
fi

z_datadir() {
   zcli=${zeniqd/zeniqd/zeniq-cli}
   zdatadir=$(mktemp -d)
   cat > $zdatadir/zeniq.conf << EOF
server=1

rpcport=57319
rpcbind=0.0.0.0
rpcallowip=127.0.0.1
rpcallowip=172.17.0.0/16
rpcallowip=172.18.0.0/16
rpcallowip=192.168.0.0/16
rpcuser=zeniq
rpcpassword=zeniq123

[regtest]

rpcport=57319
rpcbind=0.0.0.0
rpcallowip=127.0.0.1
rpcallowip=172.17.0.0/16
rpcallowip=172.18.0.0/16
rpcallowip=192.168.0.0/16
rpcuser=zeniq
rpcpassword=zeniq123
EOF
   echo $zdatadir
}

z_do(){
   $zcli -datadir=$zdatadir $@
}

z_zeniqd() {
   $zeniqd -datadir=$zdatadir -gen -printtoconsole -regtest -port=57319 &
   zdpid=$!
   sleep 1
   zaddr=$(z_do getnewaddress)
   zaddrkey=$(z_do dumpprivkey $zaddr)
   zsmartaddrkey=$(z_smartkey_from_main_pk $zaddr)
   zsmartaddr=$(z_smart_addr $zsmartaddrkey)
   #before the zeniqsmartd part we can still violate timing
   for i in $(seq 1 100); do
      z_do -regtest generatetoaddress 1 $zaddr
      echo "$i of 100 to fulfill COINBASE_MATURITY = 100"
      sleep 1
   done
   echo "mining to $zaddr 1 block every 10 min"
   bash -c "while true; do $zcli -datadir=$zdatadir -regtest generatetoaddress 1 $zaddr; sleep 600; done" &
   zgenpid=$!
}

z_info(){
   cat > $zdatadir/zinfo.sh << EOF
export WEB3_HTTP_PROVIDER_URI="http://172.18.188.10:8545"
zdatadir=$zdatadir
zsmartdir=$zsmartdir
zeniqd=$zeniqd
zcli=$zcli
zdpid=$zdpid
zgenpid=$zgenpid
zaddr=$zaddr
zaddrkey=$zaddrkey
zsmartaddrkey=$zsmartaddrkey
zsmartaddr=$zsmartaddr
zsmartgenesiskey=$zsmartgenesiskey
zsmartgenesis=$zsmartgenesis
EOF
   echo ". $zdatadir/zinfo.sh to work on another terminal"
}

z_cleanup(){
   docker compose down
   echo "killing $zdpid"
   kill -9 $zdpid
   zdpid=''
   echo "killing $zgenpid"
   kill -9 $zgenpid
   zgenpid=''
}

z_smartnet(){
   export WEB3_HTTP_PROVIDER_URI="http://172.18.188.10:8545"
   echo "logging in $zdatadir/smarttestnet.log"

   zsmartgenesiskey=$(z_smart_new)
   zsmartgenesis=$(z_smart_addr $zsmartgenesiskey)

   z_info

   sudo $zsmartdir/docker_testnet.sh 0 $zsmartgenesis &>> $zdatadir/smarttestnet.log
}

z_mainnet() {
   #zeniqd=$HOME/zeniq-core_build/src/zeniqd
   z_datadir
   z_zeniqd
   z_crss_at_height 1
   z_main_crss_from_to 101 106
   echo "type z_smartnet to start the smart network of 4 docker nodes"
}

##

z_smart_balance() {
   python3 -c 'from web3 import Web3; w3 = Web3(); print(f"{w3.eth.get_balance('"'"$1"'"')}")'
}

z_smart_chainid() {
   python3 -c 'from web3 import Web3; w3 = Web3(); print(f"{w3.eth.chain_id}")'
}

z_smart_spend() {
   python3 -c 'from web3 import Web3
import os
w3 = Web3()
frm="'''${3:-$zsmartaddr}'''"
chainId='''$(z_smart_chainid)'''
tx = w3.eth.account.sign_transaction({
"from": frm,
"to": "'''$2'''",
"value": '''$1''',
"nonce": w3.eth.get_transaction_count(frm),
"gas": 21000,
"gasPrice": w3.eth.gas_price,
"chainId": chainId
}, "'''${4:-$zsmartaddrkey}'''")
print(f"0x{w3.eth.send_raw_transaction(tx.rawTransaction).hex()}")
'
}

z_smart_new() {
   python3 -c 'import eth_account as ea;print(f"{ea.Account.create().key.hex()}")'
}

z_crss_at_height(){
   zbh=$(z_do getblockhash $1)
   echo $zbh
   ztxh=$(z_do getblock $zbh | jq -r '.tx[0]')
   echo $ztxh
   zvin=$(z_do gettxout $ztxh 0 | jq -r '.value')
   echo $zvin
   zfee=0.000002300
   znValue=$(python3 -c "print(f'{(($zvin-$zfee)*100000000):.0f}')")
   echo $znValue
   zcrssdata=$(python3 -c "import struct;print((b'crss'+struct.pack('<Q',int($znValue))+b'mylabel').hex())")
   echo $zcrssdata
   zrawtx=$(z_do createrawtransaction '''[{"txid":"'''$ztxh'''","vout":0,"sequence":4294967295}]''' '''[{"data":"'''$zcrssdata'''"}]''' 1)
   echo $zrawtx
   zsignedtx=$(z_do signrawtransactionwithwallet $zrawtx | jq -r '.hex')
   echo $zsignedtx
   zsenttx=$(z_do sendrawtransaction $zsignedtx)
   echo $zsenttx
   # mine another block to include the tx
   z_do -regtest generatetoaddress 1 $zaddr
}

z_smartkey_from_main_pk() {
   python3 -c 'import base58
import ecdsa
privkey="'''$(z_do dumpprivkey $1)'''"
privkey_bin = base58.b58decode_check(privkey.encode())[1:-1]
sk=ecdsa.SigningKey.from_string(privkey_bin, ecdsa.SECP256k1)
print(sk.to_string().hex())
'
}

z_smart_addr() {
   python3 -c 'import eth_account as ea;print(f"{ea.Account.from_key('"'"$1"'"').address}")'
}

z_main_crss_from_to(){
   curl -X POST --data-binary '{"jsonrpc":"1.0","id":"zeniqsmart","method":"crosschain","params":["'''$1'''","'''$2'''"]}' -H "Content-Type: application/json" http://zeniq:zeniq123@127.0.0.1:57319
   #found
   z_do crosschain $1 $2
   #found
   z_do crosschain $1 $2 6d7969646d7972657374
   #found
   z_do crosschain $1 $2 6d79
   #found
   z_do crosschain $1 $2 6666
   #empty
   z_do crosschain $1 $2 6d7969646d797265737477
   #empty
}

z_main_txblock(){
   z_do getblock $(z_do gettransaction ${1:-$zsenttx} | jq -r '.blockhash')
}

z_main_height(){
   z_do getblockcount
}

z_smart_height(){
   python3 -c 'from web3 import Web3; w3 = Web3(); print(f"{w3.eth.get_block_number()}")'
}



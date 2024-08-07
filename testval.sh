#!/usrenv bash

: '

Note different service names on testval{1,2,3}:
zmain
zsmart

Constants:
- zaddrkey,zaddr on mainnet
- zsmartgenesiskey, zsmartgenesis on smartnet
- zsmartaddrkey,zsmartaddr on smartnet

Usage for manual testing:

. ./testval.sh

z_install_tools_ubuntu # requires: jq and some python3 packages

z_stop # if on a successive time

z_mainnet # optional if on successive time

z_start

z_main_100

zfirstmain=$(($(z_main_height)+2))
export CCRPCEPOCHS=[[$zfirstmain,6,7200]]
z_smartnet # this clears data and starts smartnet anew

z_mk_key
z_crosschain_spend_height $(($(z_main_height)-101))

echo "$(z_smart_height) WAIT about until $((6*10*60+7200))"

z_smart_balance $zsmartaddr

z_ccrpcinfos

zsmartaddrnewkey=$(z_smart_new)
zsmartaddrnew=$(z_smart_addr $zsmartaddrnewkey)

z_smart_spend 20000000000000000000 $zsmartaddrnew
z_smart_balance $zsmartaddrnew

'

z_install_tools_ubuntu(){
   apt install jq
   apt install python3-pip
   pip3 install base58 ecdsa eth_account web3
}

z_do(){
   zeniq-cli -regtest "$@"
}

z_mk_key(){
   zaddr="miGjG7bavFSYuhJAt4MvLCehqcx2osqDFc"
   zaddrkey="cRcgFjav8EBFAGxyz9dLrtjsnNV8aJyKBHLMYSuyDMYDNFso2rJa"
   zsmartaddrkey=$(z_smartkey_from_main_pk $zaddrkey)
   zsmartaddr=$(z_smart_addr $zsmartaddrkey)
}

z_mainnet(){
   if ! [ -d ~/.zeniq_backup ]; then
      mv ~/.zeniq ~/.zeniq_backup
   fi
   rm -rf ~/.zeniq
   mkdir -p ~/.zeniq
   thisipend=$(ip a | grep /16 | cut -d '/' -f 1 | cut -d '9' -f 2)
   other1=$((94+(((thisipend-4)+1) % 3)))
   other2=$((94+(((thisipend-4)+2) % 3)))
   cat > ~/.zeniq/zeniq.conf << EOF
server=1

rpcport=57319
rpcbind=0.0.0.0
rpcallowip=127.0.0.1
rpcallowip=172.16.30.0/24
rpcuser=zeniq
rpcpassword=zeniq123

[regtest]

server=1
regtest=1
rpcport=57319
rpcbind=0.0.0.0
rpcallowip=127.0.0.1
rpcallowip=172.16.30.0/24
rpcuser=zeniq
rpcpassword=zeniq123
addnode=172.16.30.${other1}
addnode=172.16.30.${other2}
whitelist=172.16.30.0/24

EOF
   cat > /etc/systemd/system/zmain.service << EOF
[Unit]
Description="Zeniq CoreTest Testnet"

[Service]
ExecStart=/bin/zeniqd -regtest -txindex -printtoconsole
WorkingDirectory=/root/
User=root
Restart=always
RestartSec=10
SyslogIdentifier=Zeniq-CoreTest-Testnet

[Install]
WantedBy=multi-user.target
EOF
   systemctl daemon-reload
}

z_main_100() {
   for i in $(seq 1 100); do
      z_do -regtest generatetoaddress 1 $zaddr
      echo "$i of 100 to fulfill COINBASE_MATURITY = 100"
      sleep 1
   done
}

z_smartnet() {

   systemctl stop zsmart &>/dev/null

   if ! [ -d ~/.zeniqsmartd_backup ]; then
      mv ~/.zeniqsmartd ~/.zeniqsmartd_backup
   fi

   thisipend=$(ip a | grep /16 | cut -d '/' -f 1 | cut -d '9' -f 2)
   # testnodes_hex_dump at the end of this file was produced with:
   # zeniqsmartd testnet --v 3 --o ~/testnodes --populate-persistent-peers --starting-ip-address 172.16.30.94
   # tar czf - testnodes | xxd -ps -
   # rm -rf ~/testnodes
   rm -rf ~/.zeniqsmartd
   mv ~/testnodes/node$((thisipend-4)) ~/.zeniqsmartd
   rm -rf ~/testnodes

   #zsmartgenesiskey=$(z_smart_new)
   #zsmartgenesis=$(z_smart_addr $zsmartgenesiskey)
   ## needs to be the same in all nodes:
   zsmartgenesiskey="0xe127f1fddebd3218eabb5b3e41ffc55db9a526555a8d99b263fb73c9c5deaf2c"
   zsmartgenesis="0x53CB74974D4CddEF438DE77B13F18Eb3FA6309E8"
   sed -i "s/0xf96ae03f3637e3195ebcb85f0043052338196e57/$zsmartgenesis/g" ~/.zeniqsmartd/config/genesis.json
   zepochs=${CCRPCEPOCHS:-"[[1024,6,7200]]"} #needs to be the same in all nodes:
   sed -i "s/cc-rpc-epochs.*/cc-rpc-epochs = $zepochs/g" ~/.zeniqsmartd/config/app.toml
   sed -i "s/cc-rpc-fork-block.*/cc-rpc-fork-block = 0/g" ~/.zeniqsmartd/config/app.toml
   sed -i "s/mainnet-rpc-url.*/mainnet-rpc-url = \"http:\/\/127.0.0.1:57319\"/g" ~/.zeniqsmartd/config/app.toml

   cat > /etc/systemd/system/zsmart.service << EOF
[Unit]
Description="Zeniq SmartTest"
After=zmain.service
Wants=zmain.service

[Service]
ExecStart=/usr/bin/zeniqsmartd start --testing
WorkingDirectory=/root/
User=root
Restart=always
RestartSec=10
SyslogIdentifier=Zeniq-SmartTest
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF
   systemctl daemon-reload
   systemctl start zsmart
   z_info
}

z_info(){
   cat > ~/zinfo.sh << EOF
zsmartgenesiskey=$zsmartgenesiskey
zsmartgenesis=$zsmartgenesis
zepochs=$zepochs
zsmartaddrkey=$zsmartaddrkey
zsmartaddr=$zsmartaddr
zgenpid=$zgenpid
zaddr=$zaddr
zaddrkey=$zaddrkey
CCRPCEPOCHS=$CCRPCEPOCHS
EOF
   echo ". ~/zinfo.sh to work on another terminal"
}

z_stop(){
   if [ -e ~/zinfo.sh ]; then
      source ~/zinfo.sh
   fi
   systemctl stop zsmart &>/dev/null
   systemctl stop zmain &>/dev/null
   kill -9 $zgenpid &>/dev/null
}

z_start(){
   systemctl start zmain

   sleep 3
   # zaddr=$(z_do getnewaddress)
   # echo $zaddr
   # zaddrkey=$(z_do dumpprivkey $zaddr)
   # echo $zaddrkey
   z_mk_key
   z_do importprivkey $zaddrkey # all nodes should have the miner wallet
   # (1+59)/2/3 == 10 # ie a mainnet block every 10  min on the everage
   (while true; do z_do generatetoaddress 1 $zaddr &>/dev/null; sleep $((60*(1+RANDOM%59))); done) &
   zgenpid=$!
}

z_log_clear(){
   journalctl --rotate &>/dev/null
   journalctl --vacuum-time=1s  &>/dev/null
   journalctl --vacuum-time=1w  &>/dev/null
}

z_main_log(){
   journalctl -e -u zmain --since "${1:-1} hour ago"
}

z_smart_log(){
   journalctl -e -u zsmart --since "${1:-1} hour ago"
}

## these also in crosschaintest.sh

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

z_crosschain_spend_height(){
   zbh=$(z_do getblockhash $1)
   echo $zbh
   ztxh=$(z_do getblock $zbh | jq -r '.tx[0]')
   echo $ztxh
   zvin=$(z_do gettxout $ztxh 0 | jq -r '.value')
   echo $zvin
   zfee=0.000002300
   znValue=$(python3 -c "print(f'{(($zvin-$zfee)*100000000):.0f}')")
   echo $znValue
   zcrssdata=$(python3 -c "import struct;print((b'crss'+struct.pack('<Q',int($znValue))+b\"$2\").hex())")
   echo $zcrssdata
   zrawtx=$(z_do createrawtransaction '''[{"txid":"'''$ztxh'''","vout":0,"sequence":4294967295}]''' '''[{"data":"'''$zcrssdata'''"}]''' 1)
   echo $zrawtx
   zsignedtx=$(z_do signrawtransactionwithwallet $zrawtx | jq -r '.hex')
   echo $zsignedtx
   zsenttx=$(z_do sendrawtransaction $zsignedtx)
   echo $zsenttx
   # mine another block to include the tx
   z_do generatetoaddress 1 $zaddr
   echo "Spent $1"
}

z_smartkey_from_main_pk() {
   python3 -c 'import base58
import ecdsa
privkey="'''$1'''"
privkey_bin = base58.b58decode_check(privkey.encode())[1:-1]
sk=ecdsa.SigningKey.from_string(privkey_bin, ecdsa.SECP256k1)
print(sk.to_string().hex())
'
}

z_smart_addr() {
python3 -c 'import eth_account as ea;print(f"{ea.Account.from_key('"'"$1"'"').address}")'
}

z_main_crosschain(){
z_do crosschain $1 $2
}

z_main_txblock(){
z_do getblock $(z_do gettransaction ${1:-$zsenttx} | jq -r '.blockhash')
}

z_main_block(){
z_do getblock $(z_do getblockhash ${1:-$(z_main_height)})
}

z_main_tx(){
z_do decoderawtransaction $(z_do getrawtransaction ${1:-$zsenttx})
}

z_main_height(){
z_do getblockcount
}

z_smart_height(){
python3 -c 'from web3 import Web3; w3 = Web3(); print(f"{w3.eth.get_block_number()}")'
}

z_smart_block(){
python3 -c 'from web3 import Web3; w3 = Web3(); print(Web3.to_json(w3.eth.get_block(hex('''${1:-$(z_smart_height)}'''))))'
}

z_ccrpcinfos(){
curl -X POST --data '{"jsonrpc":"2.0","method":"zeniq_crosschainInfo","params":[0, '''$(z_smart_height)'''],"id":1}' -H "Content-Type: application/json" 127.0.0.1:8545
}

##

testnodes_hex_dump='1f8b0800000000000003ec3b6973dbc692f9cc5f31c5bc5a4b8948e206e9
5a57ad4e5bb675d892632729176b000c48442080c5418af1f3fef6edee19
802075f8cecbdb35ec9248cc4c4fdfd78c4a5194491a8862f0c3377b3478
5cdba6dff06cfea6cfba653ba6eeb88e61fca0e9ba03af98fded505a3d55
51f29cb11ff2342def9bf7a1f17fd3a76ce48f3ff56fa2059f2e7fc3d19c
eff2ff2b9e4df9fb69124693afab069f2e7f4bb3cdeff2ff2b9e3be43f11
8928a2a2ff4791265fbc070ad8b1acbbe46fdb96bd217fc7d2ad1f98f615
e8fbe0f3ff5cfeef3a8c7595b4c7653413dd87ac6b6886d5d38c9e6e5dea
ce435b7f686a7d4b778c91eb0c9ddfba3bb8c69ff228194701ced7ae35d3
d9b347fbb6e6e8fb723c4aa232e2f1782aa2c9b4a4596a619a142229aa62
9cf19ccf0a18422460c48b53ffaaf90a2f66fc7aec2d41450929c04ad346
0e416986279c067bfaea3552318ed2928f097857071debd2e07b39a72be6
5120125f6ceec527629c54b33121d2acd5b4f52d715650e5bc8cd284e6b8
c6b0e5cad62737e8eb9a35b45d671d91398fa3809769dec624abbcf19558
8ecb65464b7f5703887860d8b63eeaaa376fd78189bc9028bd7bdf51af57
3bac20351bf120c84541d8692670d71ddafb8e75a0edbb7b2343dfdd3f3a
dcddb59cc3a133b24dcd39324d6db8224e61d9421c590f2823b8522481c8
6751520ece2bef99581e2ac477567301b38a26cfdde55ebcb8dc7da2cd9f
073aec3d3f2aaf5e3967bf9c0af126baf8edf5c5b3f8fc5532d83b583caa
297fbfc2235d889c18bcc22de1528fd1a5ad8bfe36da8d03fd7068ef9ada
687ff7e8c8d8b33420d9d1ac91615a7bbb43fbd0d873b58383d1b7a0fdf5
d5c19195ec27cef2dc1b8e06167f1107c298ce5dffb7b3e2f0e9b353f7b9
fdcbcfaf0f39d73e8376fd83b4efbb4747d6d01c998e39dc072538dc3f3c
72f0dfc1687f6fa41f198743cb1dbadab7a0dd087cf3e2d7277f9eccf88b
e5a48a97e2e06975f1f3bc7c611fedbf3efce3cdb37837e6e765b11c7e06
ed86a21d7ebe254be059369ef2628a13bacd1bf0fee5ca0fdc662eac4d68
db64ae6ba3f11d2bd07cd703a3e17e2838b71ca18c265c331ac93ec93d58
ef859e3fd447ae66f99a1e5a9eefb9ae2d7c3d7442dd0946ba2586c2b602
77246c6158a3913d320c6e9b81350a43cd32fd36dc5c2c781e8ccbf4f351
9ba765944cc63563f5d61008344f83caaf5d9e34acd60460e39508c67e1a
253573bef0694387d0948b32ca013d001ef2b810ad515037918f3fc7996d
2ad55d8236025d0c6d0e1ec2e761687896261a0fe1f12148c773b52018dd
25689b5b9a0fb35c8dbbc233ccc00bcd200c01176d6407ae0751351401c8
4a789a6558dc743d231cd9aeb0879633e2c13d82fe1cd43e51d0fadf56d0
1fedb93f56d0be1b86b53bf4418d842fc018e15f30f2bd911e1a62c31d6e
083a1805aee95aa3a10ed66538205f9f6b1eb7fd91d0fc115861c843cfd6
84654322a31ba6edf391053ac04d1178a611de23e8cf41ed13056dfc6d05
fdd161aa1634fd7eab12231e433ed7ceb0b4eb70e470a19921007485a98f
6c01de7768871af855cd364c13fcb2236c773dcc793ce6326d0410ba6e0b
e15b6ee8f821c0b015916b18501ed679ffaf4ef2ef79eea8ffb23c9a8f9b
608801ff0b4a41e48b73b3ee6bea7fdbd4d7eb3f43734ce37bfdf7573c54
ff7d6e327c2319fcd844f0f3136059d5907a7e70639874cfcefc69f8f4f8
d0b233737295ff7cb9c7bdf0e899b6f7e78ba7bfbde4eec134fc35393e7e
fcfa2438b4f98b933791ff2ccc8f47f33fcdc134dbff4577f9e341797de5
2c5e5ca566318fdfd8bfbeccf2178f24967f6b9b6f3f77d83f7ef942ab5f
3d1fb27fc3b036fb3fe812bedbff5ff0bc6bd9d23b6545f719516d3fdd57
07be7871b6d0cfcdc585f1eca571fa643abfda7776cbe16990444e6cd987
af8cc2bf387f35fdf9d7372ff7477e661d549a77641e5dbcbc7abd78f6f3
8b27d91fcee8e0f5d49f5e7ad72795e70d77c17cdeffbb98ceff89e70efb
97bffa653a8bbf7c0fb2ff3bfbbfa60d96bf69ffc677fbff6b9e1fd9e534
2a18fce7ecf2ece43993926761148b7ee7477694e66c96e682454998e633
6a7aeeb04208362dcbac7838184ca2725a797d3f9d0d505d7a90224fe853
07969f9e5d1e3e64bbc99265bc9c324fc4e982f93c814f8c7b451a57a560
5ba23fe9b3ee60cef3c16cc917a248678267d900924fdedd66690e907211
c3e673c1ca949553d81ee6b020ca850f09eab2862157f481a81b33a202a0
74fff1e4ece470d05f79b82ef3962c1021afe272877955091ca8e20011f4
a7408a08d83ce2ec1f9727b8928964ce00cf887bb10078c09d5e8f36f267
010b633ee903d95fe74138ece673c2a384edf142b07d29aab30c8552dc32
151e05e7abe003f45eee9f23cdaf4e8fdfb022f5af44c954f2c8d290e4b2
bbb77fcc407871e44b65915cc2216ccfe13410ffe62c60fa2c03950b40cf
d802348a165c3652625e94f07cd9c9f2f47a398685ec11e47a7e06faa71b
6e5f837ffa4328b8ed6117d1dc653ed8753a63d36a06bbe5820728308941
48e880c6a3c7ebccd224ba1239c28324d7d2f6cc83a30377cfb4867b04ea
385c4d463301784b260f084047a6511210aa6594d51ca093911d76c48bf2
6299f800030bd045818333545f20d99f5619fbef2af2afe2252960ba48e2
9407500fd7c08111784012c7224610b0cf5ce451b8c4290029ca9167b3a8
2c3a21ec342e602b20a2cc2b81681f802178a8241e072925c14336496331
1771e0b17f327ff5d14be3923ee4b8ab7cc58389c8030fc0fcd45ab6d5b2
74d82d28f3c16ab0075ea228599666550cfe349a65b19889a424e96e7750
917b2cabc0914c52f50d1caf34a29f56f86c558528187c8b26295be42068
91d7ab914cf53117c03bd03a36f17df50a16aec0785504265cf209209daa
6f3df85a3453b6695f45bcdc55947ef0a040f5b84251e218c06d118d137a
513af070a8c6eaf0cdf9e1cbe393c3d3cbdde7ead58c2fd17d20bea058a0
33e8cf10c19ecf71a3ad1ca409ca896a59c07cd02171bda213295188dd4d
879c20a9a82527c968fb63e17b6909525233eec1f90e8ed6a0ef46a405fa
a746736ea2124c409853e21ecdb90715a2bf06740f07d494ed4ee08d9596
a315373ad95db3832610e074f88253295ae0acb3aaccc0f3d342f20f713a
9980a1ed806cfcb822b3cc600b3e116a522a9d6e07268ee59b47d8540bd3
363c19311fb207590c2ee101dbf2d3186229d023ae4b8c6bec01d6570f08
8a9c8c60687657c511f4af11eec5634684a800ad10608d6b3ec700ab82e3
d38bb3530ae138bb0468ca6930752acb9ad60e84f2929c4b0ac310ea45c9
19b2a5539f09139447746cbb7931a0fbd1db628dc121d24399817351c21c
338e151ae0edc877d687c3b0242d53e057e7662b6a13a73b9a551f8f5e0c
96ca8a6892303a0ea218b5c26d13039ad3e080ccdac48066ac70b82768a2
aeb5a21c201a47e03512062131a4bc07504d842f651de6e035209e81f688
1c1502abb35f1a1e02cb7c00ba896f8c9b21aa5f242fc4946220af604652
d6815b092e33b295c8eaeec1a6a0d6ba0a84cd89c0242b2a645c94a4d6d8
6de6080f6be6fd934df2ccef70cf8f10b87cdb046b08803b105a05a47c6d
288c920cc94ba43205112762c1322190cd85dc1327627a1a083fc2781fb2
8560c59432c22b21b25a4995481831a5ec009d209131c22a0025ea6777be
6d22b81bccb1171da82c50dd47b83b19fcf1ab26825f4cc8cbf37d762172
48693e44c01720def91df4e4ed4798204a15312a24466d33ec34e6735bba
e9aa7413a7a3db48f308e206ba363f4f8ba207611ed3758cae5068d7950f
447b1f0a9f800c1a83942c3f18f556d883dfdf3e807055607604094b0a2a
55545996e698fdbc2a7042f7a72ecc411dc6cc92614a2a77eee0f431bd15
c1b8c6e611fbfded3a9ee0e7a7695048758e23b0672a01e5badae4290fbf
8d8e627d9b1a186cd37d72b87bd0dd61ddc78797f8ebfcec027f6fec9e80
9c0b4a11d9147220349aaf82480d0c113923da11875ddf1759899f40d340
a865ef12db5cf0fd4defa5842382de6bd843be936ad9bbc4cb500af58f50
9fc95dfa5317c15467ab09690269bf126ac1065e0ea9bf8f597c793d9659
7d075ddc86eb3ee1d7d1ac9ab1a49a790824442682def044a455d18e1458
b91fa402ab9652a53052c1b79e5c5e9effc76be15d1015db6b8bc02005c3
4b4b692692716b447ad6655ab10597518a134f41cd21d9872cac46a90467
4edc680aea19bf02070a793fc04000800ca4bd2053f892b3b30b6014d630
88b086b95f42df45802f2ea67525fe9fec5d4503ac7791bc8779ef8017a7
d5ec38f1d22a09ced1efb65e43fab5f1fe14b9b5e0f10e839c7295ee20a1
14008bf7b09fae19164cb610111d7fd81af01e8a42f8f93f234d9332b98d
41380dc651c9e10585ce2a297828998e12853da1ae814a930d02bc165708
012f1093819c390ee3aa98822dcdb21442a85adec4924f913edb5a65ad2b
51dfaa131389df860edc49e77745908a70bf0edc94549544e06794833b3e
4077c7653418149557f879e48986b70f722c165178b739861dcad8558a04
ae2b82a2013ce56c6d4fecb940057b7335ec19c7905c035ad466e813290a
078abc63892492a37f801c4cb22281c16e12cd817fca7faf9305a8de42d9
637246e03a9fa78091b4125abd2d6de21ec201da3ae93f6e10ffd9a41790
c0e58a7e20df46e29f40788d5399302e7844351d105c5ee30bec11126c44
23a8f2bb64863afc7af7e5e9f1e9e38710c1711a57c15e990dd98bae1510
eae218a25b81e94094d456a2f272003389530f18866e9c2d72301086d75d
d30ad8b39846fe5426cc2014991cacb905e42cd41a590aa50619dac51d6d
dcd5f14feb635414102a07a665da1db5e7f836363fc2bbaec55ac02aa23f
a9a8aad3202f0d96585a33ba1d4b72c057f2b2ac543c7cee032143fd0610
f9b20586aedc5200c71245553efc46c9e38bbc8c422c3384b44c88d49077
50fe81fca7d782987ea14238b2ef046f36533e179113691adab4131664f8
9b1ad712d6aad27b50d4757cd397e8d7cd46b1860f260d509902006f8969
656b082b3148f1cae58eb2091c3c42da8ac661aa7a05e72732c1565d4a49
06e2b182b8439924a007480a0853c0c61dd57bc425fbbbebd3fb4d6ee3a5
c8dab818e3a8acfa6811bc69cac0595510b332d06e34b09ba5af62f5269b
cf90b98ba800f448ed550e058cc9aba4df59dfb6ce95ee17f80c3bb0d4d4
6917baff4ac9aff3b18704f5da7cec0186bd6fcec776cd4e6ccca0a60feb
64b64e79b71a9f91e2614f3fcd2783ec0aca7b510e706840abb63bf46b3d
87fde2ba119f73e3fc23aade2faa1b332393358b227995d187d4ac025f47
8ea3951a6dd4891a55891a55894e7703160fe66846059d66c9ae81aa23a8
118219a2f406900d8279cba8809510592e1e6260e5803d34d9be02c8b5a1
d254badd5764d84c49651e26b1c7e09813a057e7c9397a8d94111bc1b756
a5ec80482cfb9dbac954dfc36b74621fd359d01e3c9d40f5aeab3a4c69a9
4554ac37733a32d7fdd0f26625f55932fc630644b95c0b61006d35d2f45b
ba7a108c82d07775776439ae615b6e681943d31c8d7cd3d3436b6885bea3
69e2bf74d7e8eb4edfd4fa234b8a66c7e543586838aeabdb1af7b8c935db
f1f5c03246a667d91c40097d38f2da6b6db556b8ae3db402ae8950f37d5d
0b7d7da80d8dd0b0dcc0d774c3e4bee55a96d65eebb454e2d5f9e9b91425
881faf7b825275aa2cc9da897fddb1abe5e0a5e95507bf8cf1d3668f0d07
f0fdaac7768169535ec9165e51e691bfaa5e73103bf7a21882087801487f
d57cda9c16d41e929ae298ab81952fd2fcaa68a1a080de5bac44324397da
4ed11affd246bd6d2469dd9171a62a9157c6b2a65f3b4c5c374dfa95daac
6f54036876d269a7e72de5639098ef509e2713a9769f8fac0a9cee562eb6
051d5c45c5144f2b27494a691f064e710dd0f08b2c65a086030875f39e36
1e472d43a8a9cc38f538a602db1a68fa323fdc20058aba90fd297222374b
136cc1d291807f9586611db7b66f9807d5715472c24894d211894acfb0c3
d1e4b59e08f1989faa500a91a01e7c0216898e214d361a9f1d59ad96d33c
2dcb588c554228933f6d767bfac76ba07496025a96f1251e7a6e64717270
3c2b266335614c101e515d86805f52f6532a41c9f945dd5cc3a0b8823828
040a017c10c81e1d0e26f6ba51a797f74282f02ca0b0096e428391f92dd0
1a4b03a642ba8527ced41b0739f44047e93a019efaf974aa20ae5be7b517
e83c67a085b499448794124f44a0c62fe325c475be88a5cb5746481e3f06
132cd4690189bc8f918327b2b2953dfb020f9455f1224d1f523b7c031f41
99542b22880a25622c0fdafd02da2c0aef26071550f52ca164478f3f466a
da3ee12ec74fda4d4571edfb95cbd922abc3ed3d3c362e8a2893d994a48c
68ddeea8d937edeb329d4c6221e32921c62615b858c627900516ca41b4cf
04e878a589b051d6ef5063711c54f20c428ca3ac25326c2ab47d84df4e49
fa9d29b67ca6fc6acd380c343db2c6d64bb3f85aa9d189ec1edd9a1d7da5
cc4835a8203b0223980affaae648530bd62f1640a53a68bda37d59e63c29
b80aedea20a96e802993b795693da7a60fddb4484bf07b8d4f010d29af37
57f73bea5e13355ab9ef83eb2f658e0536b4beafbc39243b18fac9defa20
c8b043fd0528700b59583eb24ff676ea8d6460a877c14e98bd06605b7619
9ac5e4c4f04f44f4a17465178a0e72af1c184a47e7814cccf050691d1f3c
8d020204cfa1c8cfb789ecd6788740acdca562de414a56948b593ac73360
3a1b5c07dce8be4242f5edd4df666cd7ce2d95fe0d3c4184950c42cdd030
d1bac8b950fd7803be6c96f8780d82c63057452bacc5165625e4a0fd0e9a
7f4fadef01d3e0734f22744b6eb10a2bd81f414b5f6db82aa9482bf8756b
36f66e70a6eadba4d4b16ff954509b7752685266ef6b19ded658b8898c87
b5e50ded0606600892e5a83a733c6ebab405441e444004b2ce179039d0c0
16047aba6f4659c09a325043f7cd9b370fd9ab8454269081e7531b3ab63b
72640b06116f68d4be8e4bbaa00375bc07757fc9f6453e89cedbf102d45b
d453da91ae43e53c8b02bc6095a625e4a83c2bd4a12f0545bc7685010faf
55e1558f50c8a6c00ec542088ca54aedd4ad8019c7616cc21509809a82ee
93d5c838822145f000055f035270b298d3a52df0470811736879c5abcf5e
d6b76ee886500d4875736a7d44a3c3e63942a39a7d1d9f061b6cdeaf688f
a47572d071a8213155954e862ebdfdc8a678ff028c55e6f412e0d6f3ff6d
ef4a97da56b6f5f97bfc142e52754f5260d03cec5bd955788004c8400881
249572c9520b1c6ccbb16c83b32bb7ce83dcfb72e749ee5aab5bb26c70c2
b41df6ce6a2a01493d0fabd7d4fd01f5ae62d59ed1a1fdf2ef654df95252
8711ad3b0da002019201522a45b22daa71a4894287dad9fd54de01906b9e
547d41c4943c52616d4f8d68297acb00c350c9198627d23387b2524a5af2
88cb3c11b2ec2764169fed2349c70702041501d485c4558a80ee1b53df86
7ca2f4a1feebe5cd4e9ad0aa4c658bd15098350647234c068a7f42b9ed91
524a62df020168a1b249d9780bc9d3643408859c63c4e4234fae74c792ed
9b9e41ce18519556f264e8915b4bd26e92960feabb15f4098aa4cf214a2f
1833e3f4951670948e60979c48975b64398c0d33ebac11c85f547b522397
1fff8f01259c0b71963e99ad14449e64d589dbbda017b629d3fea82725a1
c76927c03fe42875db694bc054692783f5125a93b241256640d652752492
1af502fbad10612ab0e88e77aa75e7c496b42f329e552ee1e94ac8a419e9
ee3494ab58ae68d865b2241362c2287f5b658dbbfa20184c0a7ec3526ace
d755bef6c3d351ef0cba5ced94696693797550c625879ccfe3e1a48feb1d
fa6963d8ed13c53ec245a43474448cd6cad21d1022a17b6a54281aeddee8
4a2ae988dcbc8752528c6063582f6141451eebce041b9d56afa2d6b5820e
e61e0836ba464a7a8d133a2f53dd60a1ccfebf913fa0fea4bc32d656728e
e449b922155ae2240827e464a908de8ccb29a53530adbe42ee8db1145360
de8fb5bc201cdb9618129b45623d6942a6690d4c8baec91d3144512712e4
254699ac910b5e17f6fd482aefa61994ff4bb91ccbecb2d29e524bee675f
ade58e72dfd784de6998726f3c1827e4e6677cdec2741dde6dc0bf951943
19b0a80543597f900083986d79d9baec0f843c9b5ceeb53bb93549c61599
4824f3ec8e90faccc5c88cc7a9740d0111f414555940e4e7623623d10102
0f39da522d7145458318c75f0afb58a555a48fb28642ca0cfff9f7ffc25e
896bf1e43ffffebff2e3b658979b6bc679cac6e12ed2ee3c29d481f22002
33d79e53519e8f759d3651ccbbb449b98cdfb155054bdf77dba5e25dab65
32eef5dba60caf92b4534dd7b2d99533204a6d0544963667da6e1e93373f
8a7ba831432ddf29fad121cd51fa1ec98c75d53c553db656162020126796
8c4e4eb132410717f9447244d8c140dfb306cdf650d64585a30368504892
33d2df913a934469d220e2cd44d9fe8c5c89b418a9d52e27a56ae8e744d9
2fb3cfb8bfe0eed04b7a15a92b1cce306ffda0d70ecb23e05868370cc8a5
4c3186a4f498168416b1f3606a0d23c2f747948c80636be24393aaac76f1
6fd28756f1b5c88543d9054646b122b0014b17ca19ee6c6d4a30a8eb652e
6b99dd31afebe594244e8d131036a965b0b52fa89f1268405c3bc3514d4e
48fb1d20e30d7d1190602d395bd4279c1647befc183e431fbd95435bcb46
567b524acfdafde6a521cf99d9061a6faa72bc4919860dcac56532b3222f
0a1b10705c3d69eb518d2f490ea149efd47d54995ee58a4fcd3cb3a976f7
8d52cca51dd4a9651756d199922eec6683b4441a33a9576b52acfc5aab82
1e9722913b6db31b7c36cc2b621a77d75fbd2de80a9ecbe310dfd9d8eeb4
a9811c4f072e88f738229baf2ab0e0e93ca35479a4ecc9c5f34ab49cd021
45b1c717b4a429a77568819cc8f2b84720176002ec2cb96967b60475fa05
f593d2dbf8725665c9ddcf2b17a960a53a29544aca07aaa3a6dc536fd4e9
ace43ccdd9f8322f257d31d36171725297acc9e343a4934003b4f4594136
1ab5f88f8bac2f1d82a857ff9bce06d6ab5579208358de7ffeb322c91215
0ed4373c85ad191e8717ebea7a385a1af48c3734c92e0a3ae7c104699daa
0dc83fd9483da5acee83937a0e62fb6094b38d7fd6a46bcf16a3e69ee829
77f5d720230a1809a0bb5d945143983503e5a301021caa6cca1bd917626e
a729f668cb40e332f6758df691cc981b2561a16d99ab6aa6780fc641bb43
12b8ca7a1dcfd765152910b2abade0854a8749a74332cbe374c6a5b4905f
53a66c665672b4823ada2dfc5a97ef80b8c0dbcfa4130773f3074538d2e6
95f2bfc827607adeb4f4b38ffcce8405e7bf81aadcd3e1ef7ffcf0fe4fd3
71e7ef7fb06d8bef7f594af893cf7fc3da6f9e88e15e729292167e2086a3
41af0cebaa9b96e043b3035fc8442d7d1ed3a2d90222a38d607a1ef6ed8b
927c57c939a28a4ec5a03647c5cf79a18c6d52bfd4a10bc8247bd31f8c7a
a2b42055eefe8a2c0188f1a25e2dc19f4dfc336a15a963cedaa3fcd0cb98
e04c597686d4a98dce53c049a2b70f164a9449c553de06e882d3890a94ab
fba4a4983bcca21944337d73bd42f124e61dcaa476ca1e46fe07ca815e26
5b9a2496b9ca15d8ed002d38d27a94b9a2d283b457ab5ae611d37f298947
0d7b09be34678c57b951fd6de1d862b1068aff4145f4d96cb9ad511c6767
4aa48face4bd886eab2a4fc59512e570a970daa1317b3a0137edc1a41365
bd885b288c8a9c7cd93c2d51d426bd6cf6724713e5d959b07d4ead957326
3e69fb477f80ccc20aabb3e8274c12ecf014baee146ab3465255491e1445
8d8a389f37ac420332b3aacab2992757930aebf841f4da5fca78bea607fc
e5a00f92e2a05352cf1578aec0736ecebd22768a0e635d319b44bdc4745f
31c982c4fd204dcf93413493387b9927d60d93d2d343da45a150d5935812
d14fa0ef80ae94a6112e559c7cfdf3984a932ca2513faf8f547cd031fd35
e9fa408a8b9569a64dd46943a62ba5738c2506952c8fcc3f0037561c8e9c
50c483a4376c43cc932095ab475ab7a9f38301c8b3635199f599c0522ba8
dc2c129c511f646a5149e20a4cbe0ad0cffc939c8015752b819278b333c8
1f3faad649dbceef4f75cfb21c6b4dbd9562eaef4f9d4f6bebebeb684723
aa8f0b0ebdc9f15e8c000f48e3b249c808d64b64149a9268a6948b13fa42
a0ad979c9a946a0166e2088d2e3d7267ee202b99654ca9314dae4847091f
d87ef50405a97397e808dd45f13c9a392d9731b5433a0ef6a81c8634e034
be7412eca36c68d9f9f4a9fc083862e5d142ec20599840d89e4f535689d6
2055f953f61d6f06903b0f9d06314cd335807bf16ccb752d4f734bb996a9
3210e3b6d2f97efca895dd4f90a325033c2cacc815a9cb1fb53548b20619
b832ac95751d6b752f8ce43cff47aaddfbc8b810708adfecfe7fd3d65dbe
ff7f19e1caf15f789cfd7665fce8fe37cd762edfffa831ffbf8c40f73f5e
baa39fb4f3f0acd113c8ed7d7cf8cb5c6ac8e1da6176fd1b0f07ff87e9ff
52c2fcf83f94fddf60fcafa5842bc7ff21ecff7cffe35202efffbf76985f
ff0f06ffcd62fabf8cb060fc7fbefdc764fabf8cc0f61fb6ffb0fd87ed3f
6cff61fb0fdb7f7e75fb8ff170f01fd8ff672981f11f18ffe1baf529fdcd
f11faa55dfd48d865fdbb41bb55a4367fc07c67f60fc07c67f60fc07c67f
60fc07c67f60fc07551fc67f60fc07c67f60fc875fedda7fc67fe089c0f8
0f8cffc0f80f8cff5062fc07c67f60fc07c67f60fc07c67f60fc07c67f60
fc07c67f60fc07c67f60fc07c67f60fc07c67f60fc070c8cffc0f80f8cff
c0f80f8cffc0f80f5382cdf80f771b26c67f60fc07c67f60fc07c67f60fc
07c67fb8d9a6c6f80f8cfff088f11f18ff81f11ffeb1f0fcf7822335b72b
e347f7bfd9a67ef9fe37beff672981ee7f532641bc00aee66e6d599ee99b
8ee9d534d368d41a5b0efed4fd5ad5d7b78c8667b99eabee89eb8f5a3837
2021e6032f86e859ff5b71be6fbc1eb576c5a41119b6adfb940ce2d14682
118d28340fde3ffbfaa21bec4f4e469d89a8ef8c0e56c7c37d7bab76d4f8
7cbcdbd9ec04af87e9c47bba0269bfc972717afeb06088f49d9227aff7de
8ebe78c1abceb3d197f1eee6d0d9b66b9d4ded60f3e844585d5db47676b7
0fbbfdeeabe30ffad768e7247e31f9d0afefd69ce31deb45d7de7356f5fa
d9e1cefb8397e197c3dabb64d5f8ba77fe54d6f22f7355de82f53f7352eb
ae65fc68fd1bc6fcfd5f8ecdf73f2c27fc51584b7fa855f4bd4594ad9f95
ed17fbefcd37c7d6cecbd686b56a0d27c7feb3cf5ff4da4e7763f595f1e6
28a955ebd1cec5ee6afdebf6e9c6a169bbfb67b095bc0fab62fbe059faf9
d0d2b78ff6ab6d67f865d5387fe977aa7d67b40fcbe7db5f65e9fc2dc282
f55f3ce67be7327e70ff9f6d5bf6fcfab7749bd7ff3202edffd9596f5465
d0a6ac195645332abaf556777eb3f5df4c6dddd21dc3771dcff920b77eb2
3835db11dd1a7ba1994ed5f66bb6e6e835f95d9d346f5eba5b3657313549
15914ef770d265e48ff082acd2688fa64a41ad34cd77d446ae3e9f04f4b1
a24f5f632b9aed64183429735464682bf4f19be201c418c4ed5e28e6cb02
a1969c05a552254bab69b34562ac4cfd41715cc32b5c653a1b39afbef219
98ad48ce61176ba298aa2612634cfab19429f3578422c3eacda7d9cca499
01b3fa96734a531d5c9e535e5081ed034e4f335ccfae39565dabb955dfd0
376b5b8dcd4dcb69788e6f9b9ab3659a9a376ddc3ceb7713f66f96111bbb
936ae7fcede6336dbc17e950f6786b7876e8bc7af75288e3f6c187a383dd
ceebc3de46b57efe346bf9b7693d927331a00e9ed60d452f7c85246d76e8
af6abb51d71b9ebd696a7e6d736bcba85a1a34d9d12cdf30adeaa667378c
aaabd5ebfe9fd1f6a3b3fa96d5abf59cc9eb96e76f58c17e07a8f0e9d80d
3fbc4a1b3bbb2fdd3dfbddea512308b45bb45dff61db6fc4eedf73db6fca
fadfaced866a3bfcff895642d0ef93851923ace46f489d3c2541572c9772
b1a1c52573912d9ad0b1222d745bb068823016416039422d9a7866d1c8ee
93bd07e95b712bf474dfd5ac50d363ab15b65cd716a11e3bb1ee44be6e09
4fd856e4fac21686e5fbb66f18816d46961fc79a6586c57c0702ddc79bc3
e4f6559316b966d6b17ae1139d2788466146f2e4c22a4448d143256a8649
bb9775ce1d433177d89a0662d8461bff8af2482b7c85e92606cddb10b3f9
49b568a08d48179e1d0085088338365a9626720ad10a3c189d96ab4591bf
68a0edc0d24288e56a812b5a8619b562338a63a88be6db91db825d351611
8c956869966105a6db3262df7685ed598e1f44df19e8db54ed8603ad3fd8
81be36e5beee40876e1c67e43084692442018b117e223f6cf97a6c883972
3837d0911fa173a7efe9b0ba0c07c6370cb4566087bed0421f56611cc42d
5b13960d8c8c6e987618f816cc81c01451cb34e2ef0cf46daa76c381361e
ec405f7b9bca069a7e7f528c117a7187450e4bbb887d27109a194386ae30
75df16407d3d3bd680ae6ab6619a40971d61bbb3db5c2be804926d842c74
dd1622b4dcd80963c8435de53a5b836f0f5e17342bff690f07ffc7e6fbff
9711e6c7ffa1e0ffe80e8fff32c295e3ff00f07f0c97f53fcb088cfff36b
87f9f5ff60f07f0ca6ffcb080bc6ffe7dbff18ff7529e1d6f6bf91bed33e
6b6d54c76f0e3fac1a83e8fdf3d5cfcff5f8a4b57771beab2561abdedfff
7cb0e5edbf3a3f3d6f987af775cf6b750f06f5fa6afbcde0223d3b71f67a
d6bb37c9f3f7a77bc7f571b7da1b3c3b61fbdf92c382f5fff3fd7f18ff71
2961deffe74686a07bf0ffb9a9f1e7fefc7fc2ad378d8e38dc1dbcf3ea8d
70a0ed6d1f7dde39683967ddcdf787f5b3f74e677b6cbdbbf0ab22d8183f
df3eaebed88c0fbca1799c3e33bc23e3cddbc43ed46d5becec743e3cff72
e81fc45ee3f5e65fdbff477b38f87fbcfe971218ff8ff1ff18ff8ff1ff18
ff8ff1ff18ffef57c7ffd31e8eff279fff584a60ffcf4266ecffc9fe9fec
ffc9fe9fecffc9fe9fecffc9fe9fecfff940069afd3fffacb040fefbf9f8
ef6cff5f4a60fc77c67fbf6e7d4a7f73fc7763cbd01daf5675b66aae57ad
b98cffcef8ef8cffcef8ef8cffcef8ef8cffcef8ef8cffaeeac3f8ef8cff
cef8ef8cfffeabc17e33fe3b4f04c67f67fc77c67f67fcf712e3bf33fe3b
e3bf33fe3be3bf33fe3be3bf33fe3be3bf33fe3be3bf33fe3be3bf33fe3b
e3bf33fe3be3bf6360fc77c67f67fc77c67f67fc77c67f9f126cc67fbfdb
3031fe3be3bf33fe3be3bf33fe3be3bf33fefbcd3635c67f67fcf7478cff
cef8ef8cffce8103070e1c7ed9f0ffba06d7cb00400100'
echo $testnodes_hex_dump | xxd -ps -r | tar xzf -

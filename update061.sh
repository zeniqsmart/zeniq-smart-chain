#!/usr/bin/env bash

: '
Instructions:

# make backup. In case "wrong AppHash" happens, ~/.zeniqsmartd/data needs to be restored.

# load this file

. ./update061.sh

# prepare new files in current directory

files_here

# copy new files to node

cp_to_node 172.16.30.152

# ssh to node

ssh root@172.16.30.152

# load this file in the node prompt

. ./update061.sh

# check that the node is accordinge expectations

check_node

# stop services

systemctl stop zeniqsmartd
ps aux | grep zeniqsmartd
systemctl stop zeniq
ps aux | grep zeniqd

# modify ~/.zeniqsmartd

moeing2zeniq
ll ~/.zeniqsmartd/data
cat ~/.zeniqsmartd/config/app.toml

newconfig
cat ~/.zeniqsmartd/config/app.toml

# swap the files

swp_zeniq
ls /bin/*.org

# start services

systemctl start zeniq
ps aux | grep zeniqd
systemctl start zeniqsmartd
ps aux | grep zeniqsmartd

# check the log output

journalctl -u zeniq --since "1 hour ago" | tail
journalctl -u zeniqsmartd --since "1 hour ago" | tail

'

files_here(){
[ -e ./zeniqsmartd ] || echo "missing zeniqsmartd   "
[ -e ./zeniqd      ] || echo "missing ./zeniqd      "
[ -e ./zeniq-cli   ] || echo "missing ./zeniq-cli   "
[ -e ./zeniq-tx    ] || echo "missing ./zeniq-tx    "
[ -e update061.sh  ] || echo "missing update061.sh  "
}

cp_to_node() {
scp -l 8192 ./{zeniqsmartd,zeniqd,zeniq-cli,zeniq-tx,update061.sh} root@$1:/root/
}

check_node(){
[ -e /usr/lib/x86_64-linux-gnu/libboost_filesystem.so.1.74.0 ] || echo "not proper libboost"
echo "zeniqd version"
./zeniqd --version | grep v24.0.6
systemctl status zeniq
echo "zeniqsmartd version"
./zeniqsmartd version | grep v0.6.1
systemctl status zeniqsmartd
}

moeing2zeniq(){
cp ~/.zeniqsmartd/config/config.toml ~/.zeniqsmartd/config/config.toml.org
sed -i ~/.zeniqsmartd/config/config.toml -e "s/^addr_book_strict.*/addr_book_strict = false/g"
cp ~/.zeniqsmartd/config/app.toml ~/.zeniqsmartd/config/app.toml.org
sed -i ~/.zeniqsmartd/config/app.toml -e "s/modb_data_path/db_data_path/g"
sed -i ~/.zeniqsmartd/config/app.toml -e "s/blocks_kept_modb/blocks_kept_db/g"
sed -i ~/.zeniqsmartd/config/app.toml -e "s/moeing//g"
mv ~/.zeniqsmartd/data/modb ~/.zeniqsmartd/data/db
}

newconfig(){
sed -e '/^blocks-behind\s*=.*/d' -e '$ablocks-behind = 0' -i ~/.zeniqsmartd/config/app.toml
sed -e '/^update-of-ads-log\s*=.*/d' -e '$aupdate-of-ads-log = false' -i ~/.zeniqsmartd/config/app.toml
sed -e '/^cc-rpc-fork-block\s*=.*/d' -e '$acc-rpc-fork-block = 9123456789123456789' -i ~/.zeniqsmartd/config/app.toml
sed -e '/^cc-rpc-epochs\s*=.*/d' -e '$acc-rpc-epochs = [[302400,1008,1209600]]' -i ~/.zeniqsmartd/config/app.toml
sed -e '/^height-revision\s*=.*/d' -e '$aheight-revision = [[0,7],[66123456,11]]' -i ~/.zeniqsmartd/config/app.toml
}

swp_zeniq() {
mv /bin/zeniqsmartd /bin/zeniqsmartd.org
mv /bin/zeniqd    /bin/zeniqd.org
mv /bin/zeniq-cli /bin/zeniq-cli.org
mv /bin/zeniq-tx   /bin/zeniq-tx.org
mv zeniqsmartd /bin/
mv zeniqd /bin/
mv zeniq-cli /bin/
mv zeniq-tx /bin/
}


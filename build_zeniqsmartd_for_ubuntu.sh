#!/usr/bin/env bash

: '

docker needs to run, else start it:

sudo systemctl start docker

git clone needs to be done beforehand:

mkdir smartzeniq
cd smartzeniq
git clone --recursive https://github.com/zeniqsmart/zeniq-smart-chain
git clone --recursive https://github.com/zeniqsmart/db-zeniq-smart-chain
git clone --recursive https://github.com/zeniqsmart/evm-zeniq-smart-chain
git clone --recursive https://github.com/zeniqsmart/ads-zeniq-smart-chain
cd zeniq-smart-chain
sudo ./build_zeniqsmartd_for_ubuntu.sh

'

THISDIR=$PWD
sed -i -e"s/^\/\/ \(replace.*zeniq.*\)/\1/g" go.mod
sed -i -e"s/^\/\/ \(replace.*zeniq.*\)/\1/g" ../db-zeniq-smart-chain/go.mod
sed -i -e"s/^\/\/ \(replace.*zeniq.*\)/\1/g" ../evm-zeniq-smart-chain/go.mod
go mod tidy
go fmt ./...
go generate ./...
docker build -t zeniqsmart .
# then start the container mapping host, e.g. ~/smartzeniq to the container's /zeniq_smart using
docker run -t -v ${THISDIR%/*}:/zeniq_smart zeniqsmart zeniq-smart-chain/build.sh

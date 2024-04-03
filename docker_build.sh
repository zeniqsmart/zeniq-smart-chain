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
sudo ./docker_build.sh

'

THS="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
cd $THS

zeniq_smart=${THS%/*}

sed -i -e"s/^\/\/ \(replace.*zeniq.*\)/\1/g" go.mod
sed -i -e"s/^\/\/ \(replace.*zeniq.*\)/\1/g" ../db-zeniq-smart-chain/go.mod
sed -i -e"s/^\/\/ \(replace.*zeniq.*\)/\1/g" ../evm-zeniq-smart-chain/go.mod
go mod tidy
go fmt ./...
go generate ./...
docker build -t zeniqsmart .
# then start the container mapping host, e.g. ~/smartzeniq to the container's /zeniq_smart using
docker run -t -v $zeniq_smart:/zeniq_smart zeniqsmart zeniq-smart-chain/build.sh

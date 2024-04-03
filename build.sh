#!/usr/bin/env bash

: '

Do beforehand

mkdir smartzeniq
cd smartzeniq
git clone --recursive https://github.com/zeniqsmart/zeniq-smart-chain
git clone --recursive https://github.com/zeniqsmart/db-zeniq-smart-chain
git clone --recursive https://github.com/zeniqsmart/evm-zeniq-smart-chain
git clone --recursive https://github.com/zeniqsmart/ads-zeniq-smart-chain
cd zeniq-smart-chain

go fmt ./...
go generate ./...

To get CGO_LDFLAGS into current shell

. ./build.sh

#optionally in docker
docker run -it -v ${PWD%/*}:/zeniq_smart zeniqsmart bash

'

THS="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
cd $THS

BD=$THS/build

if [[ $1 == "clean" ]] ; then
    (cd ../evm-zeniq-smart-chain && make clean)
    rm -rf $BD
    go clean -cache
    go clean -testcache
fi

export CGO_LDFLAGS="-L$BD/zstd/lib -L$BD/bz2 -L$BD/lz4/lib -L$BD/snappy/build -L$BD/rocksdb -L$BD/zlib -L$BD/../evm-zeniq-smart-chain/evmwrap/host_bridge -lrocksdb -lsnappy -llz4 -lbz2 -lzstd -lz -lstdc++ -lm"
export CGO_CFLAGS="-I$BD/zstd/lib -I$BD/lz4/lib -I$BD/bz2 -I$BD/snappy -I$BD/snappy/build -I$BD/rocksdb/include -I$BD/zlib"
export PATH="${HOME}/go/bin:/usr/local/go/bin:$PATH"

if [[ "${BASH_SOURCE[0]}" == "${0}" ]] ; then

    mkdir build && ./deps.sh || echo "rm -rf build to build dependencies again"

    sed -i -e"s/^\/\/ \(replace.*zeniq.*\)/\1/g" go.mod
    sed -i -e"s/^\/\/ \(replace.*zeniq.*\)/\1/g" ../db-zeniq-smart-chain/go.mod
    sed -i -e"s/^\/\/ \(replace.*zeniq.*\)/\1/g" ../evm-zeniq-smart-chain/go.mod

    cd ../evm-zeniq-smart-chain/evmwrap
    make clean || true
    make -j4
    cd ../../zeniq-smart-chain
    go mod tidy

    if grep ID /etc/os-release | cut -d'=' -f2 | grep "arch" > /dev/null ; then
        go build -o ./build/zeniqsmartd ./cmd/zeniqsmartd
    else
        go build -ldflags "-linkmode 'external' -extldflags '-static'" -o ./build/zeniqsmartd ./cmd/zeniqsmartd
    fi

fi


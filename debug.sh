#!/usr/bin/env bash

: '

This is almost like build.sh,
but compiles and uses a libevmwrap.so that kontains debug info,
and starts gdb wih the created executable

gdb -args build/zeniqsmartd_debug start

'

THS=$PWD
if [[ "${THS##*/}" != "zeniq-smart-chain" ]]; then
    cd "zeniq-smart-chain"
    THS=$THS/zeniq-smart-chain
fi
BD=$THS/build

export CGO_LDFLAGS="-L$BD/zstd/lib -L$BD/bz2 -L$BD/lz4/lib -L$BD/snappy/build -L$BD/rocksdb -L$BD/zlib -L$BD/../evm-zeniq-smart-chain/evmwrap/host_bridge -lrocksdb -lsnappy -llz4 -lbz2 -lzstd -lz -lstdc++ -lm"
export CGO_CFLAGS="-g -I$BD/zstd/lib -I$BD/lz4/lib -I$BD/bz2 -I$BD/snappy -I$BD/snappy/build -I$BD/rocksdb/include -I$BD/zlib"
export PATH="${HOME}/go/bin:/usr/local/go/bin:$PATH"

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then

    mkdir build && ./deps.sh || echo "rm -rf build to build dependencies again"

    sed -i -e"s/^\/\/ \(replace.*zeniq.*\)/\1/g" go.mod
    sed -i -e"s/^\/\/ \(replace.*zeniq.*\)/\1/g" ../db-zeniq-smart-chain/go.mod
    sed -i -e"s/^\/\/ \(replace.*zeniq.*\)/\1/g" ../evm-zeniq-smart-chain/go.mod

    cd ../evm-zeniq-smart-chain
    make clean || true
    make -j4 debug
    cd ../zeniq-smart-chain
    go mod tidy

    rm -rf ./build/zeniqsmartd_debug
    rm -rf ./build/libevmwrap.so
    go build -o ./build/zeniqsmartd_debug ./cmd/zeniqsmartd
    mv ../evm-zeniq-smart-chain/evmwrap/host_bridge/libevmwrap.so build

    export LD_LIBRARY_PATH=$THS/build:$LD_LIBRARY_PATH

    if type cgdb ; then
        cgdb -args build/zeniqsmartd_debug start
    else
        gdb -args build/zeniqsmartd_debug start
    fi

fi


#!/usr/bin/env bash

BD=`pwd`/build
echo $BD
export CGO_LDFLAGS="-L$BD/zstd/lib -L$BD/bz2 -L$BD/lz4/lib -L$BD/snappy/build -L$BD/rocksdb -L$BD/zlib -L$BD/../evm-zeniq-smart-chain/evmwrap/host_bridge -lrocksdb -lsnappy -llz4 -lbz2 -lzstd -lz -lstdc++ -lm"
export CGO_CFLAGS="-I$BD/zstd/lib -I$BD/lz4/lib -I$BD/bz2 -I$BD/snappy -I$BD/snappy/build -I$BD/rocksdb/include -I$BD/zlib"
export PATH="${HOME}/go/bin:/usr/local/go/bin:$PATH"

(cd ../ads-zeniq-smart-chain && go test -c -gcflags '-N -l' &> /dev/null && ./ads-zeniq-smart-chain.test)
echo ads-zeniq-smart-chain

(cd ../db-zeniq-smart-chain/types && go test -c -gcflags '-N -l' &> /dev/null && ./types.test)
echo db-zeniq-smart-chain/types

(cd ../db-zeniq-smart-chain/db && go test -c -gcflags '-N -l' &> /dev/null && ./db.test)
echo db-zeniq-smart-chain/db

(cd ../db-zeniq-smart-chain/syncdb && go test -c -gcflags '-N -l' &> /dev/null && ./syncdb.test)
echo db-zeniq-smart-chain/syncdb

(cd ../evm-zeniq-smart-chain/ebp && go test -c -gcflags '-N -l' &> /dev/null && ./ebp.test)
echo evm-zeniq-smart-chain/ebp

(cd ../evm-zeniq-smart-chain/types && go test -c -gcflags '-N -l' &> /dev/null && ./types.test)
echo evm-zeniq-smart-chain/types

(cd ../zeniq-smart-chain/staking && go test -c -gcflags '-N -l' &> /dev/null && ./staking.test)
echo zeniq-smart-chain/staking

(cd ../zeniq-smart-chain/app && go test -c -gcflags '-N -l' &> /dev/null && ./app.test)
echo zeniq-smart-chain/app

(cd ../zeniq-smart-chain/ccrpc && go test -c -gcflags '-N -l' &> /dev/null && ./ccrpc.test)
echo zeniq-smart-chain/ccrpc

(cd ../zeniq-smart-chain/rpc/api && go test -c -gcflags '-N -l' &> /dev/null && ./api.test -test.v)
echo zeniq-smart-chain/rpc/api

(cd ../zeniq-smart-chain/seps && go test -c -gcflags '-N -l' &> /dev/null && ./seps.test)
echo zeniq-smart-chain/seps


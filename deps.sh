#!/usr/bin/env bash

set -e

BD=${0%/*}/build
pushd $BD

LZ4="1.9.4"
wget -O /tmp/lz4.tgz https://github.com/lz4/lz4/archive/refs/tags/v$LZ4.tar.gz
mkdir -p lz4 && tar -zxf /tmp/lz4.tgz -C lz4 --strip-components=1 
pushd lz4
make
# lz4/lib/liblz4.a
popd

BZ2="1.0.8"
wget -O /tmp/bz2.tgz https://github.com/libarchive/bzip2/archive/refs/tags/bzip2-$BZ2.tar.gz
mkdir -p bz2 && tar -zxf /tmp/bz2.tgz -C bz2 --strip-components=1
pushd bz2
make
# bz2/libbz2.a
popd

ZLIB="1.3.1"
wget -O /tmp/zlib.tgz https://www.zlib.net/zlib-$ZLIB.tar.gz
mkdir -p zlib && tar -zxf /tmp/zlib.tgz -C zlib --strip-components=1
pushd zlib
./configure --static && make
# zlib/libz.a
popd

ZSTD="1.5.5"
wget -O /tmp/zstd.tgz https://github.com/facebook/zstd/archive/refs/tags/v$ZSTD.tar.gz
mkdir -p zstd && tar -zxf /tmp/zstd.tgz -C zstd --strip-components=1
pushd zstd
make
# zstd/lib/libzstd.a
popd

SNAPPY="1.1.8"
wget -O /tmp/snappy.tgz https://github.com/google/snappy/archive/refs/tags/$SNAPPY.tar.gz
mkdir -p snappy/build && tar -zxf /tmp/snappy.tgz -C snappy --strip-components=1
pushd snappy/build
cmake -DSNAPPY_BUILD_TESTS=0 -DCMAKE_BUILD_TYPE=Release ../ && make -j4
# snappy/build/libsnappy.a
popd

ROCKSDB=5.18.4
wget -O /tmp/rocksdb.tgz "https://github.com/facebook/rocksdb/archive/refs/tags/v$ROCKSDB.tar.gz"
mkdir -p rocksdb && tar -xf /tmp/rocksdb.tgz -C rocksdb --strip-components=1
pushd rocksdb
sed -i '/#pragma once/c\#pragma once\n#include <cstdint>' db/compaction_iteration_stats.h
sed -i '/#pragma once/c\#pragma once\n#include <cstdint>' table/data_block_hash_index.h
sed -i '/#pragma once/c\#pragma once\n#include <cstdint>' util/string_util.h
sed -i '/#pragma once/c\#pragma once\n#include <cstdint>' include/rocksdb/utilities/checkpoint.h
CXXFLAGS="-std=c++17 -I$PWD/../snappy -I$PWD/../snappy/build -DSNAPPY -Wno-error=range-loop-construct -Wno-error=deprecated-copy -Wno-error=pessimizing-move -Wno-error=class-memaccess" LDFLAGS="-L$PWD/../snappy/build -lsnappy" make  -j4 static_lib
strip --strip-unneeded librocksdb.a
# rocksdb/librocksdb.a
popd

popd

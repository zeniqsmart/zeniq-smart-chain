#!/usr/bin/env bash

pushd /tmp
ROCKSDB_VERSION=5.18.4
wget "https://github.com/facebook/rocksdb/archive/refs/tags/v$ROCKSDB_VERSION.tar.gz"
tar -xvf v${ROCKSDB_VERSION}.tar.gz && cd rocksdb-${ROCKSDB_VERSION}
sed -i '/#pragma once/c\#pragma once\n#include <cstdint>' db/compaction_iteration_stats.h
sed -i '/#pragma once/c\#pragma once\n#include <cstdint>' table/data_block_hash_index.h 
sed -i '/#pragma once/c\#pragma once\n#include <cstdint>' util/string_util.h 
sed -i '/#pragma once/c\#pragma once\n#include <cstdint>' include/rocksdb/utilities/checkpoint.h

#make shared_lib #error: implicitly-declared ‘constexpr rocksdb::FileDescriptor::FileDescriptor
make clean
CXXFLAGS='-std=c++17 -Wno-error=range-loop-construct -Wno-error=deprecated-copy -Wno-error=pessimizing-move -Wno-error=class-memaccess' make shared_lib

sudo make install-shared INSTALL_PATH=/usr
popd

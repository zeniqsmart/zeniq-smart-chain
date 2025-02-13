#!/usr/bin/env bash

: '
sudo ./build_zeniqsmartd_for_ubuntu.sh
'

(cd ../evm-zeniq-smart-chain && make clean)
THS="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
cd $THS
rm -rf build
./docker_build.sh

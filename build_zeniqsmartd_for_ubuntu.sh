#!/usr/bin/env bash

THS="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
cd $THS
./docker_build.sh

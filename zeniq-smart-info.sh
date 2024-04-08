#!/usr/bin/env bash

: '

./zeniq-smart-info.sh at
./zeniq-smart-info.sh csv
./zeniq-smart-info.sh persistent

tendermint needed for some commands
either in current directory or installed

# to build tendermint:
git clone https://github.com/tendermint/tendermint
cd tendermint
git checkout v0.34.10
cd cmd/tendermint
go build

'

tm_dump_start(){
    if ! pgrep zeniqsmartd; then
        echo "zeniqsmartd not running" >&2
        exit 1
    fi
    tpwd=$(pwd)
    ttmp=$(mktemp -d)
    cd $ttmp
    local tendermint="tendermint"
    if command -v $tpwd/tendermint; then
        tendermint="$tpwd/tendermint"
    fi
    $tendermint debug dump . --frequency=600 --home ~/.zeniqsmartd --rpc-laddr "tcp://127.0.0.1:28545" &
    tpid=$!
    while (( $(ls $ttmp | wc -l) == 0 ))
    do
         sleep 5
    done
    local tzip=$(find . -name '*.zip')
    unzip $tzip
    local tbas=$(basename $tzip)
    local tbase=${tbas%.*}
    cd $tbase
}

tm_id_at_IP(){
    # find . -name net_info.json -exec jq -r '.peers[] | (.node_info.id|tostring) + "@" + (.remote_ip|tostring)' {} \; | paste -sd "," -
    find . -name consensus_state.json -exec jq -r '.peers[] | (.node_address|tostring)' {} \; | paste -sd "," -
}

tm_moniker_id_ip(){
    find . -name net_info.json -exec jq -r '.peers[] | (.node_info.moniker|tostring) + "," + (.node_info.id|tostring) + "," + (.remote_ip|tostring)' {} \;
}

tm_dump_end(){
    kill $tpid
    cd $tpwd
    rm -rf $ttmp
    tpid=""
    tpwd=""
    ttmp=""
}

tm_persistent() {
    cat ~/.zeniqsmartd/config/config.toml | grep "^persistent_peers=" | cut -d '=' -f 2 | tr -d '"'
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    tcmd=${1:-"now"}
    if [[ "${tcmd}" == "csv" ]]; then
        tm_dump_start 1> /dev/null
        tm_moniker_id_ip
        tm_dump_end
    elif [[ "${tcmd}" == "at" ]]; then
        tm_dump_start 1> /dev/null
        tm_id_at_IP
        tm_dump_end
    elif [[ "${tcmd}" == "persistent" ]]; then
        tm_persistent
    fi
fi


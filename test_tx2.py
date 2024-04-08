"""
type-0 tx (legacy) works (see z_smart_spend in testval.sh)
type-2 tx (this test) fail.

# have zeniqd running on host
# activate virtual environment
pip install pytest-docker
pip install bitcoinrpc
pip install eth-ape

cd zeniq-smart-chain
sudo rm -rf build
sudo ./docker_build.sh
ll build

python -m pytest
"""

import sys
import os
import web3
import eth_account as ea
import asyncio
from bitcoinrpc import BitcoinRPC
import tempfile
import pytest
import time

dckrfile='''
FROM ubuntu:22.04

ARG GOLANG_VERSION="1.20"

ARG GCC_VERSION="12"
ENV GV=${GCC_VERSION}
ARG TARGETARCH
ENV TARGETARCH=${TARGETARCH:-amd64}
ARG CHAIN_ID="0x59454E4951"

# Install apt based dependencies
RUN apt-get -y update && apt-get -y upgrade
RUN apt-get install -y software-properties-common && add-apt-repository -y ppa:ubuntu-toolchain-r/test
RUN apt-get -y install cmake gcc-${GV} g++-${GV} gcc g++ git libgflags-dev make wget
RUN apt-get -y install iputils-ping iproute2 httpie

# Make wget produce less visual noise in output
RUN echo "quiet=on\\nshow-progress=on\\nprogress=bar:force:noscroll" > ~/.wgetrc

# Setup build directory
RUN mkdir /build
WORKDIR /build

# Install Go
ENV GOROOT=/usr/local/go
ENV GOPATH=/go
ENV PATH=$GOPATH/bin:$GOROOT/bin:$PATH
RUN wget -O go.tgz https://dl.google.com/go/go${GOLANG_VERSION}.linux-${TARGETARCH}.tar.gz
RUN tar -zxf go.tgz -C /usr/local
RUN mkdir -p $GOPATH/bin

# Ugly hack: force compiling libevmwrap and zeniqsmartd with gcc-${GV} and g++-${GV}
RUN ln -s /usr/bin/gcc-${GV} /usr/local/bin/gcc
RUN ln -s /usr/bin/g++-${GV} /usr/local/bin/g++

ENV GOFLAGS="-buildvcs=false"

RUN mkdir /zeniq_smart
WORKDIR /zeniq_smart

EXPOSE 8545 8546
'''

dockercompose='''
version: "3.8"
services:
  node0:
    build: .
    command: zeniq-smart-chain/build/zeniqsmartd start --testing
    volumes:
      - {0}:/zeniq_smart
      - {1}/node0:/root/.zeniqsmartd
    networks:
      testnodes:
        ipv4_address: 172.18.188.10
  node1:
    build: .
    command: zeniq-smart-chain/build/zeniqsmartd start --testing
    volumes:
      - {0}:/zeniq_smart
      - {1}/node1:/root/.zeniqsmartd
    networks:
      testnodes:
        ipv4_address: 172.18.188.11
  node2:
    build: .
    command: zeniq-smart-chain/build/zeniqsmartd start --testing
    volumes:
      - {0}:/zeniq_smart
      - {1}/node2:/root/.zeniqsmartd
    networks:
      testnodes:
        ipv4_address: 172.18.188.12
  node3:
    build: .
    command: zeniq-smart-chain/build/zeniqsmartd start --testing
    volumes:
      - {0}:/zeniq_smart
      - {1}/node3:/root/.zeniqsmartd
    networks:
      testnodes:
        ipv4_address: 172.18.188.13
    # stdin_open: true
    # tty: true
networks:
  testnodes:
    ipam:
      driver: default
      config:
        - subnet: 172.18.188.0/24
'''


def blockcount():
    rpc = BitcoinRPC.from_config("http://localhost:57319", ("zeniq", "zeniq123"))
    BC = asyncio.run(rpc.getblockcount())
    del rpc
    return BC

def setup_acct():
    BC = blockcount()
    global acct
    acct = ea.Account.create()
    acctaddr = acct.address
    os.system(rf'build/zeniqsmartd testnet --v 4 --o {ndir} --populate-persistent-peers --starting-ip-address 172.18.188.10')
    assert os.path.exists(os.path.join(ndir,'node0','config','app.toml'))
    def fixnode(i):
        for x in [
            rf'sed -i "s/0xf96ae03f3637e3195ebcb85f0043052338196e57/{acctaddr}/g" {ndir}/node{i}/config/genesis.json',
            rf'sed -i "s/cc-rpc-epochs.*/cc-rpc-epochs = [[{BC},6,2400]]/g" {ndir}/node{i}/config/app.toml',
            rf'sed -i "s/cc-rpc-fork-block.*/cc-rpc-fork-block = 0/g" {ndir}/node{i}/config/app.toml',
            rf'sed -i "s/mainnet-rpc-url.*/mainnet-rpc-url = \"http:\/\/172.17.0.1:57319\"/g" {ndir}/node{i}/config/app.toml'
        ]:
            os.system(x)
    fixnode(0)
    fixnode(1)
    fixnode(2)
    fixnode(3)

@pytest.fixture(scope="session")
def docker_compose_file(pytestconfig):
    global ndir
    os.system('docker network prune -f')
    ndir=tempfile.mkdtemp()
    dc = dockercompose.format(os.path.split(str(pytestconfig.rootdir))[0],ndir)
    dcfile = os.path.join(ndir,'docker-compose.yml')
    with open(dcfile,'w') as f:
        f.write(dc)
    with open(os.path.join(ndir,'Dockerfile'),'w') as f:
        f.write(dckrfile)
    setup_acct()
    return dcfile

@pytest.fixture(scope="session")
def w3(docker_ip, docker_services):
    w3=web3.Web3(web3.HTTPProvider('http://172.18.188.10:8545'))
    docker_services.wait_until_responsive(timeout=30.0, pause=0.1, check=lambda: w3.is_connected())
    return w3

def logtx2(x):
    with open(os.path.expanduser('~/test_tx2.log.txt'),'w') as ndf:
      ndf.write(str(x))
      ndf.write('\n')

def test_tx_type_2(w3):
    with open(os.path.expanduser('~/test_tx2.log.txt'),'w') as ndf:
      ndf.write('\n')
    acct2 = ea.Account.create()
    w3.middleware_onion.add(web3.middleware.construct_sign_and_send_raw_middleware(acct))
    v = w3.eth.get_balance(acct.address)
    logtx2(acct.address)
    logtx2(v)
    v = w3.eth.get_balance(acct2.address)
    logtx2(acct2.address)
    logtx2(v)
    time.sleep(3)
    w3.eth.send_transaction({"from":acct.address,"value":20000000000000000000,"to":acct2.address})
    logtx2(w3.eth.get_balance(acct2.address))


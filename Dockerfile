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
RUN echo "quiet=on\nshow-progress=on\nprogress=bar:force:noscroll" > ~/.wgetrc

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

VERSION := $(shell echo $(shell git describe --tags) | sed 's/^v//')
COMMIT := $(shell git log -1 --format='%H')

build_tags = cppbtree

ldflags += -linkmode 'external'
ldflags += -extldflags '-static'
ldflags += -X github.com/zeniqsmart/zeniq-smart-chain/app.GitCommit=$(COMMIT)
ldflags += -X github.com/cosmos/cosmos-sdk/version.GitTag=$(VERSION)

BUILD_FLAGS := -tags "$(build_tags)" -ldflags '$(ldflags)'

build: go.sum
ifeq ($(OS), Windows_NT)
	go build -mod=readonly $(BUILD_FLAGS) -o build/zeniqsmartd.exe ./cmd/zeniqsmartd
else
	go build -mod=readonly $(BUILD_FLAGS) -o build/zeniqsmartd ./cmd/zeniqsmartd
endif

build-linux: go.sum
	GOOS=linux GOARCH=amd64 $(MAKE) build

.PHONY: all build build-linux

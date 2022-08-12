GIT_COMMIT_SHA := $(shell git rev-list -1 HEAD)
BUILD_TAGS = rocksdb,builtin_static
BUILD_LD_FLAGS = "-X github.com/iotaledger/wasp/packages/wasp.VersionHash=$(GIT_COMMIT_SHA)"

#
# You can override these e.g. as
#     make test TEST_PKG=./packages/vm/core/testcore/ TEST_ARG="-v --run TestAccessNodes"
#
TEST_PKG=./...
TEST_ARG=

all: build-lint

wasm:
	go install ./tools/schema
	bash contracts/wasm/scripts/generate_wasm.sh

compile-solidity:
ifeq (, $(shell which solc))
	@echo "no solc found in PATH, evm contracts won't be compiled"
else
	cd packages/vm/core/evm/iscmagic && go generate
	cd packages/evm/evmtest && go generate
endif

build: compile-solidity
	go build -o . -tags $(BUILD_TAGS) -ldflags $(BUILD_LD_FLAGS) ./...

build-lint: build lint

test-full: install
	go test -tags $(BUILD_TAGS),runheavy ./... --timeout 60m --count 1 -failfast

test: install
	go test -tags $(BUILD_TAGS) $(TEST_PKG) --timeout 40m --count 1 -failfast $(TEST_ARG)

test-short:
	go test -tags $(BUILD_TAGS) --short --count 1 -failfast $(shell go list ./... | grep -v github.com/iotaledger/wasp/contracts/wasm | grep -v github.com/iotaledger/wasp/packages/vm/)

install: compile-solidity
	go install -tags $(BUILD_TAGS) -ldflags $(BUILD_LD_FLAGS) ./...

lint:
	golangci-lint run

gofumpt-list:
	gofumpt -l ./

docker-build:
	docker build \
		--build-arg BUILD_TAGS=${BUILD_TAGS} \
		--build-arg BUILD_LD_FLAGS='${BUILD_LD_FLAGS}' \
		.

.PHONY: all build build-lint test test-short test-full install lint gofumpt-list docker-build


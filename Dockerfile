ARG GOLANG_IMAGE_TAG

# Build stage
FROM golang:${GOLANG_IMAGE_TAG} AS build

ARG BUILD_TAGS

WORKDIR /wasp

# Make sure that modules only get pulled when the module file has changed
COPY go.mod go.sum /wasp/
RUN go mod download
RUN go mod verify

# Project build stage
COPY . .

RUN go build -tags="$BUILD_TAGS"
RUN go build -tags="$BUILD_TAGS" ./tools/wasp-cli

# Testing stages
# Complete testing
FROM golang:${GOLANG_IMAGE_TAG} AS test-full
WORKDIR /run

COPY --from=build $GOPATH/pkg/mod $GOPATH/pkg/mod
COPY --from=build /wasp/ /run

CMD go test -tags rocksdb -timeout 20m ./...

# Unit tests without integration tests
FROM golang:${GOLANG_IMAGE_TAG} AS test-unit
WORKDIR /run

COPY --from=build $GOPATH/pkg/mod $GOPATH/pkg/mod
COPY --from=build /wasp/ /run

CMD go test -tags rocksdb -short ./...

# Wasp CLI build
# FROM golang:${GOLANG_IMAGE_TAG} as wasp-cli
# COPY --from=build /wasp/wasp-cli /usr/bin/wasp-cli
# ENTRYPOINT ["wasp-cli"]

# Wasp build
FROM golang:${GOLANG_IMAGE_TAG}

WORKDIR /run 

EXPOSE 7000/tcp
EXPOSE 9090/tcp
EXPOSE 5550/tcp
EXPOSE 4000/udp

COPY --from=build /wasp/wasp /usr/bin/wasp
COPY --from=build /wasp/wasp-cli /usr/bin/wasp-cli

ENTRYPOINT [ "wasp" ]

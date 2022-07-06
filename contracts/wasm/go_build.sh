#!/bin/bash
example_name=$1
flag=$2
cd $example_name

if [ -f "schema.yaml" ]; then
    if [ -f "schema.json" ]; then
        exit 1
    fi
fi

echo "Building $example_name"
schema -go $flag
echo "compiling "$example_name"_go.wasm"

if [ ! -d "./go/pkg" ]; then
    mkdir ./go/pkg
fi
tinygo build -o ./go/pkg/"$example_name"_go.wasm -target wasm -gc=leaking -opt 2 -no-debug go/main.go

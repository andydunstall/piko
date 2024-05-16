#!/bin/bash

declare -a arr=(
	"linux/amd64"
	"linux/arm64"
	"darwin/amd64"
	"darwin/arm64"
)

mkdir -p bin/artifacts

for i in "${arr[@]}"
do
	GOOSARCH=$i
	GOOS=${GOOSARCH%/*}
	GOARCH=${GOOSARCH#*/}
	BINARY_NAME=piko-$GOOS-$GOARCH

	echo "Building $BINARY_NAME..."
	GOOS=$GOOS GOARCH=$GOARCH go build -o bin/artifacts/$BINARY_NAME main.go
done

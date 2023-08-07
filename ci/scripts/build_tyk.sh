#!/bin/bash

# List of architectures for which the pump assets will be built
architectures=("amd64" "arm64" "s390x")

# Iterate over the list of architectures
for arch in "${architectures[@]}"; do
    echo "Building pump assets for architecture: $arch"
    GOOS=linux GOARCH=$arch go build -o tyk-pump-$arch
done
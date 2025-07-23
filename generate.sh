#!/bin/bash

# Install dependencies
echo "Installing Connect RPC and protoc plugins..."
go install github.com/bufbuild/buf/cmd/buf@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest

# Generate protobuf files
echo "Generating protobuf files..."
[ -n "$(go env GOBIN)" ] && export PATH="$(go env GOBIN):${PATH}"
[ -n "$(go env GOPATH)" ] && export PATH="$(go env GOPATH)/bin:${PATH}"
buf generate

# Install Go dependencies
echo "Installing Go dependencies..."
go mod tidy

echo "Done!" 
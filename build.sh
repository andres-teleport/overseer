#!/bin/sh

protoc --go_out=. --go_opt=paths=source_relative \
    --go_opt=Mapi/overseer.proto="github.com/andres-teleport/overseer/api;api" \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    --go-grpc_opt=Mapi/overseer.proto="github.com/andres-teleport/overseer/api;api" \
    api/overseer.proto

go build -o bin/ ./...

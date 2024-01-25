#!/bin/bash

CUR=`readlink -f $(dirname $0)`

protoc --proto_path="${CUR}/proto" --go_out="${CUR}/proto" --go_opt=paths=source_relative message.proto
protoc-go-inject-tag -input="${CUR}/proto/*.pb.go" -remove_tag_comment
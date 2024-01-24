#!/bin/bash

CUR=`readlink -f $(dirname $0)`

protoc --proto_path="${CUR}/proto" --go_out="${CUR}/proto" --go_opt=paths=source_relative room.proto
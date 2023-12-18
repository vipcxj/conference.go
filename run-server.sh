#!/bin/bash

CUR=$(readlink -f $(dirname $0))

$CUR/server --authServerHost=0.0.0.0 --signalHost=0.0.0.0 --ip=127.0.0.1 --authServerCors=* --signalCors=* \
  --log.level=DEBUG \
  --record.enable \
  --record.dirPath=/home/ps/workspace/mp4/hls/{{label:uid}} \
  --record.indexName=master{{ext}}

#!/bin/bash

CUR=$(readlink -f $(dirname $0))

$CUR/server --authServerHost=0.0.0.0 --signalHost=0.0.0.0 --ip=127.0.0.1 --authServerCors=* --signalCors=*

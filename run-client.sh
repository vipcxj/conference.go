#!/bin/bash

CUR=$(readlink -f $(dirname $0))

cd $CUR/js/client
npm i
npm run build
cd $CUR/js/app
npm i
HOST=0.0.0.0 npm run start

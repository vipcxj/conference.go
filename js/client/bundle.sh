#!/bin/bash
set -e
PWD_DIR=`pwd`
CUR_DIR=`readlink -f $(dirname $0)`
cd $CUR_DIR
rm -rf ./dist
rm -f ./dist.tar.gz
npm run bundle
tar czf dist.tar.gz ./dist
cd $PWD_DIR

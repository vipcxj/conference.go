#!/bin/bash

CUR=$(readlink -f $(dirname $0))

$CUR/server \
  --authServerHost="0.0.0.0" --authServerCors="*" \
  --hostOrIp="192.168.1.233" \
  --prometheus.enable \
  --prometheus.goCollectors \
  --signal.hostOrIp="0.0.0.0" \
  --signal.cors="*" \
  --signal.gin.debug \
  --signal.prometheus.enable \
  --router.port=4321 \
  --log.level=DEBUG \
  --record.enable \
  --record.basePath=/home/ps/workspace/mp4/ \
  --record.dirPath="{{year}}/{{month}}/{{day}}/{{auth.Key}}/{{hour}}-{{minute}}-{{second}}-{{uuid:6}}" \
  --record.indexName="master{{ext}}" \
  --record.dbIndex.enable \
  --record.dbIndex.mongoUrl=mongodb://mongoadmin:pass@192.168.1.233:27017 \
  --record.dbIndex.database=test \
  --record.dbIndex.collection=record \
  --record.dbIndex.key="{{auth.Key}}"


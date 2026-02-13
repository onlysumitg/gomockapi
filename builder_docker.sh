#!/usr/bin/env bash

APP_VERSION='v1.0.7'


output_name='gomockapi_' 
echo 'Building..: go :'$output_name${APP_VERSION}
env CGO_ENABLED=0 go build -o ./bin/${output_name}${APP_VERSION} ./cmd/web


echo 'Building..: Docker' 

docker build --build-arg="APP_VERSION=${APP_VERSION}"  -t onlysumitg/gomockapi:${APP_VERSION} .
docker push onlysumitg/gomockapi:${APP_VERSION}
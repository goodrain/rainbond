#!/bin/bash

IP=${1:-127.0.0.1}

function run() {
    docker run -it --rm hub.goodrain.com/dc-deploy/archiver:domain init --ip $IP > /tmp/domain.log
}

case $1 in
    *)
        run
    ;;
esac
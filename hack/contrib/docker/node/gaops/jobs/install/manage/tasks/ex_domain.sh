#!/bin/bash

IP=$1
DOMAIN=${2:-$(cat /data/.domain.log)}

function domain() {
    ex_domain=$(cat /data/.domain.log)
    if [ $DOMAIN != $ex_domain ];then
        echo "DOMAIN NOT ALLOW"
    else
        docker run --rm hub.goodrain.com/dc-deploy/archiver:domain update --ip $IP --domain $DOMAIN
        if [ $? -eq 0 ];then
            echo "domain change Success!!!"
        else
            echo "domain change error!!!"
        fi
    fi
}

case $1 in
    *)
        domain
    ;;
esac
#!/bin/bash

function log.info() {
  echo "       $*"
}

function log.error() {
  echo " !!!     $*"
  echo ""
}

function log.stdout() {
    echo "$*" >&2
}

function plugins::image() {
    if [ ! -f "/grdata/.do_plugins" ];then
        log.info "pull image from hub"
        docker pull rainbond/plugins:tcm_20180117175939
        docker pull rainbond/plugins:envoy_discover_service_20180117184912
        docker tag rainbond/plugins:tcm_20180117175939 goodrain.me/tcm_20180117175939
        docker tag rainbond/plugins:envoy_discover_service_20180117184912 goodrain.me/envoy_discover_service_20180117184912
        log.info "push image to goodrain.me"
        docker push goodrain.me/tcm_20180117175939
        docker push goodrain.me/envoy_discover_service_20180117184912
        touch /grdata/.do_plugins
    else
        log.info "pull image from goodrain.me"
        docker pull goodrain.me/tcm_20180117175939
        docker pull goodrain.me/envoy_discover_service_20180117184912
    fi
}

function exec_sql() {
    log.info "exec sql for plugins exec"
    docker cp /usr/share/gr-rainbond-node/gaops/config/plugins.sql rbd-db:/root
    docker exec rbd-db mysql -e "use console;source /root/plugins.sql"
}

function run() {
    if [ ! -f "/grdata/.do_plugins" ];then
        log.info "first node,update database"
        exec_sql
    fi
    plugins::image
    log.stdout '{
            "status":[ 
            { 
                "name":"install_rbd_plugins", 
                "condition_type":"install_rbd_plugins", 
                "condition_status":"True"
            } 
            ], 
            "exec_status":"Success",
            "type":"install"
            }'
}

case $1 in
    *)
    run
    ;;
esac

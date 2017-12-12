#!/bin/bash

REPO_VERSION=$1
ZMQ_SUB=$2
ZMQ_TO=$3 #dalaran_cep host:port

IMAGE_NAME="hub.goodrain.com/dc-deploy/cep_prism:$REPO_VERSION"

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

function image::exist() {
    IMAGE=$1
    docker images  | sed 1d | awk '{print $1":"$2}' | grep $IMAGE >/dev/null 2>&1
    if [ $? -eq 0 ];then
        log.info "image $IMAGE exists"
        return 0
    else
        log.info "image $IMAGE not exists"
        return 1
    fi
}

function image::pull() {
    IMAGE=$1
    docker pull $IMAGE
    if [ $? -eq 0 ];then
        log.info "pull image $IMAGE success"
        return 0
    else
        log.info "pull image $IMAGE failed"
        return 1
    fi
}

function compose::config_update() {
    YAML_FILE=/etc/goodrain/docker-compose.yaml
    mkdir -pv `dirname $YAML_FILE`
    if [ ! -f "$YAML_FILE" ];then
        echo "version: '2.1'" > $YAML_FILE
    fi
    dc-yaml -f $YAML_FILE -u -
}

function run() {
    # 缺少离线模式
    image::exist $IMAGE_NAME || (
        log.info "pull image $IMAGE_NAME "
        image::pull $IMAGE_NAME || (exit 1)
    )

    compose::config_update << EOF
services:
  cep_prism:
    image: $IMAGE_NAME
    container_name: cep_prism
    environment:
      ZMQ_BIND_SUB: tcp://172.30.42.1:$ZMQ_SUB
      ZMQ_PUB_TO: tcp://$ZMQ_TO
      ZMQ_IO_THREADS: 2
    logging:
      driver: "json-file"
      options:
        max-size: "50m"
        max-file: "3"
    network_mode: "host"
    restart: always
EOF

    dc-compose up -d
}

case $1 in
    *)
    run
    ;;
esac
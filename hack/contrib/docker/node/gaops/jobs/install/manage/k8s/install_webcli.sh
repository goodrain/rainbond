#!/bin/bash

REPO_VER=$1

RBD_WEBCLI_VER=$(jq --raw-output '."rbd-webcli".version' /etc/goodrain/envs/rbd.json)
RBD_WEBCLI="rainbond/rbd-webcli:$RBD_WEBCLI_VER"

HOSTIP=$(cat /etc/goodrain/envs/ip.sh | awk -F '=' '{print $2}')
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


function compose::config_update() {
    YAML_FILE=/etc/goodrain/docker-compose.yaml
    mkdir -pv `dirname $YAML_FILE`
    if [ ! -f "$YAML_FILE" ];then
        echo "version: '2.1'" > $YAML_FILE
    fi
    dc-yaml -f $YAML_FILE -u -
}

function image::exist() {
    IMAGE=$1
    docker images  | sed 1d | awk '{print $1":"$2}' | grep $IMAGE >/dev/null 2>&1
    if [ $? -eq 0 ];then
        log.info "image $IMAGE exists"
        return 0
    else
        log.error "image $IMAGE not exists"
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

function prepare() {
    log.log "nothing prepare for  webcli"
}





function install_webcli() {
    #log.info "setup webcli"

    image::exist $RBD_WEBCLI || (
        log.info "pull image: $RBD_WEBCLI"
        image::pull $RBD_WEBCLI || (
            log.stdout '{ 
                "status":[ 
                { 
                    "name":"docker_pull_webcli", 
                    "condition_type":"DOCKER_PULL_WEBCLI_ERROR", 
                    "condition_status":"False"
                } 
                ], 
                "type":"install"
                }'
            exit 1
        )
    )

    compose::config_update << EOF
services:
  rbd-webcli:
    image: $RBD_WEBCLI
    container_name: rbd-webcli
    volumes:
    - /usr/bin/kubectl:/usr/bin/kubectl
    - /root/.kube:/root/.kube
    command: --hostIP=$HOSTIP
    logging:
      driver: json-file
      options:
        max-size: 50m
        max-file: '3'
    network_mode: host
    restart: always
EOF
    dc-compose up -d

}

function run() {
    
    log.info "setup webcli"
    install_webcli
    dc-compose ps  | grep webcli | grep Up
    if [ $? -eq 0 ];then
        log.stdout '{ 
                "status":[ 
                { 
                    "name":"install_webcli", 
                    "condition_type":"INSTALL_WEBCLI", 
                    "condition_status":"True"
                } 
                ], 
                "exec_status":"Success",
                "type":"install"
                }'
    else
        log.stdout '{ 
                "status":[ 
                { 
                    "name":"install_webcli", 
                    "condition_type":"INSTALL_WEBCLI_FAILED", 
                    "condition_status":"False"
                } 
                ],
                "type":"install"
                }'
    fi
}

case $1 in
    * )
        prepare
        run
        ;;
esac
#!/bin/bash
set -o errexit
set -o pipefail

REPO_VER=$1 #版本信息
MANAGE_PORT=$2
ACP_DIM="hub.goodrain.com/dc-deploy/acp_manager:${REPO_VER}"

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

    log.info "prepare acp_manage ui"

}

function run() {

    log.info "setup acp_manage ui"

        [ ! -f "/etc/goodrain/.version" ] && (

    image::exist $ACP_DIM || (
        log.info "pull image: $ACP_DIM"
        image::pull $ACP_DIM || (
            log.stdout '{ 
            "status":[ 
            { 
                "name":"pull_manage_ui_image", 
                "condition_type":"PULL_MANAGE_UI_IMAGE", 
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
  acp_manage:
    image: $ACP_DIM
    container_name: acp_manage
    environment:
      CONSOLE_URL: http://console.goodrain.me
    volumes:
      - /logs/manage:/tmp
    logging:
      driver: "json-file"
      options:
        max-size: "50m"
        max-file: "3"
    network_mode: "host"
    restart: always
    command:
      - python
      - manage.py
      - runserver
      - 0.0.0.0:${MANAGE_PORT:-9099}
EOF

    dc-compose up -d

) || (
    echo "Contact us:Goodrain Inc.<info@goodrain.com>"
)

    dc-compose ps | grep acp_manage >/dev/null
    if [ $? -eq 0 ];then
        log.stdout '{ 
            "status":[ 
            { 
                "name":"install_manage_ui", 
                "condition_type":"INSTALL_MANAGE_UI", 
                "condition_status":"True"
            } 
            ], 
            "type":"check"
            }'
    else
        log.stdout '{ 
            "status":[ 
            { 
                "name":"install_manage_ui", 
                "condition_type":"INSTALL_MANAGE_UI", 
                "condition_status":"False"
            } 
            ], 
            "type":"check"
            }'
    fi
}

case $1 in
    * )
        prepare
        run
        ;;
esac
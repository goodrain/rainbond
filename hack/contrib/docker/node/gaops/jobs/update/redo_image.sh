#!/bin/bash

MYSQL_USER=$1
MYSQL_PASSWD=$2
MYSQL_HOST=$3
MYSQL_PORT=$4
EX_DOMAIN=$5
MYSQL_DB="region"

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
# Todo list
# 其他管理节点从goodrain.me 拉取
#

function image::exist() {
    IMAGE_NAME=$1
    docker images | sed 1d | awk '{print $1":"$2}' | grep $IMAGE_NAME >/dev/null 2>&1
    if [ $? -eq 0 ];then
        log.info "image $IMAGE_NAME exists"
        return 0
    else
        log.info "image $IMAGE_NAME not exists"
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

function image::pull() {
    IMAGES_NAME=$1
    docker pull $IMAGES_NAME
    if [ $? -eq 0 ];then
        log.info "pull image $IMAGES_NAME success"
        return 0
    else
        log.info "pull image $IMAGES_NAME failed"
        return 1
    fi
}

function image::push() {
    BASE_NAME=$1
    VERSION=$2
    if [ -n "$VERSION" ];then
        IMAGES_NAME_Pb="hub.goodrain.com/dc-deploy/$BASE_NAME:$VERSION"
    else
        IMAGES_NAME_Pb="hub.goodrain.com/dc-deploy/$BASE_NAME:latest"
    fi
    log.info "docker pull $IMAGES_NAME_Pb"
    docker pull $IMAGES_NAME_Pb
    if [ $BASE_NAME = "adapter" ];then
        IMAGES_NAME_Pr="goodrain.me/$BASE_NAME"
    else
        IMAGES_NAME_Pr="goodrain.me/$BASE_NAME:$VERSION"
    fi
    log.info "docker tag $IMAGES_NAME_Pb $IMAGES_NAME_Pr"
    docker tag $IMAGES_NAME_Pb $IMAGES_NAME_Pr
    log.info "docker push $IMAGES_NAME_Pr"
    docker push $IMAGES_NAME_Pr
    log.info "    docker for $BASE_NAME end"
}

function run() {
    
    if [ ! -f "/grdata/.do_image" ];then
        log.info "first node"
        image::push runner latest
        image::push adapter 3.4
        image::push pause-amd64 3.0 
        image::push builder latest
        touch /grdata/.do_image
    else
        log.info "not 1st node"
        image::exist goodrain.me/runner:latest || image::pull goodrain.me/runner:latest || image::push runner latest
        image::exist goodrain.me/adapter:latest || image::pull goodrain.me/adapter:latest || image::push adapter 3.4
        image::exist goodrain.me/pause-amd64:3.0 || image::pull goodrain.me/pause-amd64:3.0 || image::push pause-amd64 3.0                                                                                     
        image::exist goodrain.me/builder:latest || image::pull goodrain.me/builder:latest || image::push builder latest
    fi

    image::pull rainbond/rbd-webcli:$(jq --raw-output '."rbd-webcli".version' /etc/goodrain/envs/rbd.json)
    image::pull rainbond/rbd-api:$(jq --raw-output '."rbd-api".version' /etc/goodrain/envs/rbd.json)
    image::pull rainbond/rbd-chaos:$(jq --raw-output '."rbd-chaos".version' /etc/goodrain/envs/rbd.json)
    image::pull rainbond/rbd-worker:$(jq --raw-output '."rbd-worker".version' /etc/goodrain/envs/rbd.json)
    image::pull rainbond/rbd-mq:$(jq --raw-output '."rbd-mq".version' /etc/goodrain/envs/rbd.json)
    image::pull rainbond/rbd-entrance:$(jq --raw-output '."rbd-entrance".version' /etc/goodrain/envs/rbd.json)
    image::pull rainbond/rbd-eventlog:$(jq --raw-output '."rbd-eventlog".version' /etc/goodrain/envs/rbd.json)
    image::pull rainbond/rbd-app-ui:$(jq --raw-output '."rbd-app-ui".version' /etc/goodrain/envs/rbd.json)
    image::pull rainbond/rbd-lb:$(jq --raw-output '."rbd-lb".version' /etc/goodrain/envs/rbd.json)

    log.info "update docker-compose.yaml"
    sed -i "s#3.4.1#3.4.2#g" /etc/goodrain/docker-compose.yaml
    sed -i "s#rbd-app-ui:3.4#rbd-app-ui:3.4.2#g" /etc/goodrain/docker-compose.yaml
    #sed -i "s#rbd-lb:3.4#rbd-lb:3.4.2#g" /etc/goodrain/docker-compose.yaml
    dc-compose up -d
    log.info "migrate database"
    docker exec rbd-app-ui python /app/ui/manage.py migrate

    compose::config_update << EOF
services:
  rbd-lb:
    image: rainbond/rbd-lb:3.4.2
    container_name: rbd-lb
    environment:
      NGINX_INIT_PORT: 80
      MYSQL_HOST: ${MYSQL_HOST:-127.0.0.1}
      MYSQL_PORT: ${MYSQL_PORT:-3306}
      MYSQL_USERNAME: $MYSQL_USER
      MYSQL_PASSWORD: $MYSQL_PASSWD
      MYSQL_DATABASE: $MYSQL_DB
      HTTP_SUFFIX_URL: ${EX_DOMAIN#.*}
      NGINX_SSL_PORT: 10443
    volumes:
      - /etc/goodrain/openresty:/usr/local/openresty/nginx/conf
      - /data/openrestry/logs:/usr/local/openresty/nginx/logs
    logging:
      driver: "json-file"
      options:
        max-size: "50m"
        max-file: "3"
    network_mode: "host"
    restart: always
EOF

    dc-compose up -d rbd-lb 

    log.stdout '{
            "global":{
                    "REGION_TAG":"cloudbang"
                 },
            "status":[ 
            { 
                "name":"redo_rbd_images", 
                "condition_type":"REDO_RBD_IMAGES", 
                "condition_status":"True"
            }
            ], 
            "exec_status":"Success",
            "type":"install"
            }'
}

case $1 in
    * )
        run
    ;;
esac
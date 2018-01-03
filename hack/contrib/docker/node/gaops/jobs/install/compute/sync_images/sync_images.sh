#!/bin/bash

# 同步镜像
# 安装相关组件
REPO_VERSION=$1
INSTALL_TYPE=${2:-online}

IMAGE_PATH="/root/acpimg" 

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
    IMAGE_NAME=$1
    docker images | sed 1d | awk '{print $1":"$2}' | grep $IMAGES_NAME >/dev/null 2>&1
    if [ $? -eq 0 ];then
        echo "image $IMAGE_NAME exists"
        return 0
    else
        echo "image $IMAGE_NAME not exists"
        return 1
    fi
}

function image::pull() {
    IMAGES_NAME=$1
    docker pull $IMAGES_NAME
    if [ $? -eq 0 ];then
        echo "pull image $IMAGES_NAME success"
        return 0
    else
        echo "pull image $IMAGES_NAME failed"
        return 1
    fi
}

function image::load() {
    # local type
    IMAGE_NAME=$1
    IMAGE_NAME_ID=`echo ${IMAGE_NAME}|sed 's/:/_/'`
    if [ -f "$IMAGE_PATH/$IMAGE_NAME_ID.gz" ];then
        cat $IMAGE_PATH/$IMAGE_NAME_ID.gz | docker load
    else
        exit 1
    fi
}

function image::push() {
    BASE_NAME=$1
    VERSION=$2
    if [ -n "$VERSION" ];then
        IMAGES_NAME_Pb="hub.goodrain.com/dc-deploy/$BASE_NAME:$VERSION"
    else
        IMAGES_NAME_Pb="hub.goodrain.com/dc-deploy/$BASE_NAME"
    fi
    IMAGES_NAME_Pr="goodrain.me/$BASE_NAME"
    image::exist $IMAGES_NAME_Pb || image::pull $IMAGES_NAME_Pb || image::load $IMAGES_NAME_Pb 

    docker tag IMAGES_NAME_Pb IMAGES_NAME_Pr 
    docker push IMAGES_NAME_Pr
}

function image::package() {
    ex_pkg=$1
    docker run --rm -v /var/run/docker.sock:/var/run/docker.sock hub.goodrain.com/dc-deploy/archiver $ex_pkg
}

function run() {
    log.info "pull images"
    image::exist goodrain.me/runner:latest || image::pull goodrain.me/runner:latest || image::push runner latest
    image::exist goodrain.me/adapter:latest || image::pull goodrain.me/adapter:latest || image::push adapter $REPO_VERSION
    image::exist goodrain.me/pause-amd64:3.0 || image::pull goodrain.me/pause-amd64:3.0 || image::push pause-amd64 3.0
    image::package gr-nsenter
    image::package gr-docker-compose
    image::package gr-docker-utils
    image::package gr-midonet-cni 
}

case $1 in
    * )
        run
        ;;
esac
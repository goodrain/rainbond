#!/bin/bash

REPO_VER=$1

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
    docker tag $IMAGES_NAME_Pb $IMAGES_NAME_Pr
    docker push $IMAGES_NAME_Pr
}

function run() {
    image::push runner latest
    image::push adapter 3.4
    image::push pause-amd64 3.0
    image::push builder latest

    log.stdout '{ 
            "status":[ 
            { 
                "name":"do_rbd_images", 
                "condition_type":"DO_RBD_IMAGES", 
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
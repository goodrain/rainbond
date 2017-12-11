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

function log.section() {
    local title=$1
    local title_length=${#title}
    local width=$(tput cols)
    local arrival_cols=$[$width-$title_length-2]
    local left=$[$arrival_cols/2]
    local right=$[$arrival_cols-$left]

    echo ""
    printf "=%.0s" `seq 1 $left`
    printf " $title "
    printf "=%.0s" `seq 1 $right`
    echo ""
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
    docker tag $IMAGES_NAME_Pb $IMAGES_NAME_Pr
    docker push $IMAGES_NAME_Pr
}

function run() {
    image::push runner latest
    image::push adapter $REPO_VER
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
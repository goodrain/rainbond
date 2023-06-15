#!/bin/bash

export DOCKER_VER=19.03.5
RBD_VER=${RBD_VER:-'v5.14.1-release'}

function rbd_amd64_images() {

    cat >./offline/image_arch/amd64_list.txt <<EOF
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-node:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-resource-proxy:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-eventlog:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-worker:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-gateway:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-chaos:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-api:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-webcli:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-mq:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-monitor:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-mesh-data-panel:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-init-probe:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-grctl:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rainbond-operator:$RBD_VER
EOF
    docker login -u "$DOMESTIC_DOCKER_USERNAME" -p "$DOMESTIC_DOCKER_PASSWORD" "${DOMESTIC_BASE_NAME}"
    while read rbd_image_name; do
        rbd_image=$(echo ${rbd_image_name} | awk -F"/" '{print $NF}')
        docker pull ${rbd_image_name}
        docker tag ${rbd_image_name} registry.cn-hangzhou.aliyuncs.com/goodrain/${rbd_image}-amd64
        docker push registry.cn-hangzhou.aliyuncs.com/goodrain/${rbd_image}-amd64
    done <./offline/image_arch/amd64_list.txt

}

function rbd_arm64_images() {

    cat >./offline/image_arch/arm64_list.txt <<EOF
docker.io/rainbond/rbd-node:$RBD_VER-arm64
docker.io/rainbond/rbd-resource-proxy:$RBD_VER-arm64
docker.io/rainbond/rbd-eventlog:$RBD_VER-arm64
docker.io/rainbond/rbd-worker:$RBD_VER-arm64
docker.io/rainbond/rbd-gateway:$RBD_VER-arm64
docker.io/rainbond/rbd-chaos:$RBD_VER-arm64
docker.io/rainbond/rbd-api:$RBD_VER-arm64
docker.io/rainbond/rbd-webcli:$RBD_VER-arm64
docker.io/rainbond/rbd-mq:$RBD_VER-arm64
docker.io/rainbond/rbd-monitor:$RBD_VER-arm64
docker.io/rainbond/rbd-mesh-data-panel:$RBD_VER-arm64
docker.io/rainbond/rbd-init-probe:$RBD_VER-arm64
docker.io/rainbond/rbd-grctl:$RBD_VER-arm64
docker.io/rainbond/rainbond-operator:$RBD_VER-arm64

EOF
    while read rbd_image_name; do
        rbd_image=$(echo ${rbd_image_name} | awk -F"/" '{print $NF}')
        rbd_image="${rbd_image%-arm64}"
        docker pull ${rbd_image_name}
        docker tag ${rbd_image_name} registry.cn-hangzhou.aliyuncs.com/goodrain/${rbd_image}-arm64
        docker push registry.cn-hangzhou.aliyuncs.com/goodrain/${rbd_image}-arm64
    done <./offline/image_arch/arm64_list.txt

}

function handle_images_arch() {

  cat >./offline/image_arch/list.txt <<EOF
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-node:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-resource-proxy:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-eventlog:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-worker:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-gateway:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-chaos:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-api:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-webcli:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-mq:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-monitor:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-mesh-data-panel:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-init-probe:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-grctl:$RBD_VER
registry.cn-hangzhou.aliyuncs.com/goodrain/rainbond-operator:$RBD_VER
EOF
    while read rbd_image_name; do
         rbd_image=$(echo ${rbd_image_name} | awk -F"/" '{print $NF}')
         docker manifest rm registry.cn-hangzhou.aliyuncs.com/goodrain/${rbd_image}
         docker manifest create registry.cn-hangzhou.aliyuncs.com/goodrain/${rbd_image} registry.cn-hangzhou.aliyuncs.com/goodrain/${rbd_image}-amd64 registry.cn-hangzhou.aliyuncs.com/goodrain/${rbd_image}-arm64
         docker manifest push registry.cn-hangzhou.aliyuncs.com/goodrain/${rbd_image}
    done <./offline/image_arch/list.txt
}


function main() {

    mkdir -p ./offline ./offline/image_arch
    rbd_amd64_images
    rbd_arm64_images
    handle_images_arch
}

main

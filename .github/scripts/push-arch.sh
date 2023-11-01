#!/bin/bash
################################################################################
# Copyright (c) Goodrain, Inc.
#
# This source code is licensed under the LGPL-3.0 license found in the
# LICENSE file in the root directory of this source tree.
################################################################################

IMAGE_NAMESPACE=${IMAGE_NAMESPACE:-"rainbond"}

DOMESTIC_NAME=${DOMESTIC_BASE_NAME:-'registry.cn-hangzhou.aliyuncs.com'}
DOMESTIC_NAMESPACE=${DOMESTIC_NAMESPACE:-'goodrain'}

function push_domestic_amd64 {

  image_list="$IMAGE_NAMESPACE/rbd-node:$RBD_VER
$IMAGE_NAMESPACE/rbd-resource-proxy:$RBD_VER
$IMAGE_NAMESPACE/rbd-eventlog:$RBD_VER
$IMAGE_NAMESPACE/rbd-worker:$RBD_VER
$IMAGE_NAMESPACE/rbd-gateway:$RBD_VER
$IMAGE_NAMESPACE/rbd-chaos:$RBD_VER
$IMAGE_NAMESPACE/rbd-api:$RBD_VER
$IMAGE_NAMESPACE/rbd-webcli:$RBD_VER
$IMAGE_NAMESPACE/rbd-mq:$RBD_VER
$IMAGE_NAMESPACE/rbd-monitor:$RBD_VER
$IMAGE_NAMESPACE/rbd-mesh-data-panel:$RBD_VER
$IMAGE_NAMESPACE/rbd-init-probe:$RBD_VER
$IMAGE_NAMESPACE/rbd-grctl:$RBD_VER
$IMAGE_NAMESPACE/rbd-shell:$RBD_VER
$IMAGE_NAMESPACE/rainbond-operator:$RBD_VER"
    
    for images in ${image_list}; do
      domestic_image=$(echo "${images}" | awk -F"/" '{print $NF}')

      docker pull "${images}" || exit 1
      docker tag "${images}" "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/$domestic_image-amd64"
      docker push "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/$domestic_image-amd64"
    done
}

function push_domestic_arm64 {

  image_list="$IMAGE_NAMESPACE/rbd-node:$RBD_VER-arm64
$IMAGE_NAMESPACE/rbd-resource-proxy:$RBD_VER-arm64
$IMAGE_NAMESPACE/rbd-eventlog:$RBD_VER-arm64
$IMAGE_NAMESPACE/rbd-worker:$RBD_VER-arm64
$IMAGE_NAMESPACE/rbd-gateway:$RBD_VER-arm64
$IMAGE_NAMESPACE/rbd-chaos:$RBD_VER-arm64
$IMAGE_NAMESPACE/rbd-api:$RBD_VER-arm64
$IMAGE_NAMESPACE/rbd-webcli:$RBD_VER-arm64
$IMAGE_NAMESPACE/rbd-mq:$RBD_VER-arm64
$IMAGE_NAMESPACE/rbd-monitor:$RBD_VER-arm64
$IMAGE_NAMESPACE/rbd-mesh-data-panel:$RBD_VER-arm64
$IMAGE_NAMESPACE/rbd-init-probe:$RBD_VER-arm64
$IMAGE_NAMESPACE/rbd-grctl:$RBD_VER-arm64
$IMAGE_NAMESPACE/rbd-shell:$RBD_VER-arm64
$IMAGE_NAMESPACE/rainbond-operator:$RBD_VER-arm64"
    
    for images in ${image_list}; do
      domestic_image=$(echo "${images}" | awk -F"/" '{print $NF}')

      docker pull "${images}" || exit 1
      docker tag "${images}" "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/$domestic_image"
      docker push "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/$domestic_image"
    done
}

function push_arch() {

  push_domestic_amd64
  push_domestic_arm64

  image_list="$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rbd-node:$RBD_VER
$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rbd-resource-proxy:$RBD_VER
$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rbd-eventlog:$RBD_VER
$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rbd-worker:$RBD_VER
$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rbd-gateway:$RBD_VER
$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rbd-chaos:$RBD_VER
$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rbd-api:$RBD_VER
$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rbd-webcli:$RBD_VER
$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rbd-mq:$RBD_VER
$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rbd-monitor:$RBD_VER
$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rbd-mesh-data-panel:$RBD_VER
$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rbd-init-probe:$RBD_VER
$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rbd-grctl:$RBD_VER
$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rbd-shell:$RBD_VER
$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond-operator:$RBD_VER"
    
    for images in ${image_list}; do
      domestic_image=$(echo "${images}" | awk -F"/" '{print $NF}')
      docker manifest rm "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/$domestic_image"
      docker manifest create "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/$domestic_image" "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/$domestic_image-amd64" "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/$domestic_image-arm64"
      docker manifest push "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/$domestic_image"
    done
}

function push_arch_allinone {

  docker pull "$IMAGE_NAMESPACE/rainbond:${RBD_VER}-allinone" || exit 1
  docker tag "$IMAGE_NAMESPACE/rainbond:${RBD_VER}-allinone" "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond:${RBD_VER}-allinone-amd64"
  docker push "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond:${RBD_VER}-allinone-amd64"

  docker pull "$IMAGE_NAMESPACE/rainbond:${RBD_VER}-arm64-allinone" || exit 1
  docker tag "$IMAGE_NAMESPACE/rainbond:${RBD_VER}-arm64-allinone" "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond:${RBD_VER}-arm64-allinone"
  docker push "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond:${RBD_VER}-arm64-allinone"

  docker manifest rm "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond:${RBD_VER}-allinone"
  docker manifest create "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond:${RBD_VER}-allinone" "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond:${RBD_VER}-allinone-amd64" "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond:${RBD_VER}-arm64-allinone"
  docker manifest push "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond:${RBD_VER}-allinone"

  docker manifest create "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond:${RBD_VER}" "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond:${RBD_VER}-allinone-amd64" "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond:${RBD_VER}-arm64-allinone"
  docker manifest push "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond:${RBD_VER}"
}

function push_arch_dind {
  docker pull "$IMAGE_NAMESPACE/rainbond:${RBD_VER/-release}-dind-allinone" || exit 1
  docker tag "$IMAGE_NAMESPACE/rainbond:${RBD_VER/-release}-dind-allinone" "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond:${RBD_VER/-release}-dind-allinone-amd64"
  docker push "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond:${RBD_VER/-release}-dind-allinone-amd64"

  docker pull "$IMAGE_NAMESPACE/rainbond:${RBD_VER/-release}-arm64-dind-allinone" || exit 1
  docker tag "$IMAGE_NAMESPACE/rainbond:${RBD_VER/-release}-arm64-dind-allinone" "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond:${RBD_VER/-release}-arm64-dind-allinone"
  docker push "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond:${RBD_VER/-release}-arm64-dind-allinone"

  docker manifest rm "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond:${RBD_VER/-release}-dind-allinone"
  docker manifest create "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond:${RBD_VER/-release}-dind-allinone" "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond:${RBD_VER/-release}-dind-allinone-amd64" "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond:${RBD_VER/-release}-arm64-dind-allinone"
  docker manifest push "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/rainbond:${RBD_VER/-release}-dind-allinone"
}

function push_arch_runner {
  docker pull "$IMAGE_NAMESPACE/runner:${RBD_VER}" || exit 1
  docker tag "$IMAGE_NAMESPACE/runner:${RBD_VER}" "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/runner:${RBD_VER}-amd64"
  docker push "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/runner:${RBD_VER}-amd64"

  docker pull "$IMAGE_NAMESPACE/runner:${RBD_VER}-arm64" || exit 1
  docker tag "$IMAGE_NAMESPACE/runner:${RBD_VER}-arm64" "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/runner:${RBD_VER}-arm64"
  docker push "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/runner:${RBD_VER}-arm64"

  docker manifest rm "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/runner:${RBD_VER}"
  docker manifest create "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/runner:${RBD_VER}" "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/runner:${RBD_VER}-amd64" "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/runner:${RBD_VER}-arm64"
  docker manifest push "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/runner:${RBD_VER}"
}

function push_arch_builder {
  docker pull "$IMAGE_NAMESPACE/builder:${RBD_VER}" || exit 1
  docker tag "$IMAGE_NAMESPACE/builder:${RBD_VER}" "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/builder:${RBD_VER}-amd64"
  docker push "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/builder:${RBD_VER}-amd64"

  docker pull "$IMAGE_NAMESPACE/builder:${RBD_VER}-arm64" || exit 1
  docker tag "$IMAGE_NAMESPACE/builder:${RBD_VER}-arm64" "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/builder:${RBD_VER}-arm64"
  docker push "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/builder:${RBD_VER}-arm64"

  docker manifest rm "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/builder:${RBD_VER}"
  docker manifest create "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/builder:${RBD_VER}" "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/builder:${RBD_VER}-amd64" "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/builder:${RBD_VER}-arm64"
  docker manifest push "$DOMESTIC_NAME/$DOMESTIC_NAMESPACE/builder:${RBD_VER}"
}

docker login "${DOMESTIC_NAME}" -u "$DOMESTIC_DOCKER_USERNAME" -p "$DOMESTIC_DOCKER_PASSWORD"

if [ "$1" = "builder-runner" ]; then
  push_arch_runner
  push_arch_builder
else
  push_arch
  push_arch_allinone
  push_arch_dind
fi
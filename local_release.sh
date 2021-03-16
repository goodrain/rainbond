#! /bin/bash

export VERSION=v5.3.0-release
export BUILD_IMAGE_BASE_NAME=registry.cn-hangzhou.aliyuncs.com/goodrain
./release.sh all push

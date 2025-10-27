#!/bin/bash

# 设置变量
IMAGE_NAME="block-mechanica"
TAG=${1:-latest}
DOCKERFILE_PATH="deploy/docker/Dockerfile"

echo "开始构建 Docker 镜像..."
echo "镜像名称: ${IMAGE_NAME}"
echo "标签: ${TAG}"
echo "Dockerfile 路径: ${DOCKERFILE_PATH}"

# 构建镜像
docker build -f ${DOCKERFILE_PATH} -t ${IMAGE_NAME}:${TAG} .

if [ $? -eq 0 ]; then
    echo "镜像构建成功!"
    echo "镜像信息:"
    docker images ${IMAGE_NAME}:${TAG}
    
    echo ""
    echo "运行命令示例:"
    echo "docker run -p 8080:8080 ${IMAGE_NAME}:${TAG}"
else
    echo "镜像构建失败!"
    exit 1
fi 

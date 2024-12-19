#!/bin/bash

# 检查是否提供了服务名称和镜像名称
if [ "$#" -lt 2 ]; then
  echo "Usage: $0 <service_name> <image_name>"
  echo "Example: $0 api my-api-image:1.0"
  exit 1
fi

# 获取输入参数
service_name=$1
image_name=$2

# 自动生成目标目录路径
base_dir="./hack/contrib/docker"
target_dir="$base_dir/$service_name"

# 检查服务目录是否存在
if [ ! -d "$target_dir" ]; then
  echo "Error: Directory for service '$service_name' ('$target_dir') does not exist."
  exit 1
fi

# 检查是否存在 Dockerfile
if [ ! -f "$target_dir/Dockerfile" ]; then
  echo "Error: No Dockerfile found in '$target_dir'."
  exit 1
fi

# 构建镜像
echo "Building Docker image '$image_name' for service '$service_name' from directory '$target_dir'..."
nerdctl build -f "$target_dir/Dockerfile" -t "$image_name" --namespace=k8s.io --address /var/run/k3s/containerd/containerd.sock ./

if [ $? -eq 0 ]; then
  echo "Successfully built image: $image_name"
else
  echo "Failed to build image: $image_name"
  exit 1
fi

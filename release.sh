#!/bin/bash
set -o errexit

# define package name
WORK_DIR=/go/src/github.com/goodrain/rainbond
BASE_NAME=rainbond
IMAGE_BASE_NAME=${IMAGE_NAMESPACE:-'rainbond'}
DOMESTIC_NAMESPACE=${DOMESTIC_NAMESPACE:-'goodrain'}
ENABLE_WAF=${ENABLE_WAF:-'true'}

if [ "$BUILD_GOARCH" ]; then
    GOARCH=${BUILD_GOARCH}
elif [ $(arch) = "aarch64" ] || [ $(arch) = "arm64" ]; then
    GOARCH=arm64
elif [ $(arch) = "x86_64" ]; then
    GOARCH=amd64
fi

GO_VERSION=1.20-alpine3.16

GOPROXY=${GOPROXY:-'https://goproxy.cn'}

if [ "$DISABLE_GOPROXY" == "true" ]; then
	GOPROXY=
fi
if [ -z "$GOOS" ]; then
	GOOS="linux"
fi
if [ "$DEBUG" ]; then
	set -x
fi
BRANCH=$(git symbolic-ref HEAD 2>/dev/null | cut -d"/" -f 3)
if [ -z "$VERSION" ]; then
	if [ -z "$TRAVIS_TAG" ]; then
		if [ -z "$TRAVIS_BRANCH" ]; then
			VERSION=$BRANCH-dev
		else
			VERSION=$TRAVIS_BRANCH-dev
		fi
	else
		VERSION=$TRAVIS_TAG
	fi
fi

buildTime=$(date +%F-%H)
git_commit=$(git log -n 1 --pretty --format=%h)

release_desc=${VERSION}-${git_commit}-${buildTime}
build_items=(api chaos mq webcli worker eventlog init-probe grctl resource-proxy)

##############################
## 构建二进制文件
##############################
build::binary() {
	echo "---> build binary:$1"
	home=$(pwd)
	local go_mod_cache="${home}/.cache"
	# 定义二进制文件路径
	local OUTPATH="./_output/binary/$GOOS/${BASE_NAME}-$1"
	# 定义各模块的 Dockerfile 路径
	local DOCKER_PATH="./hack/contrib/docker/$1"
	local BUILD_STACK_PATH="./hack/contrib/docker/buildstack"
	# 定义基础构建镜像
	local build_image="goodrain.me/golang-gcc-buildstack:${GO_VERSION}"
	local build_dir="./cmd/$1"
	local build_tag=""
	# 定义 Dockerfile 文件名称
	local DOCKERFILE_BASE=${BUILD_DOCKERFILE_BASE:-'Dockerfile'}
	# 根据架构选择不同的编译参数
    if [ "$GOARCH" = "amd64" ]; then
	    local build_args="-w -s -X github.com/goodrain/rainbond/cmd.version=${release_desc}"
	elif [ "$GOARCH" = "arm64" ]; then
	    local build_args="-w -s -linkmode external -extldflags '-static' -X github.com/goodrain/rainbond/cmd.version=${release_desc}"
    fi
	# 判断是否存在 ignorebuild 文件，存在则不构建
	if [ -f "${DOCKER_PATH}/ignorebuild" ]; then
		return
	fi
		# Build 打包二进制使用的基本镜像
    docker build -t goodrain.me/golang-gcc-buildstack:${GO_VERSION} --build-arg GO_VERSION="${GO_VERSION}" -f "${BUILD_STACK_PATH}/${DOCKERFILE_BASE}" "${BUILD_STACK_PATH}"

	CGO_ENABLED=1
	if [ "$1" = "eventlog" ]; then
		if [ "$GOARCH" = "arm64" ]; then
			DOCKERFILE_BASE="Dockerfile.arm"
		fi
		# Build eventlog 使用的基本镜像
		docker build -t goodrain.me/event-build:v1 -f "${DOCKER_PATH}/build/${DOCKERFILE_BASE}" "${DOCKER_PATH}/build/"
		build_image="goodrain.me/event-build:v1"
		local build_args="-w -s -X github.com/goodrain/rainbond/cmd.version=${release_desc}"
	elif [ "$1" = "chaos" ]; then
		build_dir="./cmd/builder"
	elif [ "$1" = "monitor" ]; then
		CGO_ENABLED=0
	elif [ "$1" = "grctl" ] || [ "$1" = "mesh-data-panel" ]; then
		local build_args="-w -s -linkmode external -extldflags '-static' -X github.com/goodrain/rainbond/cmd.version=${release_desc}"
	fi
	# 在 Docker 环境中编译二进制文件
	docker run --rm -e CGO_ENABLED=${CGO_ENABLED} -e GOARCH=${GOARCH} -e GOPROXY=${GOPROXY} -e GOOS="${GOOS}" -v "${go_mod_cache}":/go/pkg/mod -v "$(pwd)":${WORK_DIR} -w ${WORK_DIR} ${build_image} go build -ldflags "${build_args}" -tags "${build_tag}" -o "${OUTPATH}" ${build_dir}
	if [ "$GOOS" = "windows" ]; then
		mv "$OUTPATH" "${OUTPATH}.exe"
	fi
	if [ "$GOARCH" = "amd64" ]; then
		# 安装压缩二进制文件体积工具
		if [ ! $(which upx) ]; then
		  sudo apt-get install -y upx
		fi
			# 执行压缩二进制文件体积
 	  	sudo upx --best --lzma "${OUTPATH}"
#	elif [ "$GOARCH" = "arm64" ]; then
#		wget https://rainbond-pkg.oss-cn-shanghai.aliyuncs.com/upx/upx-4.0.2-arm64_linux/upx
#		chmod +x upx
#		mv upx /usr/local/bin/upx
#		upx --best --lzma "${OUTPATH}"
	fi
}

##############################
## 构建 Docker 镜像
##############################
build::image() {
	# 定义二进制文件路径
	local OUTPATH="./_output/binary/$GOOS/${BASE_NAME}-$1"
	# 定义各模块的 Dockerfile 路径
	local build_image_dir="./_output/image/$1/"
	# 定义源路径
	local source_dir="./hack/contrib/docker/$1"
	# 定义 Dockerfile 构建基础镜像版本
	local BASE_IMAGE_VERSION=${BUILD_BASE_IMAGE_VERSION:-'3.16'}
	# 定义 Dockerfile 名称
	local DOCKERFILE_BASE=${BUILD_DOCKERFILE_BASE:-'Dockerfile'}
	mkdir -p "${build_image_dir}"
	chmod 777 "${build_image_dir}"
	# 判断是否需要构建镜像，存在 ignorebuild 文件则不构建
	if [ ! -f "${source_dir}/ignorebuild" ]; then
		if [ !${CACHE} ] || [ ! -f "${OUTPATH}" ]; then
			build::binary "$1"
		fi
		cp "${OUTPATH}" "${build_image_dir}"
	fi
	cp -r ${source_dir}/* "${build_image_dir}"
	pushd "${build_image_dir}"
	echo "---> build image:$1"
	# 判断是否为 ARM64 架构，选择不同的基础镜像版本，没有 Arm 需求略过
	if [ "$GOARCH" = "arm64" ]; then
		if [ "$1" = "gateway" ]; then
			BASE_IMAGE_VERSION="1.19.3.2-alpine"
			if [ "$ENABLE_WAF" == "true" ]; then
        BASE_IMAGE_VERSION="1.21.4.1-waf-arm"
      fi
		elif [ "$1" = "eventlog" ]; then
			DOCKERFILE_BASE="Dockerfile.arm"
		elif [ "$1" = "mesh-data-panel" ]; then
			DOCKERFILE_BASE="Dockerfile.arm"
		elif [ "$1" = "api" ]; then
		    curl -o helm https://pkg.goodrain.com/pkg/helm-arm64
		    chmod +x helm
		fi
	else
		# gateway 目前不使用该脚本打包，可忽略
		if [ "$1" = "gateway" ]; then
			BASE_IMAGE_VERSION="1.19.3.2"
			if [ "$ENABLE_WAF" == "true" ]; then
        BASE_IMAGE_VERSION="1.21.4.1-waf"
      fi
		elif [ "$1" = "api" ]; then
			curl -o helm https://pkg.goodrain.com/pkg/helm
			chmod +x helm
		fi
	fi
	if [ "$1" = "shell" ]; then
		cp ../../binary/$GOOS/rainbond-grctl grctl
	fi
	# 构建 Docker 镜像
	docker build --build-arg RELEASE_DESC="${release_desc}" --build-arg BASE_IMAGE_VERSION="${BASE_IMAGE_VERSION}" --build-arg GOARCH="${GOARCH}" -t "${IMAGE_BASE_NAME}/rbd-$1:${VERSION}" -f "${DOCKERFILE_BASE}" .
	# 验证 Docker 镜像
	docker run --rm "${IMAGE_BASE_NAME}/rbd-$1:${VERSION}" version
	if [ $? -ne 0 ]; then
		echo "image version is different ${release_desc}"
		exit 1
	fi
	# 测试 Docker 镜像
	if [ -f "${source_dir}/test.sh" ]; then
		"${source_dir}/test.sh" "${IMAGE_BASE_NAME}/rbd-$1:${VERSION}"
	fi
	# 推送 Docker 镜像
	if [ "$2" = "push" ]; then
		# 判断是否存在 Docker 用户名，存在则推送，否则不推送
		if [ $DOCKER_USERNAME ]; then
			docker login -u "$DOCKER_USERNAME" -p "$DOCKER_PASSWORD"
			docker push "${IMAGE_BASE_NAME}/rbd-$1:${VERSION}"
		fi
		# 判断是否存在国内镜像仓库用户名，存在则推送，否则不推送
		if [ "${DOMESTIC_DOCKER_USERNAME}" ]; then
			docker tag "${IMAGE_BASE_NAME}/rbd-$1:${VERSION}" "${DOMESTIC_BASE_NAME}/${DOMESTIC_NAMESPACE}/rbd-$1:${VERSION}"
			docker login -u "$DOMESTIC_DOCKER_USERNAME" -p "$DOMESTIC_DOCKER_PASSWORD" "${DOMESTIC_BASE_NAME}"
			docker push "${DOMESTIC_BASE_NAME}/${DOMESTIC_NAMESPACE}/rbd-$1:${VERSION}"
		fi
	fi
	popd
	# 删除构建镜像目录
	rm -rf "${build_image_dir}"
}

# 构建所有镜像
build::image::all() {
	for item in "${build_items[@]}"; do
		build::image "$item" "$1"
	done
}

# 构建所有二进制文件
build::binary::all() {
	for item in "${build_items[@]}"; do
		build::binary "$item" "$1"
	done
}

case $1 in
binary)
	if [ "$2" = "all" ]; then
		build::binary::all "$2"
	else
		build::binary "$2"
	fi
	;;
*)
	if [ "$1" = "all" ]; then
		build::image::all "$2"
	else
		build::image "$1" "$2"
	fi
	;;
esac
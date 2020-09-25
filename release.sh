#!/bin/bash
set -o errexit

# define package name
WORK_DIR=/go/src/github.com/goodrain/rainbond
BASE_NAME=rainbond
IMAGE_BASE_NAME=rainbond
if [ "$BUILD_IMAGE_BASE_NAME" ]; 
then 
IMAGE_BASE_NAME=${BUILD_IMAGE_BASE_NAME}
fi
CACHE=${CACHE:true}
GO_VERSION=1.13

GOPROXY=${GOPROXY:-'https://goproxy.io'}
if [ -z "$GOOS" ];then
  GOOS="linux"
fi
if [ "$DEBUG" ];then
  set -x
fi
BRANCH=$(git symbolic-ref HEAD 2>/dev/null | cut -d"/" -f 3)
if [ -z "$VERSION" ];then
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
build_items=(api chaos gateway monitor mq webcli worker eventlog init-probe mesh-data-panel grctl node resource-proxy)


build::binary() {
	echo "---> build binary:$1"
	home=$(pwd)
	local go_mod_cache="${home}/.cache"
	local OUTPATH="./_output/binary/$GOOS/${BASE_NAME}-$1"
	local DOCKER_PATH="./hack/contrib/docker/$1"
	local build_image="golang:${GO_VERSION}"
	local build_args="-w -s -X github.com/goodrain/rainbond/cmd.version=${release_desc}"
	local build_dir="./cmd/$1"
	local build_tag=""
	if [ ! -f "${DOCKER_PATH}/ignorebuild" ];then
		return
	fi
	CGO_ENABLED=1
	if [ "$1" = "eventlog" ];then
		docker build -t goodraim.me/event-build:v1 "${DOCKER_PATH}/build"
		build_image="goodraim.me/event-build:v1"
	elif [ "$1" = "chaos" ];then
		build_dir="./cmd/builder"
	elif [ "$1" = "gateway" ];then
		build_image="golang:1.13-alpine"
	elif [ "$1" = "monitor" ];then
		CGO_ENABLED=0
    fi
	docker run --rm -e CGO_ENABLED=${CGO_ENABLED} -e GOPROXY=${GOPROXY} -e GOOS="${GOOS}" -v "${go_mod_cache}":/go/pkg/mod  -v "$(pwd)":${WORK_DIR} -w ${WORK_DIR} -it ${build_image} go build -ldflags "${build_args}" -tags "${build_tag}"  -o "${OUTPATH}" ${build_dir}
	if [ "$GOOS" = "windows" ];then
	    mv "$OUTPATH"  "${OUTPATH}.exe"
	fi
}

build::image() {
	local OUTPATH="./_output/binary/$GOOS/${BASE_NAME}-$1"
	local build_image_dir="./_output/image/$1/"
	local source_dir="./hack/contrib/docker/$1"
	sudo mkdir -p "${build_image_dir}"
	sudo chmod 777 "${build_image_dir}"
	if [ ! -f "${source_dir}/ignorebuild" ];then
		if [  !${CACHE} ] || [ ! -f "${OUTPATH}" ];then
			build::binary "$1"
		fi
		sudo cp "${OUTPATH}" "${build_image_dir}"
	fi	
	sudo cp -r ${source_dir}/* "${build_image_dir}"
	pushd "${build_image_dir}"
		echo "---> build image:$1"
		sudo sed "s/__RELEASE_DESC__/${release_desc}/" Dockerfile > Dockerfile.release
		sudo docker build -t "${IMAGE_BASE_NAME}/rbd-$1:${VERSION}" -f Dockerfile.release .
		sudo docker run -it --rm "${IMAGE_BASE_NAME}/rbd-$1:${VERSION}" version
		if [  $? -ne 0 ];then
			echo "image version is different ${release_desc}"
			exit 1
		fi
		if [ -f "${source_dir}/test.sh" ];then
			"${source_dir}/test.sh" "${IMAGE_BASE_NAME}/rbd-$1:${VERSION}"
		fi
		if [ "$2" = "push" ];then
		    sudo docker login -u "$DOCKER_USERNAME" -p "$DOCKER_PASSWORD"
			sudo docker push "${IMAGE_BASE_NAME}/rbd-$1:${VERSION}"
			if [ "${DOMESTIC_BASE_NAME}" ];
			then
				sudo docker tag "${IMAGE_BASE_NAME}/rbd-$1:${VERSION}" "${DOMESTIC_BASE_NAME}/${DOMESTIC_NAMESPACE}/rbd-$1:${VERSION}"
				sudo docker login -u "$DOMESTIC_DOCKER_USERNAME" -p "$DOMESTIC_DOCKER_PASSWORD" "${DOMESTIC_BASE_NAME}"
				sudo docker push "${DOMESTIC_BASE_NAME}/${DOMESTIC_NAMESPACE}/rbd-$1:${VERSION}"
			fi
		fi
	popd
	sudo rm -rf "${build_image_dir}"
}

build::image::all(){
	for item in "${build_items[@]}"
	do
		build::image "$item" "$1"
	done
}

build::binary::all(){
	for item in "${build_items[@]}"
	do
		build::binary "$item" "$1"
	done
}

case $1 in
	binary)
	    if [ "$2" = "all" ];then
			build::binary::all "$2"
		else
		    build::binary "$2"	
		fi	
	;;
	*)
		if [ "$1" = "all" ];then
			build::image::all "$2"
		else
			build::image "$1" "$2"
		fi
	;;
esac

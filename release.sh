#!/bin/bash
set -o errexit

# define package name
WORK_DIR=/go/src/github.com/goodrain/rainbond
BASE_NAME=rainbond
GO_VERSION=1.11

VERSION=5.0
buildTime=$(date +%F-%H)
git_commit=$(git log -n 1 --pretty --format=%h)

release_desc=${VERSION}-${git_commit}-${buildTime}

build::node() {
	local releasedir=./.release
	local distdir=$releasedir/dist/usr/local
    [ ! -d "$distdir" ] && mkdir -p $distdir/bin || rm -rf $distdir/bin/*
	echo "---> Build Binary For RBD"
	echo "rbd plugins version:$release_desc"
	echo "build node"
    docker run --rm -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:${GO_VERSION} go build -ldflags "-w -s -X github.com/goodrain/rainbond/cmd.version=${release_desc}"  -o $releasedir/dist/usr/local/bin/node ./cmd/node
	echo "build grctl"
	docker run --rm -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:${GO_VERSION} go build -ldflags "-w -s -X github.com/goodrain/rainbond/cmd.version=${release_desc}"  -o $releasedir/dist/usr/local/bin/grctl ./cmd/grctl
	echo "build certutil"
	docker run --rm -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:${GO_VERSION} go build -ldflags "-w -s -X github.com/goodrain/rainbond/cmd.version=${release_desc}"  -o $releasedir/dist/usr/local/bin/grcert ./cmd/certutil
	pushd $distdir
	tar zcf pkg.tgz `find . -maxdepth 1|sed 1d`

	cat >Dockerfile <<EOF
FROM alpine:3.6
COPY pkg.tgz /
EOF
	docker build -t ${BASE_NAME}/cni:rbd_v$VERSION .
	docker push ${BASE_NAME}/cni:rbd_v$VERSION 
	popd
}

build::binary() {
	echo "---> build binary:$1"
	local DOCKER_PATH=./hack/contrib/docker/$1
	HOME=`pwd`
	if [ "$1" = "eventlog" ];then
		docker build -t goodraim.me/event-build:v1 ${DOCKER_PATH}/build
		docker run --rm -v `pwd`:${WORK_DIR} -w ${WORK_DIR} goodraim.me/event-build:v1 go build  -ldflags "-w -s -X github.com/goodrain/rainbond/cmd.version=${release_desc}"  -o ${DOCKER_PATH}/${BASE_NAME}-$1 ./cmd/eventlog
	elif [ "$1" = "chaos" ];then
		docker run --rm -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:${GO_VERSION} go build -ldflags "-w -s -X github.com/goodrain/rainbond/cmd.version=${release_desc}"  -o ${DOCKER_PATH}/${BASE_NAME}-$1 ./cmd/builder
	elif [ "$1" = "monitor" ];then
		docker run --rm -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:${GO_VERSION} go build -ldflags "-w -s -extldflags '-static' -X github.com/goodrain/rainbond/cmd.version=${release_desc}" -tags 'netgo static_build' -o ${DOCKER_PATH}/${BASE_NAME}-$1 ./cmd/$1
	else
		docker run --rm -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:${GO_VERSION} go build -ldflags "-w -s -X github.com/goodrain/rainbond/cmd.version=${release_desc}"  -o ${DOCKER_PATH}/${BASE_NAME}-$1 ./cmd/$1
	fi
}

build::image() {
	local REPO_PATH="$PWD"
	pushd ./hack/contrib/docker/$1
		echo "---> build binary:$1"
		local DOCKER_PATH="./hack/contrib/docker/$1"
		if [ "$1" = "eventlog" ];then
			docker build -t goodraim.me/event-build:v1 build
			docker run --rm -v ${REPO_PATH}:${WORK_DIR} -w ${WORK_DIR} goodraim.me/event-build:v1 go build  -ldflags "-w -s -X github.com/goodrain/rainbond/cmd.version=${release_desc}"  -o ${DOCKER_PATH}/${BASE_NAME}-$1 ./cmd/eventlog
		elif [ "$1" = "chaos" ];then
			docker run --rm -v ${REPO_PATH}:${WORK_DIR} -w ${WORK_DIR} -it golang:${GO_VERSION} go build -ldflags "-w -s -X github.com/goodrain/rainbond/cmd.version=${release_desc}"  -o ${DOCKER_PATH}/${BASE_NAME}-$1 ./cmd/builder
		elif [ "$1" = "monitor" ];then
			docker run --rm -v ${REPO_PATH}:${WORK_DIR} -w ${WORK_DIR} -it golang:${GO_VERSION} go build -ldflags "-w -s -extldflags '-static' -X github.com/goodrain/rainbond/cmd.version=${release_desc}" -tags 'netgo static_build' -o ${DOCKER_PATH}/${BASE_NAME}-$1 ./cmd/$1
		else
			docker run --rm -v ${REPO_PATH}:${WORK_DIR} -w ${WORK_DIR} -it golang:${GO_VERSION} go build -ldflags "-w -s -X github.com/goodrain/rainbond/cmd.version=${release_desc}"  -o ${DOCKER_PATH}/${BASE_NAME}-$1 ./cmd/$1
		fi
		echo "---> build image:$1"
		sed "s/__RELEASE_DESC__/${release_desc}/" Dockerfile > Dockerfile.release
		docker build -t ${BASE_NAME}/rbd-$1:${VERSION} -f Dockerfile.release .
		docker push ${BASE_NAME}/rbd-$1:${VERSION}
		rm -f ./Dockerfile.release
		rm -f ./${BASE_NAME}-$1
	popd
}

build::all(){
	local build_items=(api chaos entrance monitor mq webcli worker eventlog)
	for item in ${build_items[@]}
	do
		build::image $item
	done
	build::node
}

case $1 in
	node)
		build::node
	;;
	*)
		if [ "$1" = "all" ];then
			build::all
		else
			build::image $1
		fi
	;;
esac

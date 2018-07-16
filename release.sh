#!/bin/bash
set -o errexit

# define package name
PROGRAM="gr-rainbond"
WORK_DIR=/go/src/github.com/goodrain/rainbond
DOCKER_PATH=./hack/contrib/docker/$1
BASE_NAME=rainbond
releasedir=./.release
distdir=${releasedir}/dist


VERSION=$(git branch | grep '^*' | cut -d ' ' -f 2 | awk -F'V' '{print $2}')
buildTime=$(date +%F-%H)
git_commit=$(git log -n 1 --pretty --format=%h)
if [ -z "$VERSION" ];then
    VERSION=cloud
fi
release_desc=${VERSION}-${git_commit}-${buildTime}

function prepare() {
	rm -rf $releasedir
    mkdir -pv $releasedir/{tmp,dist}
    path=$PWD
    [ ! -d "$distdir/usr/local/" ] && mkdir -p $distdir/usr/local/bin

}

function build() {
	echo "---> Build Binary For RBD"
	echo "rbd plugins version:$release_desc"
	
	echo "build node"
    docker run --rm -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:1.8.3 go build -ldflags "-w -s -X github.com/goodrain/rainbond/cmd.version=${release_desc}"  -o $releasedir/dist/usr/local/bin/node ./cmd/node
	echo "build grctl"
	docker run --rm -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:1.8.3 go build -ldflags "-w -s -X github.com/goodrain/rainbond/cmd.version=${release_desc}"  -o $releasedir/dist/usr/local/bin/grctl ./cmd/grctl

	cd $releasedir/dist/usr/local/
	tar zcf pkg.tgz `find . -maxdepth 1|sed 1d`

	cat >Dockerfile <<EOF
FROM alpine:3.6
COPY pkg.tgz /
EOF
	docker build -t rainbond/cni:rbd_v3.6 .
	
}

function build::rpm() {
	echo "---> Make Build RPM"
	source "hack/build-rpm.sh"
}

function build::deb() {
	echo "---> Make Build DEB"
	source "hack/build-deb.sh"
}

function build::image() {
	echo "---> Build Image:$1 FOR RBD"
	
	if [ "$1" = "eventlog" ];then
		docker build -t goodraim.me/event-build:v1 ${DOCKER_PATH}/build
		docker run --rm -v `pwd`:${WORK_DIR} -w ${WORK_DIR} goodraim.me/event-build:v1 go build  -ldflags "-w -s -X github.com/goodrain/rainbond/cmd.version=${release_desc}"  -o ${DOCKER_PATH}/${BASE_NAME}-$1 ./cmd/eventlog
	elif [ "$1" = "chaos" ];then
		docker run --rm -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:1.8.3 go build -ldflags "-w -s -X github.com/goodrain/rainbond/cmd.version=${release_desc}"  -o ${DOCKER_PATH}/${BASE_NAME}-$1 ./cmd/builder
	elif [ "$1" = "monitor" ];then
		docker run --rm -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:1.8.3 go build -ldflags "-w -s -extldflags '-static' -X github.com/goodrain/rainbond/cmd.version=${release_desc}" -tags 'netgo static_build' -o ${DOCKER_PATH}/${BASE_NAME}-$1 ./cmd/$1
		#go build -ldflags "-w -s -extldflags '-static'" -tags 'netgo static_build' -o ${DOCKER_PATH}/${BASE_NAME}-$1 ./cmd/monitor
	else
		docker run --rm -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:1.8.3 go build -ldflags "-w -s -X github.com/goodrain/rainbond/cmd.version=${release_desc}"  -o ${DOCKER_PATH}/${BASE_NAME}-$1 ./cmd/$1
	fi
	cd  ${DOCKER_PATH}
	sed "s/__RELEASE_DESC__/${release_desc}/" Dockerfile > Dockerfile.release
	docker build -t ${BASE_NAME}/rbd-$1:${VERSION} -f Dockerfile.release .
	rm -f ./Dockerfile.release
	rm -f ./${BASE_NAME}-$1
}

case $1 in
	build)
		prepare
		build
	;;
	pkg)
		prepare
		build
		build::rpm
		build::deb
	;;
	*)
		build::image $1
	;;
esac

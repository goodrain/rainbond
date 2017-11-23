#!/bin/bash
set -o errexit

# define package name
PROGRAM="gr-rainbond"
WORK_DIR=/go/src/github.com/goodrain/rainbond
DOCKER_PATH=./hack/contrib/docker/$1
BASE_NAME=rainbond
releasedir=./.release
distdir=${releasedir}/dist
gaops='git@code.goodrain.com:goodrain/gaops.git'


gitDescribe=$(git describe --tag|sed 's/^v//')
describe_items=($(echo $gitDescribe | tr '-' ' '))
describe_len=${#describe_items[@]}
VERSION=${describe_items[0]}
git_commit=$(git log -n 1 --pretty --format=%h)
if [ $describe_len -ge 3 ];then
    buildRelease=${describe_items[-2]}.${describe_items[-1]}
else
    buildRelease=0.$git_commit
fi
if [ -z "$VERSION" ];then
    VERSION=3.4
fi

function prepare() {
	rm -rf $releasedir
    mkdir -pv $releasedir/{tmp,dist}
    path=$PWD
    git clone $gaops  $releasedir/tmp
    [ ! -d "$distdir/usr/local/" ] && mkdir -p $distdir/usr/local/bin
    [ ! -d "$distdir/usr/share/gr-rainbond-node/gaops/" ] && mkdir -pv $distdir/usr/share/gr-rainbond-node/gaops
    cd $releasedir/tmp
    rm -rf .git
    
    tar zcf  ../dist/usr/share/gr-rainbond-node/gaops/gaops.tgz ./ 
    cd $path
    rm -rf $releasedir/tmp
}

function build() {
	echo "---> Build Binary For ACP"
	echo "build rainbond-node"
    docker run -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:1.8.3 go build -ldflags '-w -s'  -o $releasedir/dist/usr/local/bin/${BASE_NAME}-node ./cmd/node
	echo "build rainbond-grctl"
	docker run -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:1.8.3 go build -ldflags '-w -s'  -o $releasedir/dist/usr/local/bin/${BASE_NAME}-grctl ./cmd/grctl
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
	echo "---> Build Image:$1 FOR ACP"
	
	git_commit=$(git log -n 1 --pretty --format=%h)
    branch_info=($(git branch | grep '^*' | cut -d ' ' -f 2 | tr '-' " "))
    release_desc=${branch_info}-${VERSION}-${buildRelease}
	if [ "$1" = "eventlog" ];then
		docker build -t goodraim.me/event-build:v1 ${DOCKER_PATH}/build
		docker run --rm -v `pwd`:${WORK_DIR} -w ${WORK_DIR} goodraim.me/event-build:v1 go build  -ldflags '-w -s'  -o ${DOCKER_PATH}/${BASE_NAME}-$1 ./cmd/eventlog
	elif [ "$1" = "chaos" ];then
		docker run -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:1.8.3 go build -ldflags '-w -s'  -o ${DOCKER_PATH}/${BASE_NAME}-$1 ./cmd/builder
	else
		docker run -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:1.8.3 go build -ldflags '-w -s'  -o ${DOCKER_PATH}/${BASE_NAME}-$1 ./cmd/$1
	fi
	cd  ${DOCKER_PATH}
	sed "s/__RELEASE_DESC__/${release_desc}/" Dockerfile > Dockerfile.release
	docker build -t hub.goodrain.com/${BASE_NAME}/rbd-$1:${VERSION} -f Dockerfile.release .
	docker tag hub.goodrain.com/${BASE_NAME}/rbd-$1:${VERSION} ${BASE_NAME}/rbd-$1:${VERSION}
	rm -f ./Dockerfile.release
	rm -f ./${BASE_NAME}-$1
}

case $1 in
	build)
		prepare
		build
	;;
	rpm)
		prepare
		build
		build::rpm
	;;
	deb)
		prepare
		build
		build::deb
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
#!/bin/bash
set -o errexit

# define package name
PROGRAM="gr-rainbond"
WORK_DIR=/go/src/github.com/goodrain/rainbond
DOCKER_PATH=./hack/contrib/docker/$1
BASE_NAME=rainbond
releasedir=./.release
distdir=${releasedir}/dist
gaopsdir=/hack/contrib/docker/node/gaops


gitDescribe=$(git describe --tag|sed 's/^v//')
describe_items=($(echo $gitDescribe | tr '-' ' '))
branch_info=($(git branch | grep '^*' | cut -d ' ' -f 2))
describe_len=${#describe_items[@]}
VERSION=$(git branch | grep '^*' | cut -d ' ' -f 2 | tr '-' " " | awk '{print $2}')
git_commit=$(git log -n 1 --pretty --format=%h)

if [ $describe_len -ge 3 ];then
    #buildRelease=${describe_items[-2]}.${describe_items[-1]}
	buildRelease=${describe_items[*]: -2:1}.${describe_items[*]: -1}
else
    buildRelease=0.$git_commit
fi
if [ -z "$VERSION" ];then
    VERSION=cloud
fi

release_desc=${branch_info}-${VERSION}-${buildRelease}

function prepare() {
	rm -rf $releasedir
    mkdir -pv $releasedir/{tmp,dist}
    path=$PWD
    #git clone $gaops  $releasedir/tmp
    [ ! -d "$distdir/usr/local/" ] && mkdir -p $distdir/usr/local/bin
    [ ! -d "$distdir/usr/share/gr-rainbond-node/gaops/" ] && mkdir -pv $distdir/usr/share/gr-rainbond-node/gaops
    cd $releasedir/tmp
    cp -a $path$gaopsdir/* ./
    tar zcf  ../dist/usr/share/gr-rainbond-node/gaops/gaops.tgz ./ 
    cd $path
    rm -rf $releasedir/tmp
}

function build() {
	echo "---> Build Binary For ACP"
	echo "build rainbond-node"
    docker run --rm -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:1.8.3 go build -ldflags '-w -s'  -o $releasedir/dist/usr/local/bin/${BASE_NAME}-node ./cmd/node
	echo "grctl version:$release_desc"
	sed -i "s/0.0.0/$release_desc/g" ./cmd/grctl/option/version.go
	echo "build rainbond-grctl"
	docker run --rm -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:1.8.3 go build -ldflags '-w -s'  -o $releasedir/dist/usr/local/bin/${BASE_NAME}-grctl ./cmd/grctl
	sed -i "s/$release_desc/0.0.0/g" ./cmd/grctl/option/version.go
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
	
	if [ "$1" = "eventlog" ];then
		docker build -t goodraim.me/event-build:v1 ${DOCKER_PATH}/build
		docker run --rm -v `pwd`:${WORK_DIR} -w ${WORK_DIR} goodraim.me/event-build:v1 go build  -ldflags '-w -s'  -o ${DOCKER_PATH}/${BASE_NAME}-$1 ./cmd/eventlog
	elif [ "$1" = "chaos" ];then
		docker run --rm -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:1.8.3 go build -ldflags '-w -s'  -o ${DOCKER_PATH}/${BASE_NAME}-$1 ./cmd/builder
	else
		docker run --rm -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:1.8.3 go build -ldflags '-w -s'  -o ${DOCKER_PATH}/${BASE_NAME}-$1 ./cmd/$1
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
	dev_deb)
		build::deb
	;;
	dev_rpm)
		build::rpm
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
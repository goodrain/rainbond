#!/bin/bash
set -o errexit

# define package name
PROGRAM="gr-rainbond"
WORK_DIR=/go/src/github.com/goodrain/rainbond
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

case $1 in
	build)
		prepare
		build
	;;
	rpm)
		build::rpm
	;;
	deb)
		build::deb
	;;
	*)
		prepare
		build
		build::rpm
		build::deb
	;;
esac
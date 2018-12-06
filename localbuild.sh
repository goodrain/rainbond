#!/bin/bash
set -o errexit

# define package name
WORK_DIR=/go/src/github.com/goodrain/rainbond
BASE_NAME=rainbond
releasedir=./.release
distdir=${releasedir}/dist
GO_VERSION=1.11

VERSION=5.0
buildTime=$(date +%F-%H)
git_commit=$(git log -n 1 --pretty --format=%h)
release_desc=${VERSION}-${git_commit}-${buildTime}

function prepare() {
	rm -rf $releasedir
    mkdir -pv $releasedir/{tmp,dist}
    path=$PWD
    [ ! -d "$distdir/usr/local/" ] && mkdir -p $distdir/usr/local/bin
}

build_items=(api builder entrance grctl monitor mq node webcli worker eventlog)

function localbuild() {
	if [ "$1" = "all" ];then
		for item in ${build_items[@]}
		do
    		echo "build local ${item}"
    		go build -ldflags "-w -s -X github.com/goodrain/rainbond/cmd.version=${release_desc}"  -o _output/${GOOS}/${VERSION}/rainbond-$item ./cmd/$item
		done	
	else
		echo "build local $1 ${VERSION}"

		outputname="_output/${GOOS}/${VERSION}/rainbond-$1"
		if [ "$GOOS" = "windows" ];then
			outputname="_output/${GOOS}/${VERSION}/rainbond-$1.exe"
		fi
		ldflags="-w -s -X github.com/goodrain/rainbond/cmd.version=${release_desc}"
		if [ "$STATIC" = "true" ];then
		    ldflags="${ldflags} -extldflags '-static'"
		fi
		go build -ldflags "${ldflags}"  -o ${outputname} ./cmd/$1
	fi
}

case $1 in
	*)
		prepare
		if [ "$1" = "all" ];then
			for item in ${build_items[@]}
			do
			  localbuild $item
			done	
	    fi
		localbuild $1
	;;
esac

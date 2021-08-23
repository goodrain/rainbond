#!/bin/bash
set -o errexit

# define package name
releasedir=./.release
distdir=${releasedir}/dist

VERSION=$(git symbolic-ref HEAD 2>/dev/null | cut -d"/" -f 3)
buildTime=$(date +%F-%H)
git_commit=$(git log -n 1 --pretty --format=%h)
release_desc=${VERSION}-${git_commit}-${buildTime}

function prepare() {
	rm -rf $releasedir
	mkdir -pv $releasedir/{tmp,dist}
	[ ! -d "$distdir/usr/local/" ] && mkdir -p $distdir/usr/local/bin
}

build_items=(api builder grctl monitor mq node-proxy webcli worker eventlog init-probe mesh-data-panel)

function localbuild() {
	if [ "$1" = "all" ]; then
		for item in "${build_items[@]}"; do
			echo "build local ${item}"
			go build -ldflags "-X github.com/goodrain/rainbond/cmd.version=${release_desc}" -o _output/${GOOS}/${VERSION}/rainbond-$item ./cmd/$item
		done
	else
		echo "build local $1 ${VERSION}"

		outputname="_output/${GOOS}/${VERSION}/rainbond-$1"
		if [ "$GOOS" = "windows" ]; then
			outputname="_output/${GOOS}/${VERSION}/rainbond-$1.exe"
		fi
		ldflags="-X github.com/goodrain/rainbond/cmd.version=${release_desc}"
		if [ "$STATIC" = "true" ]; then
			ldflags="${ldflags} -extldflags '-static'"
		fi
		go build -v -ldflags "${ldflags}" -o ${outputname} ./cmd/$1
	fi
}

case $1 in
*)
	prepare
	if [ "$1" = "all" ]; then
		for item in "${build_items[@]}"; do
			localbuild $item
		done
	fi
	localbuild $1
	;;
esac

#!/bin/bash
set -e

(
	debbuild_root=${releasedir}/deb
	for release_dir in $(find hack/deb/* -maxdepth 0 -type d)
	do
		release=${release_dir##*/}
		debbuildRelease=${buildRelease}-${GIT_CM}~${release}
		RELEASE_PATH=$debbuild_root/$release/${PROGRAM}-${VERSION}-${debbuildRelease}
		
		rm -rf $debbuild_root/$release
		mkdir -pv $RELEASE_PATH

		cp -a $release_dir/debian $RELEASE_PATH/debian

		mkdir -p $RELEASE_PATH/usr/bin
		[ -d $releasedir/dist ] && (
			rsync -a hack/contrib/ $releasedir/dist/
			[ -d build/node/gaops/jobs ] && (
				mkdir -p $releasedir/dist/usr/share/gr-rainbond-node/gaops
				rsync -a build/node/gaops/ $releasedir/dist/usr/share/gr-rainbond-node/gaops
			)
			rsync -a $releasedir/dist/ $RELEASE_PATH
		)

		BUILD_IMAGE=inner.goodrain.com/deb-build:$release
		docker run --rm -v $PWD/$debbuild_root/$release:/debbuild -w /debbuild/${PROGRAM}-${VERSION}-${debbuildRelease} -e VERSION=$VERSION -e debRelease=$debbuildRelease $BUILD_IMAGE build
	done
) 2>&1
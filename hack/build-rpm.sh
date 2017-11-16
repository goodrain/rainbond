#!/bin/bash
set -e
set -x

(
	rpmbuild_root=${releasedir}/rpm
	for release_dir in $(find hack/rpm/* -maxdepth 0 -type d)
	do
		release=${release_dir##*/}
		RELEASE_PATH=$rpmbuild_root/$release
		rm -rf $RELEASE_PATH
		mkdir -p $RELEASE_PATH/{BUILD,BUILDROOT,RPMS,SOURCES,SPECS,SRPMS}
		SOURCE_TARGET=$rpmbuild_root/$release/SOURCES
		SPEC_TARGET=$rpmbuild_root/$release/SPECS

		cp -a $release_dir/SPECS/* $SPEC_TARGET

		rsync -a $release_dir/files/ $SOURCE_TARGET/${PROGRAM}-${VERSION}
		cp -a $distdir/* $SOURCE_TARGET/${PROGRAM}-${VERSION}/
		cd $SOURCE_TARGET && tar zcf ${PROGRAM}-${VERSION}.tar.gz ${PROGRAM}-${VERSION} && cd -

		BUILD_IMAGE=inner.goodrain.com/rpm-build:$release
		for file in $(find $SPEC_TARGET -name '*.spec')
		do
			docker run --rm -v $PWD/$RELEASE_PATH:/root/rpmbuild -e rpmRelease=$buildRelease -e VERSION=$VERSION $BUILD_IMAGE SPECS/${file##*/}
		done
	done
) 2>&1
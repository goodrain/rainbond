#!/bin/bash
set -o errexit

# define package name
PROGRAM="gr-acp-node"
	
releasedir=./.release
distdir=${releasedir}/dist
acp_actions='git@code.goodrain.com:acp/node_actions.git'


gitDescribe=$(git describe --tag|sed 's/^v//')
describe_items=($(echo $gitDescribe | tr '-' ' '))
describe_len=${#describe_items[@]}
VERSION=${describe_items[0]}
if [ $describe_len -ge 3 ];then
    buildRelease=${describe_items[-2]}.${describe_items[-1]}
else
    buildRelease=0
fi



release_name='enterprise'    # enterprise | community
release_version='3.4'   # 3.2 | 2017.05

git_commit=$(git log -n 1 --pretty --format=%h)

function prepare() {
	rm -rf $releasedir
    mkdir -p $releasedir/{tmp,dist}
    path=$PWD
    git clone $acp_actions  $releasedir/tmp
    [ ! -d "$distdir/usr/local/" ] && mkdir -p $distdir/usr/local/{acp-node,bin}
    cd $releasedir/tmp
    rm -rf .git
    
    tar zcvf ../dist/usr/local/acp-node/sh.tgz ./
    cd $path
    rm -rf $releasedir/tmp
}

function build() {
	echo "build image"
    docker run -it --rm -v `pwd`:/go/src/acp_node -w /go/src/acp_node/cmd golang:1.8.3 go build -ldflags '-w -s' -o acp-node
    mv $PWD/cmd/acp-node $releasedir/dist/usr/local/bin/
}

function build::rpm() {
	echo "---> Make Build RPM"
	source "hack/build-rpm.sh"
}

function add_repo() {
	echo "add deb/rpm package to repo"
    count_rpm=$(ls /root/release/docker_images/acp_node/.release/rpm/centos-7/RPMS/x86_64/ | wc -l)
    if [ $count_rpm -eq 1 ];then
        /root/bin/repo-ctl -r $release_version add /root/release/docker_images/acp_node/.release/rpm/centos-7/RPMS/x86_64/*.rpm
    fi
    echo "---> show package list"
    /root/bin/repo-ctl -r $release_version list
}


action=$1

case $action in
	build)
		prepare
		build
	;;
	rpm)
		build::rpm
	;;
	*)
		prepare
		build
		build::rpm
        add_repo
	;;
esac
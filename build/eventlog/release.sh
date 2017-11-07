#!/bin/bash
set -xe

image_name="acp_event_log"
release_type=$1
release_ver=$2

if [ "$release_type" == "" ];then
  echo "please input release type (community | enterprise | all ) and version"
  exit 1
fi

trap 'clean_tmp; exit' QUIT TERM EXIT

function clean_tmp() {
  echo "clean temporary file..."
  [ -f Dockerfile.release ] && rm -rf Dockerfile.release
  [ -f event_log ] && rm -rf event_log
}

function release(){
  release_name=$1      # enterprise | community
  release_version=$2   # 3.2 | 2017.05
  git checkout ${release_name}-${release_version}
  echo "pull newest code..."
  git pull

  # make bin
  make build-alpine

  # get commit sha
  git_commit=$(git log -n 1 --pretty --format=%h)


  # get git describe info
  release_desc=${release_name}-${release_version}-${git_commit}

  sed "s/__RELEASE_DESC__/${release_desc}/" Dockerfile > Dockerfile.release

  docker build -t hub.goodrain.com/dc-deploy/${image_name}:${release_version} -f Dockerfile.release .
  docker push hub.goodrain.com/dc-deploy/${image_name}:${release_version}
}

case $release_type in
"community")
    release $1 $release_ver
    ;;
"enterprise")
    release $1 $release_ver
    ;;
"all")
    release "community" $release_ver
    release "enterprise" $release_ver
    ;;
esac

#!/bin/bash
if [ "$1" = "bash" ];then
    exec /bin/bash
elif [ "$1" = "version" ];then
    echo "$RELEASE_DESC"
elif [ "$1" = "copy" ];then
    cp -a /run/rainbond-grctl /rootfs/usr/local/bin/
else
    exec /run/rainbond-grctl "$@"
fi
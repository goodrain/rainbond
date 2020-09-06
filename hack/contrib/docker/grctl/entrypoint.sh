#!/bin/bash
if [ "$1" = "bash" ];then
    exec /bin/bash
elif [ "$1" = "version" ];then
    /run/rainbond-grctl version
elif [ "$1" = "copy" ];then
    cp -a /run/rainbond-grctl /rootfs/usr/local/bin/
else
    exec /run/rainbond-grctl "$@"
fi
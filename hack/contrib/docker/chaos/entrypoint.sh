#!/bin/bash
if [ "$1" = "bash" ];then
    exec /bin/bash
elif [ "$1" = "version" ];then
    /run/rainbond-chaos version
else
    exec /sbin/tini -- /run/rainbond-chaos "$@"
fi
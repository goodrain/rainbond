#!/bin/bash
if [ "$1" = "bash" ];then
    exec /bin/bash
elif [ "$1" = "version" ];then
    /run/rainbond-worker version
else
    exec /sbin/tini -- /run/rainbond-worker "$@"
fi
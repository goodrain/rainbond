#!/bin/bash
if [ "$1" = "bash" ];then
    exec /bin/bash
elif [ "$1" = "version" ];then
    /run/rainbond-mq version
else
    exec /sbin/tini -- /run/rainbond-mq "$@"
fi
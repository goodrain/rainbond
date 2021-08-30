#!/bin/bash
if [ "$1" = "bash" ];then
    exec /bin/bash
elif [ "$1" = "version" ];then
    /run/rainbond-mq version
else
    exec /run/rainbond-mq $@
fi
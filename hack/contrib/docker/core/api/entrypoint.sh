#!/bin/bash
if [ "$1" = "bash" ];then
    exec /bin/bash
elif [ "$1" = "version" ];then
    /run/rainbond-api version
else
    exec /run/rainbond-api --start=true $@
fi
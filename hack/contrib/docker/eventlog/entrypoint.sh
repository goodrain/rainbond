#!/bin/bash
if [ "$1" = "bash" ];then
    exec /bin/bash
elif [ "$1" = "version" ];then
    /run/rainbond-eventlog version
else
    exec /run/rainbond-eventlog $@
fi
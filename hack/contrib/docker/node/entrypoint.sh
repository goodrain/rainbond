#!/bin/bash
if [ "$1" = "bash" ];then
    exec /bin/bash
elif [ "$1" = "version" ];then
    /run/rainbond-node version
else
    exec /bin/tini -- /run/rainbond-node $@
fi
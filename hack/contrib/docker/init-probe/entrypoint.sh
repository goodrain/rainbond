#!/bin/bash
if [ "$1" = "bash" ];then
    exec /bin/bash
elif [ "$1" = "version" ];then
    /run/rainbond-init-probe version
else
    exec /run/rainbond-init-probe $@
fi
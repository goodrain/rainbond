#!/bin/bash
if [ "$1" = "bash" ];then
    exec /bin/bash
elif [ "$1" = "version" ];then
    /run/rainbond-chaos version
else
    exec /run/rainbond-chaos $@
fi
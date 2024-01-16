#!/bin/ash
if [ "$1" = "bash" ];then
    exec /bin/ash
elif [ "$1" = "version" ];then
    /run/rainbond-monitor version
else
    exec /run/rainbond-monitor $@
fi
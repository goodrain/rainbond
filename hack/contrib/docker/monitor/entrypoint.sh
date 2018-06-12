#!/bin/ash
if [ "$1" = "bash" ];then
    exec /bin/ash
elif [ "$1" = "version" ];then
    echo ${RELEASE_DESC}
else
    exec /run/rainbond-monitor $@
fi
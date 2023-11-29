#!/bin/bash
if [ "$1" = "bash" ];then
    exec /bin/bash
elif [ "$1" = "version" ];then
    /usr/bin/rainbond-webcli version
else
    exec /bin/tini -- /usr/bin/rainbond-webcli $@
fi
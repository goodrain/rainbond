#!/bin/bash
if [ "$1" = "bash" ]; then
    exec /bin/bash
elif [ "$1" = "version" ]; then
    /run/rainbond-node-proxy version
else
    exec /run/rainbond-node-proxy $@
fi

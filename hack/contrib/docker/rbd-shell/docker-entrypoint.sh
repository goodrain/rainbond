#!/bin/sh
if [ "$1" = "bash" ];then
    exec /bin/sh
elif [ "${1}" = 'version' ];then
    echo "${RELEASE_DESC}"
else
    exec /docker-entrypoint.sh "$@"
fi 

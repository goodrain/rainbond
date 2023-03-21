#!/bin/bash
if [ "$1" = "bash" ];then
    exec /bin/bash
elif [ "${1}" = 'version' ];then
    echo "kubectl-$(kubectl version --client -o json | jq .clientVersion.gitVersion | sed s/\"//g)-grctl-\"$(grctl version)\""
else
    exec /usr/local/bin/docker-entrypoint.sh "$@"
fi

#!/bin/sh

if [ -z $NODE_TYPE ];then
    eval $(ssh-agent) > /dev/null
    eval $(ssh-add) > /dev/null
    #eval $(ssh-add /path/key) > /dev/null
    ACP_NODE_OPTS='--log-level=debug  --kube-conf=/etc/goodrain/kubernetes/admin.kubeconfig --run-mode=master'
else
    ACP_NODE_OPTS='--log-level=debug'
fi

exec /usr/local/bin/acp-node $ACP_NODE_OPTS
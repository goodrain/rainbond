#!/bin/sh

ETCD_ADDR=$(cat /etc/goodrain/envs/etcd.sh | awk -F '=' '{print $2}')
HOSTIP=$(cat /etc/goodrain/envs/ip.sh | awk -F '=' '{print $2}')

if [ -z $NODE_TYPE ];then
    eval $(ssh-agent) > /dev/null
    eval $(ssh-add) > /dev/null
    #eval $(ssh-add /path/key) > /dev/null
    ACP_NODE_OPTS="--static-task-path=/usr/share/gr-rainbond-node/gaops/tasks/ --etcd=http://$ETCD_ADDR:2379 --hostIP=$HOSTIP --kube-conf=/etc/goodrain/kubernetes/kubeconfig  --run-mode master --noderule manage,compute"
else
    ACP_NODE_OPTS='--log-level=debug'
fi

exec /usr/local/bin/rainbond-node $ACP_NODE_OPTS
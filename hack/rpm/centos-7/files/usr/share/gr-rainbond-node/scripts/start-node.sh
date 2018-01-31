#!/bin/sh

HOSTIP=$(cat /etc/goodrain/envs/ip.sh | awk -F '=' '{print $2}')

if [ -z $NODE_TYPE ];then
    eval $(ssh-agent) > /dev/null
    eval $(ssh-add) > /dev/null
    ETCD_ADDR=$(cat /etc/goodrain/envs/etcd.sh | awk -F '=' '{print $2}')
    NODE_OPTS="--log-level=debug --statsd.mapping-config=/usr/share/gr-rainbond-node/gaops/config/mapper.yml --static-task-path=/usr/share/gr-rainbond-node/gaops/tasks/ --etcd=http://$ETCD_ADDR:2379  --kube-conf=/etc/goodrain/kubernetes/kubeconfig --hostIP=$HOSTIP --run-mode master --noderule ${ROLE:-manage}"
else

    NODE_OPTS="--log-level=debug --statsd.mapping-config=/usr/share/gr-rainbond-node/gaops/config/mapper.yml --static-task-path=/usr/share/gr-rainbond-node/gaops/tasks/ --etcd=http://127.0.0.1:2379 --kube-conf=/etc/goodrain/kubernetes/kubeconfig --hostIP=$HOSTIP"
fi

exec /usr/local/bin/rainbond-node $NODE_OPTS
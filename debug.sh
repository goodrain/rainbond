#! /bin/sh

export GOPROXY=https://goproxy.cn
go mod vendor
dlv --headless --log --listen :9009 --api-version 2 --accept-multiclient debug ./cmd/builder/builder.go -- --rbd-repo=rbd-resource-proxy --rbd-namespace=rbd-system --pvc-cache-name=rbd-chaos-cache --pvc-grdata-name=rbd-cpt-grdata --etcd-endpoints=http://rbd-etcd:2379 --hostIP=$(POD_IP) --mysql=root:31af076e@tcp(rbd-db-rw:3306)/region
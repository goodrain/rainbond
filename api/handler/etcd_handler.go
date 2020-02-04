package handler

import (
	"context"
	"github.com/coreos/etcd/clientv3"
)

// EtcdHandler defines handler methods about k8s pods.
type EtcdHandler struct {
	etcdCli *clientv3.Client
}

// NewEtcdHandler creates a new PodHandler.
func NewEtcdHandler(etcdCli *clientv3.Client) *EtcdHandler {
	return &EtcdHandler{etcdCli}
}

func (h *EtcdHandler) CleanEtcd(keys []string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, key := range keys {
		h.etcdCli.Delete(ctx, key) // TODO 同时删除多个key
	}

}

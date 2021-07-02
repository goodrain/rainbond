package handler

import (
	"context"
	"fmt"
	"github.com/coreos/etcd/clientv3"
	"github.com/sirupsen/logrus"
)

// EtcdKeyType etcd key type
type EtcdKeyType int

const (
	// ServiceCheckEtcdKey source check etcd key
	ServiceCheckEtcdKey EtcdKeyType = iota
	// ShareResultEtcdKey share result etcd key
	ShareResultEtcdKey
	//BackupRestoreEtcdKey backup restore etcd key
	BackupRestoreEtcdKey
)

// EtcdHandler defines handler methods about k8s pods.
type EtcdHandler struct {
	etcdCli *clientv3.Client
}

// NewEtcdHandler creates a new PodHandler.
func NewEtcdHandler(etcdCli *clientv3.Client) *EtcdHandler {
	return &EtcdHandler{etcdCli}
}

// CleanAllServiceData -
func (h *EtcdHandler) CleanAllServiceData(keys []string) {
	for _, key := range keys {
		h.cleanEtcdByKey(key, ServiceCheckEtcdKey, ShareResultEtcdKey, BackupRestoreEtcdKey)
	}
}

// CleanServiceCheckData clean service check etcd data
func (h *EtcdHandler) CleanServiceCheckData(key string) {
	h.cleanEtcdByKey(key, ServiceCheckEtcdKey)
}

func (h *EtcdHandler) cleanEtcdByKey(key string, keyTypes ...EtcdKeyType) {
	if key == "" {
		logrus.Warn("get empty etcd data key, ignore it")
	}
	for _, keyType := range keyTypes {
		prefix := ""
		switch keyType {
		case ServiceCheckEtcdKey:
			prefix = fmt.Sprintf("/servicecheck/%s", key)
		case ShareResultEtcdKey:
			prefix = fmt.Sprintf("/rainbond/shareresult/%s", key)
		case BackupRestoreEtcdKey:
			prefix = fmt.Sprintf("/rainbond/backup_restore/%s", key)
		}
		h.cleanEtcdData(prefix)
	}

}

func (h *EtcdHandler) cleanEtcdData(prefix string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logrus.Debugf("ready for delete etcd key:%s", prefix)
	_, err := h.etcdCli.Delete(ctx, prefix)
	if err != nil {
		logrus.Warnf("delete etcd key[%s] failed: %s", prefix, err.Error())
	}
}

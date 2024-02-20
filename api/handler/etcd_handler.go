package handler

import (
	"fmt"
	"github.com/goodrain/rainbond/db"
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
}

// NewEtcdHandler creates a new PodHandler.
func NewEtcdHandler() *EtcdHandler {
	return &EtcdHandler{}
}

// CleanAllServiceData -
func (h *EtcdHandler) CleanAllServiceData(keys []string) {
	for _, key := range keys {
		h.cleanDateBaseByKey(key, ServiceCheckEtcdKey, ShareResultEtcdKey, BackupRestoreEtcdKey)
	}
}

// CleanServiceCheckData clean service check etcd data
func (h *EtcdHandler) CleanServiceCheckData(key string) {
	h.cleanDateBaseByKey(key, ServiceCheckEtcdKey)
}

func (h *EtcdHandler) cleanDateBaseByKey(key string, keyTypes ...EtcdKeyType) {
	if key == "" {
		logrus.Warn("get empty etcd data key, ignore it")
		return
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
		h.cleanDateBaseData(prefix)
	}

}

func (h *EtcdHandler) cleanDateBaseData(prefix string) {
	err := db.GetManager().KeyValueDao().DeleteWithPrefix(prefix)
	if err != nil {
		logrus.Warnf("delete db key[%s] failed: %s", prefix, err.Error())
	}
}

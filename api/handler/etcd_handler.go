package handler

import (
	"encoding/json"
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

// CleanDateBaseHandler -
type CleanDateBaseHandler struct {
}

// NewCleanDateBaseHandler -
func NewCleanDateBaseHandler() *CleanDateBaseHandler {
	return &CleanDateBaseHandler{}
}

// CleanAllServiceData -
func (h *CleanDateBaseHandler) CleanAllServiceData(keys []string) {
	for _, key := range keys {
		h.cleanDateBaseByKey(key, ServiceCheckEtcdKey, ShareResultEtcdKey, BackupRestoreEtcdKey)
	}
}

// CleanServiceCheckData clean service check etcd data
func (h *CleanDateBaseHandler) CleanServiceCheckData(key string) {
	if key == "" {
		return
	}
	receiptKey := "/servicecheck/" + key
	receipt, err := db.GetManager().KeyValueDao().Get(receiptKey)
	if err != nil || receipt == nil || !serviceCheckCanBeReleasedOnCreate(receipt.V) {
		return
	}
	// Delete the exact check only. Import receipts require an explicit handoff
	// after all consuming components have committed, not the first creation.
	if err := db.GetManager().KeyValueDao().Delete(receiptKey); err != nil {
		logrus.Warn("could not release completed service check")
	}
}

func (h *CleanDateBaseHandler) cleanDateBaseByKey(key string, keyTypes ...EtcdKeyType) {
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

func (h *CleanDateBaseHandler) cleanDateBaseData(prefix string) {
	err := db.GetManager().KeyValueDao().DeleteWithPrefix(prefix)
	if err != nil {
		logrus.Warnf("delete db key[%s] failed: %s", prefix, err.Error())
	}
}

func serviceCheckCanBeReleasedOnCreate(value string) bool {
	var receipt struct {
		Status   string                       `json:"check_status"`
		Services []map[string]json.RawMessage `json:"service_info"`
	}
	if json.Unmarshal([]byte(value), &receipt) != nil ||
		(receipt.Status != "Success" && receipt.Status != "Failure") || receipt.Services == nil {
		return false
	}
	for _, service := range receipt.Services {
		if service == nil {
			return false
		}
		if _, imported := service["tar_images"]; imported {
			return false
		}
	}
	return true
}

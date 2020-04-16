package sync

import (
	"github.com/Sirupsen/logrus"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
)

// VolumeTypeEvent -
type VolumeTypeEvent struct {
	vtEventCh chan *model.TenantServiceVolumeType
	stopCh    chan struct{}
}

// New -
func New(stopCh chan struct{}) *VolumeTypeEvent {
	return &VolumeTypeEvent{
		stopCh:    stopCh,
		vtEventCh: make(chan *model.TenantServiceVolumeType, 100),
	}
}

// GetChan -
func (e *VolumeTypeEvent) GetChan() chan<- *model.TenantServiceVolumeType {
	return e.vtEventCh
}

// Handle -
func (e *VolumeTypeEvent) Handle() {
	for {
		select {
		case <-e.stopCh:
			return
		case vt := <-e.vtEventCh:
			createOrUpdate(vt)
		}
	}
}

func createOrUpdate(vt *model.TenantServiceVolumeType) {
	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()

	if _, err := db.GetManager().VolumeTypeDao().CreateOrUpdateVolumeType(vt); err != nil {
		logrus.Errorf("sync storageclass error : %s, ignore it", err.Error())
		tx.Rollback()
	}
	if err := tx.Commit().Error; err != nil {
		logrus.Errorf("commit sync storage class error: %s", err.Error())
	}
}

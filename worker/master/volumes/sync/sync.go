package sync

import (
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/sirupsen/logrus"
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
			if vt.VolumeType == "local-path" {
				vt.NameShow = "本地存储"
			}
			if _, err := db.GetManager().VolumeTypeDao().CreateOrUpdateVolumeType(vt); err != nil {
				logrus.Errorf("sync storageClass error : %s, ignore it", err.Error())
			}
		}
	}
}

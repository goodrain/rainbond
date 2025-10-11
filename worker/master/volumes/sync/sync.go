package sync

import (
	"context"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	workerutil "github.com/goodrain/rainbond/worker/util"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// Init - initialize and sync all existing StorageClasses
func (e *VolumeTypeEvent) Init(k8sClient kubernetes.Interface) error {
	logrus.Info("start to sync all existing StorageClasses to database")

	scList, err := k8sClient.StorageV1().StorageClasses().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("failed to list StorageClasses: %v", err)
		return err
	}

	logrus.Infof("found %d StorageClasses in cluster", len(scList.Items))

	for _, sc := range scList.Items {
		vt := workerutil.TransStorageClass2RBDVolumeType(&sc)
		if vt.VolumeType == "local-path" {
			vt.NameShow = "本地存储"
		}

		logrus.Infof("syncing StorageClass: %s (provisioner: %s)", sc.Name, sc.Provisioner)

		if _, err := db.GetManager().VolumeTypeDao().CreateOrUpdateVolumeType(vt); err != nil {
			logrus.Errorf("failed to sync StorageClass %s: %v", sc.Name, err)
			continue
		}

		logrus.Infof("successfully synced StorageClass: %s", sc.Name)
	}

	logrus.Info("finished syncing all StorageClasses")
	return nil
}

// Handle -
func (e *VolumeTypeEvent) Handle() {
	for {
		select {
		case <-e.stopCh:
			logrus.Info("VolumeTypeEvent handler stopped")
			return
		case vt := <-e.vtEventCh:
			logrus.Infof("received StorageClass event: %s (provisioner: %s)", vt.VolumeType, vt.Provisioner)

			if vt.VolumeType == "local-path" {
				vt.NameShow = "本地存储"
			}

			if _, err := db.GetManager().VolumeTypeDao().CreateOrUpdateVolumeType(vt); err != nil {
				logrus.Errorf("failed to sync StorageClass %s: %v, ignore it", vt.VolumeType, err)
			} else {
				logrus.Infof("successfully synced StorageClass event: %s", vt.VolumeType)
			}
		}
	}
}

package model

import (
	"time"

	datav1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupScheduleInput 用于更新备份策略
type BackupScheduleInput struct {
	RBDService
	ClusterBackup
}

// BackupInput 用于创建备份
type BackupInput struct {
	RBDService
}

// BackupListQuery 用于获取备份列表
type BackupListQuery struct {
	RBDService
	Pagination
}

// BackupItem 用户备份
type BackupItem struct {
	Name   string                   `json:"name"`
	Status datav1alpha1.BackupPhase `json:"status"`
	Time   time.Time                `json:"time"`
}

type BackupRepo struct {
	Name                      string                       `json:"name"`
	Type                      string                       `json:"type"`
	AccessMethod              datav1alpha1.AccessMethod    `json:"accessMethod"`
	Phase                     datav1alpha1.BackupRepoPhase `json:"phase"`
	GeneratedStorageClassName string                       `json:"generatedStorageClassName,omitempty"`
	BackupPVCName             string                       `json:"backupPVCName,omitempty"`
	Conditions                []metav1.Condition           `json:"conditions,omitempty"`
}

type BackupRepoInput struct {
	Name            string                               `json:"name"`
	StorageProvider string                               `json:"storageProviderRef"`
	AccessMethod    datav1alpha1.AccessMethod            `json:"accessMethod"`
	PVReclaimPolicy corev1.PersistentVolumeReclaimPolicy `json:"pvReclaimPolicy"`
	VolumeCapacity  string                               `json:"volumeCapacity"`
	Config          map[string]string                    `json:"config"`
	Credential      corev1.SecretReference               `json:"credential"`
	Secrets         map[string]string                    `json:"secrets,omitempty"`
	PathPrefix      string                               `json:"pathPrefix"`
}

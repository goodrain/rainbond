package model

import (
	"time"

	datav1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

// BackupScheduleInput 用于更新备份设置
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
	Name         string                       `json:"name"`
	Type         string                       `json:"type"`
	AccessMethod datav1alpha1.AccessMethod    `json:"accessMethod"`
	Phase        datav1alpha1.BackupRepoPhase `json:"phase"`
}

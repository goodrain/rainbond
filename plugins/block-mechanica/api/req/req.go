// Package req 用于处理请求
package req

import (
	"github.com/furutachiKurea/block-mechanica/internal/model"
)

// ClusterRequest request body
type ClusterRequest struct {
	model.ClusterInfo
	model.ClusterResource
	model.ClusterBackup
	RBDService model.RBDService `json:"rbdService"`
}

type BackupRequest struct {
	RBDService model.RBDService `json:"rbdService"`
	model.ClusterBackup
}

type DeleteClustersRequest struct {
	ServiceIDs []string `json:"serviceIDs"`
}

type DeleteBackupsRequest struct {
	model.RBDService
	Backups []string `json:"backups"`
}

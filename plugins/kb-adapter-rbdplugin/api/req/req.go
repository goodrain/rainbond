// Package req 用于处理请求
package req

import (
	"strings"

	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/furutachiKurea/block-mechanica/internal/model"
)

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

type ManageClusterLifecycleRequest struct {
	Operation  string   `json:"operation"`
	ServiceIDs []string `json:"service_ids"`
}

type GetClusterEventsRequest struct {
	model.RBDService
	model.Pagination
}

type RestoreFromBackupRequest struct {
	model.RBDService
	NewServiceID string `json:"new_service_id"`
	BackupName   string `json:"backup_name"`
}

// ManageClusterType 将 ManageClusterLifecycleRequest.Operation 转换为 OpsType
func (m *ManageClusterLifecycleRequest) ManageClusterType() opsv1alpha1.OpsType {
	switch strings.TrimSpace(strings.ToLower(m.Operation)) {
	case "start":
		return opsv1alpha1.StartType
	case "stop":
		return opsv1alpha1.StopType
	case "restart":
		return opsv1alpha1.RestartType
	default:
		return opsv1alpha1.OpsType(m.Operation)
	}
}

type GetPodDetailRequest struct {
	ServiceID string `json:"service_id" param:"service-id"`
	PodName   string `json:"pod_name" param:"pod-name"`
}

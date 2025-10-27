package cluster

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/furutachiKurea/block-mechanica/internal/log"
	"github.com/furutachiKurea/block-mechanica/internal/model"
	"github.com/furutachiKurea/block-mechanica/internal/mono"
	"github.com/furutachiKurea/block-mechanica/service/kbkit"
	"github.com/furutachiKurea/block-mechanica/service/registry"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetConnectInfo 获取指定 Cluster 的连接账户信息,
// 从 Kubernetes Secret 中获取数据库账户的用户名和密码
//
// Secret 名称由对应数据库类型的 Coordinator 适配器生成
func (s *Service) GetConnectInfo(ctx context.Context, rbd model.RBDService) ([]model.ConnectInfo, error) {
	cluster, err := kbkit.GetClusterByServiceID(ctx, s.client, rbd.ServiceID)
	if err != nil {
		return nil, fmt.Errorf("get cluster by service_id %s: %w", rbd.ServiceID, err)
	}

	dbType := kbkit.ClusterType(cluster)
	clusterAdapter, ok := registry.Cluster[dbType]
	if !ok {
		return nil, fmt.Errorf("unsupported cluster type: %s", dbType)
	}
	secretName := clusterAdapter.Coordinator.GetSecretName(cluster.Name)

	secret := &corev1.Secret{}
	timeoutCtx, cancel := context.WithTimeout(ctx, 80*time.Second)
	defer cancel()
	err = wait.PollUntilContextCancel(timeoutCtx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		err := s.client.Get(ctx, client.ObjectKey{
			Name:      secretName,
			Namespace: cluster.Namespace,
		}, secret)

		if err != nil {
			log.Debug("Failed to get secret or not exist, skipping",
				log.String("cluster", cluster.Name),
				log.Err(err),
			)
			return false, nil
		}

		if _, exists := secret.Data["username"]; !exists {
			return false, nil
		}

		if _, exists := secret.Data["password"]; !exists {
			return false, nil
		}

		log.Debug("Secret exists and contains necessary fields",
			log.String("secret_name", secretName),
			log.String("namespace", cluster.Namespace),
		)

		return true, nil
	})

	if err != nil {
		return nil, fmt.Errorf("wait for secret %s/%s to be ready: %w", cluster.Namespace, secretName, err)
	}

	user, err := mono.GetSecretField(secret, "username")
	if err != nil {
		return nil, fmt.Errorf("get username: %w", err)
	}

	pwd, err := mono.GetSecretField(secret, "password")
	if err != nil {
		return nil, fmt.Errorf("get password: %w", err)
	}

	connectInfo := model.ConnectInfo{
		User:     user,
		Password: pwd,
	}

	log.Debug("get connect info",
		log.Any("connect_info", connectInfo),
	)

	return []model.ConnectInfo{connectInfo}, nil
}

// GetClusterDetail 通过 ServiceIdentifier.ID 获取 Cluster 的详细信息
func (s *Service) GetClusterDetail(ctx context.Context, rbd model.RBDService) (*model.ClusterDetail, error) {
	cluster, err := kbkit.GetClusterByServiceID(ctx, s.client, rbd.ServiceID)
	if err != nil {
		return nil, err
	}

	podList, err := s.getClusterPods(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("get cluster pods: %w", err)
	}

	component := cluster.Spec.ComponentSpecs[0]
	resourceInfo := s.extractResourceInfo(component)
	basicInfo := s.buildBasicInfo(cluster, component, rbd, podList)

	detail := &model.ClusterDetail{
		Basic:    basicInfo,
		Resource: resourceInfo,
	}

	if cluster.Spec.Backup == nil || cluster.Spec.Backup.Enabled == nil || !*cluster.Spec.Backup.Enabled {
		log.Debug("get cluster detail",
			log.Any("detail", detail),
		)
		return detail, nil
	}

	backupInfo, err := s.buildBackupInfo(cluster.Spec.Backup)
	if err != nil {
		return nil, fmt.Errorf("build backup info: %w", err)
	}
	detail.Backup = *backupInfo

	log.Debug("get cluster detail",
		log.Any("detail", detail),
	)

	return detail, nil
}

// extractResourceInfo 提取集群资源信息
func (s *Service) extractResourceInfo(component kbappsv1.ClusterComponentSpec) model.ClusterResourceStatus {
	cpuMilli := component.Resources.Limits.Cpu().MilliValue()
	memoryBytes := component.Resources.Limits.Memory().Value()
	memoryMiB := memoryBytes / MiB

	var storageGiB int64
	if len(component.VolumeClaimTemplates) > 0 {
		storageQty := component.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]
		storageGiB = storageQty.Value() / GiB
	}

	return model.ClusterResourceStatus{
		CPUMilli:  cpuMilli,
		MemoryMi:  memoryMiB,
		StorageGi: storageGiB,
		Replicas:  component.Replicas,
	}
}

func (s *Service) buildBasicInfo(
	cluster *kbappsv1.Cluster,
	component kbappsv1.ClusterComponentSpec,
	rbdService model.RBDService,
	podList []model.Status,
) model.BasicInfo {
	startTime := getStartTimeISO(cluster.Status.Conditions)
	status := strings.ToLower(string(cluster.Status.Phase))

	var storageClass string
	if len(component.VolumeClaimTemplates) > 0 &&
		component.VolumeClaimTemplates[0].Spec.StorageClassName != nil {
		storageClass = *component.VolumeClaimTemplates[0].Spec.StorageClassName
	}

	return model.BasicInfo{
		ClusterInfo: model.ClusterInfo{
			Name:              cluster.Name,
			Namespace:         cluster.Namespace,
			Type:              cluster.Spec.ClusterDef,
			Version:           component.ServiceVersion,
			StorageClass:      storageClass,
			TerminationPolicy: cluster.Spec.TerminationPolicy,
		},
		RBDService: model.RBDService{ServiceID: rbdService.ServiceID},
		Status: model.ClusterStatus{
			Status:    status,
			StatusCN:  transStatus(status),
			StartTime: startTime,
		},
		Replicas:           podList,
		IsSupportBackup:    kbkit.IsSupportBackup(kbkit.ClusterType(cluster)),
		IsSupportParameter: kbkit.IsSupportParameter(kbkit.ClusterType(cluster)),
	}
}

func (s *Service) buildBackupInfo(backup *kbappsv1.ClusterBackup) (*model.BackupInfo, error) {
	backupSchedule := &model.BackupSchedule{}
	if err := backupSchedule.Uncron(backup.CronExpression); err != nil {
		return nil, fmt.Errorf("parse backup schedule, cron: %s, err: %w", backup.CronExpression, err)
	}

	return &model.BackupInfo{
		ClusterBackup: model.ClusterBackup{
			BackupRepo:      backup.RepoName,
			Schedule:        *backupSchedule,
			RetentionPeriod: backup.RetentionPeriod,
		},
	}, nil
}

// getStartTimeISO 提取 Cluster 最近一次达到 Ready 且 Status 为 True 的时间点（metav1.Time）
func getStartTimeISO(conditions []metav1.Condition) string {
	var last *metav1.Time
	for _, cond := range conditions {
		if cond.Type == "Ready" && cond.Status == "True" {
			if last == nil || cond.LastTransitionTime.After(last.Time) {
				t := cond.LastTransitionTime
				last = &t
			}
		}
	}
	if last == nil {
		return ""
	}
	return formatToISO8601Time(last.Time)
}

func transStatus(status string) string {
	switch status {
	case "creating":
		return "创建中"
	case "running":
		return "运行中"
	case "updating":
		return "更新中"
	case "stopping":
		return "停止中"
	case "stopped":
		return "已停止"
	case "deleting":
		return "删除中"
	case "failed":
		return "失败"
	case "abnormal":
		return "异常"
	default:
		return status
	}
}

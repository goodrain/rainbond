package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/index"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/log"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/model"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/mono"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/service/kbkit"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/service/registry"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	datav1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ReasonBackupRunning = "备份正在执行中，无法删除"
)

// Service 提供 BackupRepo 相关操作
// 依赖 controller-runtime client
type Service struct {
	client client.Client
}

func NewService(c client.Client) *Service {
	return &Service{
		client: c,
	}
}

// ListAvailableBackupRepos 返回所有 Available 的 BackupRepo
func (s *Service) ListAvailableBackupRepos(ctx context.Context) ([]*model.BackupRepo, error) {
	repos, err := s.listBackupRepos(ctx)
	if err != nil {
		return nil, err
	}

	available := mono.Filter(repos, func(r *model.BackupRepo) bool {
		return r.Phase == datav1alpha1.BackupRepoReady
	})

	return available, nil
}

// ReScheduleBackup 重新调度 Cluster 的备份配置
//
// 通过 Patch cluster 中的备份字段来实现 back schedule 的更新
func (s *Service) ReScheduleBackup(ctx context.Context, schedule model.BackupScheduleInput) error {
	cluster, err := kbkit.GetClusterByServiceID(ctx, s.client, schedule.ServiceID)
	if err != nil {
		return fmt.Errorf("get cluster by service_id: %w", err)
	}

	// Determine backup method based on cluster type (required by KubeBlocks)
	adapter := registry.Cluster[kbkit.ClusterType(cluster)]
	backupMethod := adapter.Coordinator.GetBackupMethod()

	needUpdate := false
	var patchData map[string]any
	if schedule.BackupRepo == "" {
		if cluster.Spec.Backup != nil {
			patchData = map[string]any{
				"spec": map[string]any{
					"backup": map[string]any{
						"enabled": false,
					},
				},
			}
			needUpdate = true
		}
	} else {
		if cluster.Spec.Backup == nil {
			enabled := true
			patchData = map[string]any{
				"spec": map[string]any{
					"backup": map[string]any{
						"repoName":        schedule.BackupRepo,
						"enabled":         &enabled,
						"method":          backupMethod,
						"cronExpression":  schedule.Schedule.Cron(),
						"retentionPeriod": schedule.RetentionPeriod,
					},
				},
			}
			needUpdate = true
		} else {
			backupPatch := make(map[string]any)

			if cluster.Spec.Backup.RepoName != schedule.BackupRepo {
				backupPatch["repoName"] = schedule.BackupRepo
			}

			if cluster.Spec.Backup.CronExpression != schedule.Schedule.Cron() {
				backupPatch["cronExpression"] = schedule.Schedule.Cron()
			}

			if cluster.Spec.Backup.RetentionPeriod != schedule.RetentionPeriod {
				backupPatch["retentionPeriod"] = schedule.RetentionPeriod
			}

			if cluster.Spec.Backup.Enabled == nil || !*cluster.Spec.Backup.Enabled {
				enabled := true
				backupPatch["enabled"] = &enabled
			}

			// Always ensure method is present to satisfy validation
			backupPatch["method"] = backupMethod

			if len(backupPatch) > 0 {
				patchData = map[string]any{
					"spec": map[string]any{
						"backup": backupPatch,
					},
				}
				needUpdate = true
			}
		}
	}

	if !needUpdate {
		return nil
	}

	patchBytes, err := json.Marshal(patchData)
	if err != nil {
		return fmt.Errorf("marshal patch data: %w", err)
	}

	// Patch backup 配置
	if err := s.client.Patch(ctx, cluster, client.RawPatch(types.MergePatchType, patchBytes)); err != nil {
		return fmt.Errorf("patch cluster backup configuration: %w", err)
	}

	return nil
}

// BackupCluster 执行集群备份操作
//
// 参考：https://kubeblocks.io/docs/preview/kubeblocks-for-mysql/05-backup-restore/02-create-full-backup
func (s *Service) BackupCluster(ctx context.Context, req model.BackupInput) error {
	log.Debug("Starting backup operation",
		log.String("service_id", req.ServiceID),
	)

	cluster, err := kbkit.GetClusterByServiceID(ctx, s.client, req.ServiceID)
	if err != nil {
		return fmt.Errorf("get cluster by service_id: %w", err)
	}

	if cluster.Status.Phase == kbappsv1.StoppedClusterPhase || cluster.Status.Phase == kbappsv1.StoppingClusterPhase {
		return fmt.Errorf("cluster %s/%s is not running", cluster.Namespace, cluster.Name)
	}

	adapter := registry.Cluster[kbkit.ClusterType(cluster)]

	if cluster.Spec.Backup == nil || !*cluster.Spec.Backup.Enabled {
		return fmt.Errorf("backup is not enabled for cluster %s", cluster.Name)
	}

	backupMethod := adapter.Coordinator.GetBackupMethod()

	if err := kbkit.CreateBackupOpsRequest(ctx, s.client, cluster, backupMethod); err != nil {
		return fmt.Errorf("create backup opsrequest: %w", err)
	}

	log.Info("Created backup OpsRequest",
		log.String("cluster", cluster.Name),
		log.String("backup_method", backupMethod))

	return nil
}

// ListBackups 返回给定的 Cluster 的备份列表
func (s *Service) ListBackups(ctx context.Context, query model.BackupListQuery) (*model.PaginatedResult[model.BackupItem], error) {
	query.Validate()

	cluster, err := kbkit.GetClusterByServiceID(ctx, s.client, query.ServiceID)
	if err != nil {
		return nil, err
	}

	backups, err := getBackupsByIndex(ctx, s.client, cluster.Name, cluster.Namespace)
	if err != nil {
		return nil, err
	}

	sortBackupsByTime(backups)

	backupList := make([]model.BackupItem, 0, len(backups))
	for _, backup := range backups {
		backupTime := backup.CreationTimestamp.UTC()
		if backup.Status.StartTimestamp != nil {
			backupTime = backup.Status.StartTimestamp.UTC()
		}

		backupPhase := datav1alpha1.BackupPhaseNew
		if backup.Status.Phase != "" {
			backupPhase = backup.Status.Phase
		}

		backupList = append(backupList, model.BackupItem{
			Name:   backup.Name,
			Status: backupPhase,
			Time:   backupTime,
		})
	}

	result := kbkit.Paginate(backupList, query.Page, query.PageSize)

	log.Debug("paginated backup list",
		log.String("cluster", cluster.Name),
		log.Any("backupList", backupList),
	)

	return &model.PaginatedResult[model.BackupItem]{
		Items: result,
		Total: len(backupList),
	}, nil
}

// listBackupRepos 返回所有命名空间下的 BackupRepo 信息
func (s *Service) listBackupRepos(ctx context.Context) ([]*model.BackupRepo, error) {
	var repoList datav1alpha1.BackupRepoList
	if err := s.client.List(ctx, &repoList); err != nil {
		return nil, fmt.Errorf("list BackupRepo: %w", err)
	}
	result := make([]*model.BackupRepo, 0, len(repoList.Items))
	for _, item := range repoList.Items {
		result = append(result, &model.BackupRepo{
			Name:         item.Name,
			Type:         item.Spec.StorageProviderRef,
			AccessMethod: item.Spec.AccessMethod,
			Phase:        item.Status.Phase,
		})
	}
	return result, nil
}

// getBackupsByIndex 使用索引查询 Backup，失败时回退到标签查询
func getBackupsByIndex(ctx context.Context, c client.Client, clusterName, namespace string) ([]datav1alpha1.Backup, error) {
	var backupList datav1alpha1.BackupList

	indexKey := fmt.Sprintf("%s/%s", namespace, clusterName)
	if err := c.List(ctx, &backupList, client.MatchingFields{index.NamespaceInstanceField: indexKey}); err == nil {
		return backupList.Items, nil
	}

	selector := client.MatchingLabels{constant.AppInstanceLabelKey: clusterName}
	if err := c.List(ctx, &backupList, selector, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list backups for cluster %s in namespace %s: %w", clusterName, namespace, err)
	}

	return backupList.Items, nil
}

// DeleteBackups 批量删除指定备份
//
// 根据 service_id 查找对应的 Cluster，然后删除请求中指定名称的备份
// 返回成功删除的备份名称列表
func (s *Service) DeleteBackups(ctx context.Context, rbd model.RBDService, backupNames []string) ([]string, error) {
	cluster, err := kbkit.GetClusterByServiceID(ctx, s.client, rbd.ServiceID)
	if err != nil {
		return nil, fmt.Errorf("get cluster by service_id %s: %w", rbd.ServiceID, err)
	}

	backups, err := getBackupsByIndex(ctx, s.client, cluster.Name, cluster.Namespace)
	if err != nil {
		return nil, fmt.Errorf("get backups for cluster %s: %w", cluster.Name, err)
	}

	backupMap := make(map[string]*datav1alpha1.Backup)
	for i := range backups {
		backupMap[backups[i].Name] = &backups[i]
	}

	var deleted []string

	for _, name := range backupNames {
		backup, exists := backupMap[name]
		if !exists {
			continue
		}

		if canDelete, reason := s.canDeleteBackup(backup); !canDelete {
			log.Info("备份无法删除", log.String("backup", name), log.String("reason", reason))
			continue
		}

		if err := s.client.Delete(ctx, backup); err != nil {
			if apierrors.IsNotFound(err) {
				deleted = append(deleted, name)
				continue
			}

			log.Error("删除备份失败", log.String("backup", name), log.String("cluster", cluster.Name), log.Err(err))
			continue
		}

		deleted = append(deleted, name)
	}

	return deleted, nil
}

// canDeleteBackup 检查备份是否可以安全删除
func (s *Service) canDeleteBackup(backup *datav1alpha1.Backup) (bool, string) {
	if backup.Status.Phase == datav1alpha1.BackupPhaseRunning {
		return false, ReasonBackupRunning
	}

	return true, ""
}

// sortBackupsByTime 按时间倒序排列备份
func sortBackupsByTime(backups []datav1alpha1.Backup) {
	sort.Slice(backups, func(i, j int) bool {
		a, b := backups[i], backups[j]

		timeA := a.CreationTimestamp.UTC()
		if a.Status.StartTimestamp != nil {
			timeA = a.Status.StartTimestamp.UTC()
		}

		timeB := b.CreationTimestamp.UTC()
		if b.Status.StartTimestamp != nil {
			timeB = b.Status.StartTimestamp.UTC()
		}

		return timeA.After(timeB)
	})
}

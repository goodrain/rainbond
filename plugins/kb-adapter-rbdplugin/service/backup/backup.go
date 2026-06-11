package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/index"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/log"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/model"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/service/kbkit"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/service/registry"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	datav1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// ListAvailableBackupRepos 返回集群内所有 BackupRepo，并保留 Ready/Failed/PreChecking 等状态。
func (s *Service) ListAvailableBackupRepos(ctx context.Context) ([]*model.BackupRepo, error) {
	return s.listBackupRepos(ctx)
}

func (s *Service) CreateBackupRepo(ctx context.Context, input model.BackupRepoInput) (*model.BackupRepo, error) {
	if err := validateBackupRepoInput(input, true); err != nil {
		return nil, err
	}

	if err := s.upsertBackupRepoSecret(ctx, input); err != nil {
		return nil, err
	}

	repo, err := buildBackupRepo(input)
	if err != nil {
		return nil, err
	}
	if err := s.client.Create(ctx, repo); err != nil {
		return nil, fmt.Errorf("create BackupRepo %s: %w", input.Name, err)
	}
	return backupRepoToModel(repo), nil
}

func (s *Service) UpdateBackupRepo(ctx context.Context, name string, input model.BackupRepoInput) (*model.BackupRepo, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("backup repo name is required")
	}
	if input.Name == "" {
		input.Name = name
	}
	if input.Name != name {
		return nil, fmt.Errorf("backup repo name %s does not match path %s", input.Name, name)
	}
	if err := validateBackupRepoInput(input, false); err != nil {
		return nil, err
	}

	var repo datav1alpha1.BackupRepo
	if err := s.client.Get(ctx, types.NamespacedName{Name: name}, &repo); err != nil {
		return nil, fmt.Errorf("get BackupRepo %s: %w", name, err)
	}
	if input.StorageProvider != "" && input.StorageProvider != repo.Spec.StorageProviderRef {
		return nil, fmt.Errorf("storageProviderRef is immutable")
	}
	if err := s.upsertBackupRepoSecret(ctx, input); err != nil {
		return nil, err
	}

	updated, err := buildBackupRepo(input)
	if err != nil {
		return nil, err
	}
	repo.Spec.AccessMethod = updated.Spec.AccessMethod
	repo.Spec.VolumeCapacity = updated.Spec.VolumeCapacity
	repo.Spec.PVReclaimPolicy = updated.Spec.PVReclaimPolicy
	repo.Spec.Config = updated.Spec.Config
	repo.Spec.Credential = updated.Spec.Credential
	repo.Spec.PathPrefix = updated.Spec.PathPrefix
	if err := s.client.Update(ctx, &repo); err != nil {
		return nil, fmt.Errorf("update BackupRepo %s: %w", name, err)
	}
	return backupRepoToModel(&repo), nil
}

func (s *Service) DeleteBackupRepo(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("backup repo name is required")
	}
	if err := s.ensureBackupRepoNotInUse(ctx, name); err != nil {
		return err
	}

	var repo datav1alpha1.BackupRepo
	if err := s.client.Get(ctx, types.NamespacedName{Name: name}, &repo); err != nil {
		return fmt.Errorf("get BackupRepo %s: %w", name, err)
	}
	if err := s.client.Delete(ctx, &repo); err != nil {
		return fmt.Errorf("delete BackupRepo %s: %w", name, err)
	}
	if repo.Spec.Credential != nil && repo.Spec.Credential.Name != "" && repo.Spec.Credential.Namespace != "" {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      repo.Spec.Credential.Name,
				Namespace: repo.Spec.Credential.Namespace,
			},
		}
		if err := s.client.Delete(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete BackupRepo credential %s/%s: %w", secret.Namespace, secret.Name, err)
		}
	}
	return nil
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
					"backup": nil,
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
		result = append(result, backupRepoToModel(&item))
	}
	return result, nil
}

func validateBackupRepoInput(input model.BackupRepoInput, requireSecret bool) error {
	if strings.TrimSpace(input.Name) == "" {
		return fmt.Errorf("backup repo name is required")
	}
	if strings.TrimSpace(input.StorageProvider) == "" {
		return fmt.Errorf("storageProviderRef is required")
	}
	if input.Config == nil {
		return fmt.Errorf("config is required")
	}
	if strings.TrimSpace(input.Config["bucket"]) == "" {
		return fmt.Errorf("config.bucket is required")
	}
	if strings.TrimSpace(input.Config["endpoint"]) == "" {
		return fmt.Errorf("config.endpoint is required")
	}
	if input.Credential.Name == "" || input.Credential.Namespace == "" {
		return fmt.Errorf("credential.name and credential.namespace are required")
	}
	if requireSecret && (input.Secrets["accessKeyId"] == "" || input.Secrets["secretAccessKey"] == "") {
		return fmt.Errorf("accessKeyId and secretAccessKey are required")
	}
	return nil
}

func buildBackupRepo(input model.BackupRepoInput) (*datav1alpha1.BackupRepo, error) {
	accessMethod := input.AccessMethod
	if accessMethod == "" {
		accessMethod = datav1alpha1.AccessMethodTool
	}
	reclaimPolicy := input.PVReclaimPolicy
	if reclaimPolicy == "" {
		reclaimPolicy = corev1.PersistentVolumeReclaimRetain
	}
	capacityText := strings.TrimSpace(input.VolumeCapacity)
	if capacityText == "" {
		capacityText = "100Gi"
	}
	capacity, err := resource.ParseQuantity(capacityText)
	if err != nil {
		return nil, fmt.Errorf("parse volumeCapacity %s: %w", capacityText, err)
	}

	config := make(map[string]string, len(input.Config))
	for k, v := range input.Config {
		config[k] = v
	}

	return &datav1alpha1.BackupRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name: input.Name,
		},
		Spec: datav1alpha1.BackupRepoSpec{
			StorageProviderRef: input.StorageProvider,
			AccessMethod:       accessMethod,
			VolumeCapacity:     capacity,
			PVReclaimPolicy:    reclaimPolicy,
			Config:             config,
			Credential: &corev1.SecretReference{
				Name:      input.Credential.Name,
				Namespace: input.Credential.Namespace,
			},
			PathPrefix: input.PathPrefix,
		},
	}, nil
}

func (s *Service) upsertBackupRepoSecret(ctx context.Context, input model.BackupRepoInput) error {
	if len(input.Secrets) == 0 {
		return nil
	}

	key := types.NamespacedName{Name: input.Credential.Name, Namespace: input.Credential.Namespace}
	var secret corev1.Secret
	err := s.client.Get(ctx, key, &secret)
	if apierrors.IsNotFound(err) {
		secret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      input.Credential.Name,
				Namespace: input.Credential.Namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{},
		}
		for k, v := range input.Secrets {
			secret.Data[k] = []byte(v)
		}
		if err := s.client.Create(ctx, &secret); err != nil {
			return fmt.Errorf("create BackupRepo credential %s/%s: %w", key.Namespace, key.Name, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get BackupRepo credential %s/%s: %w", key.Namespace, key.Name, err)
	}
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	for k, v := range input.Secrets {
		secret.Data[k] = []byte(v)
	}
	if err := s.client.Update(ctx, &secret); err != nil {
		return fmt.Errorf("update BackupRepo credential %s/%s: %w", key.Namespace, key.Name, err)
	}
	return nil
}

func (s *Service) ensureBackupRepoNotInUse(ctx context.Context, name string) error {
	var clusterList kbappsv1.ClusterList
	if err := s.client.List(ctx, &clusterList); err != nil {
		return fmt.Errorf("list clusters for BackupRepo usage: %w", err)
	}
	for _, cluster := range clusterList.Items {
		if cluster.Spec.Backup != nil && cluster.Spec.Backup.RepoName == name {
			return fmt.Errorf("backup repo %s is in use by cluster %s/%s", name, cluster.Namespace, cluster.Name)
		}
	}
	return nil
}

func backupRepoToModel(item *datav1alpha1.BackupRepo) *model.BackupRepo {
	return &model.BackupRepo{
		Name:                      item.Name,
		Type:                      item.Spec.StorageProviderRef,
		AccessMethod:              item.Spec.AccessMethod,
		Phase:                     item.Status.Phase,
		GeneratedStorageClassName: item.Status.GeneratedStorageClassName,
		BackupPVCName:             item.Status.BackupPVCName,
		Conditions:                item.Status.Conditions,
	}
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

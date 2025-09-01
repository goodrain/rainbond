package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/furutachiKurea/block-mechanica/internal/index"
	"github.com/furutachiKurea/block-mechanica/internal/log"
	"github.com/furutachiKurea/block-mechanica/internal/model"
	"github.com/furutachiKurea/block-mechanica/internal/mono"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	MiB = 1024 * 1024
	GiB = 1024 * 1024 * 1024
)

// ClusterService 提供针对 Cluster 相关操作
type ClusterService struct {
	client client.Client
}

func NewClusterService(c client.Client) *ClusterService {
	return &ClusterService{
		client: c,
	}
}

// CreateCluster 依据 req 创建 KubeBlocks Cluster
//
// 通过将 service_id 添加至 Cluster 的 labels 中以关联 KubeBlocks Component 与 Cluster,
// 同时，Rainbond 也通过这层关系来判断 Rainbond 组件是否为 KubeBlocks Component
func (s *ClusterService) CreateCluster(ctx context.Context, c model.ClusterInput) error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}

	clusterAdapter, ok := _clusterRegistry[c.Type]
	if !ok {
		return fmt.Errorf("unsupported cluster type: %s", c.Type)
	}

	cluster, err := clusterAdapter.Builder.BuildCluster(ctx, c)
	if err != nil {
		return fmt.Errorf("build %s cluster: %w", c.Type, err)
	}

	if err := s.client.Create(ctx, cluster); err != nil {
		return fmt.Errorf("create cluster: %w", err)
	}

	c.Name = cluster.Name
	if err := s.associateToKubeBlocksComponent(ctx, cluster, c); err != nil {
		return fmt.Errorf("associate to rainbond component: %w", err)
	}

	return nil
}

// AssociateToKubeBlocksComponent 将 KubeBlocks 组件和 Cluster 通过 service_id 关联
func (s *ClusterService) associateToKubeBlocksComponent(ctx context.Context, cluster *kbappsv1.Cluster, input model.ClusterInput) error {
	log.Debug("start associate cluster to rainbond component",
		log.String("service_id", input.RBDService.ServiceID),
		log.String("cluster", cluster.Name),
	)

	const labelServiceID = index.ServiceIDLabel

	err := wait.PollUntilContextCancel(ctx, 500*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		var latest kbappsv1.Cluster
		if err := s.client.Get(ctx, client.ObjectKey{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
		}, &latest); err != nil {
			log.Debug("Cluster not found yet, waiting",
				log.String("cluster", cluster.Name),
				log.String("namespace", cluster.Namespace),
			)
			return false, nil
		}

		if latest.Labels != nil && latest.Labels[labelServiceID] == input.RBDService.ServiceID {
			log.Debug("Cluster already has correct service_id label",
				log.String("service_id", input.RBDService.ServiceID),
			)
			return true, nil
		}

		patchData := fmt.Sprintf(`{
			"metadata": {
				"labels": {
					"%s": "%s"
				}
			}
		}`, labelServiceID, input.RBDService.ServiceID)

		if err := s.client.Patch(ctx, &kbappsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cluster.Name,
				Namespace: cluster.Namespace,
			},
		}, client.RawPatch(types.MergePatchType, []byte(patchData))); err != nil {
			log.Debug("Patch operation failed, retrying",
				log.String("cluster", cluster.Name),
				log.Err(err),
			)
			return false, nil
		}

		log.Debug("Successfully added service_id label to cluster",
			log.String("service_id", input.RBDService.ServiceID),
			log.String("cluster", cluster.Name),
		)
		return true, nil
	})

	if err != nil {
		return fmt.Errorf("failed to associate cluster %s/%s with service_id label after retries: %w", cluster.Namespace, cluster.Name, err)
	}

	log.Info("Associated KubeBlocks Cluster to Rainbond component",
		log.String("service_id", input.RBDService.ServiceID),
		log.String("cluster", cluster.Name),
	)

	return nil
}

// GetConnectInfo 获取指定 Cluster 的连接账户信息,
// 从 Kubernetes Secret 中获取 root 账户的用户名和密码
//
// Secret 命名规则: {clustername}-{clustertype}-account-root
func (s *ClusterService) GetConnectInfo(ctx context.Context, rbd model.RBDService) ([]model.ConnectInfo, error) {
	cluster, err := getClusterByServiceID(ctx, s.client, rbd.ServiceID)
	if err != nil {
		return nil, fmt.Errorf("get cluster by service_id %s: %w", rbd.ServiceID, err)
	}

	secretName := fmt.Sprintf("%s-%s-account-root", cluster.Name, clusterType(cluster))

	secret := &corev1.Secret{}
	err = wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		err := s.client.Get(ctx, client.ObjectKey{
			Name:      secretName,
			Namespace: cluster.Namespace,
		}, secret)

		if err != nil {
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

	dbUSER, err := mono.GetSecretField(secret, "username")
	if err != nil {
		return nil, fmt.Errorf("get username: %w", err)
	}

	dbPASS, err := mono.GetSecretField(secret, "password")
	if err != nil {
		return nil, fmt.Errorf("get password: %w", err)
	}

	connectInfo := model.ConnectInfo{
		User:     dbUSER,
		Password: dbPASS,
	}

	log.Debug("get connect info",
		log.Any("connect_info", connectInfo),
	)

	return []model.ConnectInfo{connectInfo}, nil
}

// GetClusterDetail 通过 ServiceIdentifier.ID 获取 Cluster 的详细信息
func (s *ClusterService) GetClusterDetail(ctx context.Context, rbd model.RBDService) (*model.ClusterDetail, error) {
	cluster, err := getClusterByServiceID(ctx, s.client, rbd.ServiceID)
	if err != nil {
		return nil, err
	}

	podList, err := s.getClusterPods(ctx, cluster.Name, cluster.Namespace)
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

	if cluster.Spec.Backup == nil {
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
func (s *ClusterService) extractResourceInfo(component kbappsv1.ClusterComponentSpec) model.ClusterResourceStatus {
	cpuMilli := component.Resources.Limits.Cpu().MilliValue()
	memoryBytes := component.Resources.Limits.Memory().Value()
	memoryMiB := memoryBytes / MiB

	storageQty := component.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]
	storageGiB := storageQty.Value() / GiB

	return model.ClusterResourceStatus{
		CPUMilli:  cpuMilli,
		MemoryMi:  memoryMiB,
		StorageGi: storageGiB,
		Replicas:  component.Replicas,
	}
}

func (s *ClusterService) buildBasicInfo(
	cluster *kbappsv1.Cluster,
	component kbappsv1.ClusterComponentSpec,
	rbdService model.RBDService,
	podList []model.PodStatus,
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
		Replicas: podList,
	}
}

func (s *ClusterService) buildBackupInfo(backup *kbappsv1.ClusterBackup) (*model.BackupInfo, error) {
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

// ExpansionCluster 对 Cluster 进行伸缩操作
//
// 使用 opsrequest 将 Cluster 的资源规格进行伸缩，使其变为 model.ExpansionInput 的期望状态
func (s *ClusterService) ExpansionCluster(ctx context.Context, expansion model.ExpansionInput) error {
	log.Debug("Expansion",
		log.String("service_id", expansion.ServiceID),
		log.Any("expansion", expansion),
	)

	cluster, err := getClusterByServiceID(ctx, s.client, expansion.ServiceID)
	if err != nil {
		return err
	}
	if len(cluster.Spec.ComponentSpecs) == 0 {
		return fmt.Errorf("cluster %s/%s has no componentSpecs", cluster.Namespace, cluster.Name)
	}

	component := cluster.Spec.ComponentSpecs[0]
	componentName := component.Name
	if componentName == "" {
		componentName = cluster.Spec.ClusterDef
	}

	desiredCPU, err := resource.ParseQuantity(expansion.CPU)
	if err != nil {
		return fmt.Errorf("parse desired cpu %q: %w", expansion.CPU, err)
	}
	desiredMem, err := resource.ParseQuantity(expansion.Memory)
	if err != nil {
		return fmt.Errorf("parse desired memory %q: %w", expansion.Memory, err)
	}
	desiredStorage, err := resource.ParseQuantity(expansion.Storage)
	if err != nil {
		return fmt.Errorf("parse desired storage %q: %w", expansion.Storage, err)
	}

	currentCPU := component.Resources.Limits.Cpu()
	currentMem := component.Resources.Limits.Memory()
	var (
		hasPVC          = len(component.VolumeClaimTemplates) > 0
		volumeTplName   string
		currentStorage  resource.Quantity
		storageClassRef *string
	)
	if hasPVC {
		volumeTpl := component.VolumeClaimTemplates[0]
		volumeTplName = volumeTpl.Name
		if size, ok := volumeTpl.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			currentStorage = size
		}
		storageClassRef = volumeTpl.Spec.StorageClassName
	}

	var opsCreated bool

	expansionCtx := model.ExpansionContext{
		Cluster:       cluster,
		ComponentName: componentName,
		// 水平伸缩
		CurrentReplicas: component.Replicas,
		DesiredReplicas: expansion.Replicas,
		// 垂直伸缩
		CurrentCPU: *currentCPU,
		CurrentMem: *currentMem,
		DesiredCPU: desiredCPU,
		DesiredMem: desiredMem,
		// 存储扩容
		HasPVC:          hasPVC,
		VolumeTplName:   volumeTplName,
		CurrentStorage:  currentStorage,
		DesiredStorage:  desiredStorage,
		StorageClassRef: storageClassRef,
	}

	hCreated, err := s.handleHorizontalScaling(ctx, expansionCtx)
	if err != nil {
		return fmt.Errorf("horizontal scaling: %w", err)
	}
	opsCreated = opsCreated || hCreated

	if currentCPU != nil && currentMem != nil {
		vCreated, err := s.handleVerticalScaling(ctx, expansionCtx)
		if err != nil {
			return fmt.Errorf("vertical scaling: %w", err)
		}
		opsCreated = opsCreated || vCreated
	}

	sCreated, err := s.handleVolumeExpansion(ctx, expansionCtx)
	if err != nil {
		return fmt.Errorf("volume expansion: %w", err)
	}
	opsCreated = opsCreated || sCreated

	if !opsCreated {
		log.Info("No expansion needed, cluster already matches desired spec",
			log.String("cluster", cluster.Name),
			log.String("service_id", expansion.ServiceID))
	}

	return nil
}

func (s *ClusterService) StartCluster(ctx context.Context, cluster *kbappsv1.Cluster) error {
	return createLifecycleOpsRequest(ctx, s.client, cluster, opsv1alpha1.StartType)
}

func (s *ClusterService) StopCluster(ctx context.Context, cluster *kbappsv1.Cluster) error {
	return createLifecycleOpsRequest(ctx, s.client, cluster, opsv1alpha1.StopType)
}

// DeleteCluster 删除 KubeBlocks 数据库集群
//
// 批量删除指定 serviceIDs 对应的 Cluster，忽略找不到的 service_id
func (s *ClusterService) DeleteCluster(ctx context.Context, serviceIDs []string) error {
	for _, serviceID := range serviceIDs {
		err := s.deleteCluster(ctx, serviceID, false)
		if err != nil {
			if errors.Is(err, ErrTargetNotFound) {
				continue
			}
			return fmt.Errorf("delete cluster for service_id %s: %w", serviceID, err)
		}
	}
	return nil
}

// CancelClusterCreate 取消集群创建
//
// 在删除前将 TerminationPolicy 调整为 WipeOut，确保 PVC/PV 等存储资源一并清理，避免脏数据残留
func (s *ClusterService) CancelClusterCreate(ctx context.Context, rbd model.RBDService) error {
	return s.deleteCluster(ctx, rbd.ServiceID, true)
}

// deleteCluster 内部删除方法，提供是否将 TerminationPolicy 设置为 WipeOut 的选项
func (s *ClusterService) deleteCluster(ctx context.Context, serviceID string, isCancle bool) error {
	cluster, err := getClusterByServiceID(ctx, s.client, serviceID)
	if err != nil {
		return fmt.Errorf("get cluster by service_id %s: %w", serviceID, err)
	}

	log.Info("Found cluster for deletion",
		log.String("service_id", serviceID),
		log.String("cluster_name", cluster.Name),
		log.String("namespace", cluster.Namespace),
		log.String("current_termination_policy", string(cluster.Spec.TerminationPolicy)),
		log.Bool("wipe_out", isCancle))

	if isCancle && cluster.Spec.TerminationPolicy != kbappsv1.WipeOut {
		log.Info("Updating TerminationPolicy to WipeOut before deletion",
			log.String("cluster_name", cluster.Name),
			log.String("namespace", cluster.Namespace))

		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Spec.TerminationPolicy = kbappsv1.WipeOut

		if err := s.client.Patch(ctx, cluster, patch); err != nil {
			return fmt.Errorf("patch cluster %s/%s terminationPolicy to WipeOut: %w",
				cluster.Namespace, cluster.Name, err)
		}

		log.Info("Successfully updated TerminationPolicy to WipeOut",
			log.String("cluster_name", cluster.Name),
			log.String("namespace", cluster.Namespace))
	}

	policy := metav1.DeletePropagationForeground
	deleteOptions := &client.DeleteOptions{
		PropagationPolicy: &policy,
	}

	if err := s.client.Delete(ctx, cluster, deleteOptions); err != nil {
		return fmt.Errorf("delete cluster %s/%s: %w", cluster.Namespace, cluster.Name, err)
	}

	log.Info("Successfully initiated cluster deletion",
		log.String("service_id", serviceID),
		log.String("cluster_name", cluster.Name),
		log.String("namespace", cluster.Namespace),
		log.Bool("wipe_out", isCancle))

	return nil
}

// handleHorizontalScaling 处理水平伸缩（副本数）
func (s *ClusterService) handleHorizontalScaling(ctx context.Context, scalingCtx model.ExpansionContext) (bool, error) {
	if scalingCtx.DesiredReplicas == scalingCtx.CurrentReplicas {
		return false, nil
	}

	delta := scalingCtx.DesiredReplicas - scalingCtx.CurrentReplicas

	opsParams := model.HorizontalScalingOpsParams{
		Cluster:       scalingCtx.Cluster,
		ComponentName: scalingCtx.ComponentName,
		DeltaReplicas: delta,
	}

	if err := createHorizontalScalingOpsRequest(ctx, s.client, opsParams); err != nil {
		if err == ErrCreateOpsSkipped {
			return false, nil
		}
		return false, fmt.Errorf("create horizontal scaling opsrequest: %w", err)
	}

	log.Info("Created horizontal scaling OpsRequest",
		log.String("cluster", scalingCtx.Cluster.Name),
		log.String("component", scalingCtx.ComponentName),
		log.Int32("deltaReplicas", delta))

	return true, nil
}

// handleVerticalScaling 处理垂直伸缩（CPU/内存）
func (s *ClusterService) handleVerticalScaling(ctx context.Context, scalingCtx model.ExpansionContext) (bool, error) {
	needVScale := scalingCtx.CurrentCPU.Cmp(scalingCtx.DesiredCPU) != 0 ||
		scalingCtx.CurrentMem.Cmp(scalingCtx.DesiredMem) != 0

	if !needVScale {
		return false, nil
	}

	opsParams := model.VerticalScalingOpsParams{
		Cluster:       scalingCtx.Cluster,
		ComponentName: scalingCtx.ComponentName,
		CPU:           scalingCtx.DesiredCPU,
		Memory:        scalingCtx.DesiredMem,
	}

	if err := createVerticalScalingOpsRequest(ctx, s.client, opsParams); err != nil {
		if err == ErrCreateOpsSkipped {
			return false, nil
		}
		return false, fmt.Errorf("create vertical scaling opsrequest: %w", err)
	}

	log.Info("Created vertical scaling OpsRequest",
		log.String("cluster", scalingCtx.Cluster.Name),
		log.String("component", scalingCtx.ComponentName),
		log.String("desiredCPU", scalingCtx.DesiredCPU.String()),
		log.String("desiredMemory", scalingCtx.DesiredMem.String()))

	return true, nil
}

// handleVolumeExpansion 处理存储扩容
func (s *ClusterService) handleVolumeExpansion(ctx context.Context, scalingCtx model.ExpansionContext) (bool, error) {
	if !scalingCtx.HasPVC {
		return false, nil
	}

	switch scalingCtx.DesiredStorage.Cmp(scalingCtx.CurrentStorage) {
	case 0:
		return false, nil
	case -1:
		log.Warn("Storage shrinking detected but not supported, skipping operation",
			log.String("cluster", scalingCtx.Cluster.Name),
			log.String("component", scalingCtx.ComponentName),
			log.String("volumeTemplate", scalingCtx.VolumeTplName),
			log.String("currentStorage", scalingCtx.CurrentStorage.String()),
			log.String("desiredStorage", scalingCtx.DesiredStorage.String()))
		return false, nil
	case 1:
		canExpand := true
		var skipReason string

		if scalingCtx.StorageClassRef == nil || *scalingCtx.StorageClassRef == "" {
			canExpand = false
			skipReason = "storageClass not set on volumeClaimTemplate"
		} else {
			var sc storagev1.StorageClass
			if err := s.client.Get(ctx, client.ObjectKey{Name: *scalingCtx.StorageClassRef}, &sc); err != nil {
				log.Warn("Failed to get StorageClass, skipping volume expansion",
					log.String("cluster", scalingCtx.Cluster.Name),
					log.String("component", scalingCtx.ComponentName),
					log.String("volumeTemplate", scalingCtx.VolumeTplName),
					log.String("storageClass", *scalingCtx.StorageClassRef),
					log.String("error", err.Error()))
				canExpand = false
				skipReason = "failed to get StorageClass"
			} else if sc.AllowVolumeExpansion == nil || !*sc.AllowVolumeExpansion {
				canExpand = false
				skipReason = "StorageClass does not allow volume expansion"
			}
		}

		if !canExpand {
			log.Warn("Volume expansion skipped due to configuration constraints",
				log.String("cluster", scalingCtx.Cluster.Name),
				log.String("component", scalingCtx.ComponentName),
				log.String("volumeTemplate", scalingCtx.VolumeTplName),
				log.String("reason", skipReason),
				log.String("currentStorage", scalingCtx.CurrentStorage.String()),
				log.String("desiredStorage", scalingCtx.DesiredStorage.String()))
			return false, nil
		}

		opsParams := model.VolumeExpansionOpsParams{
			Cluster:                 scalingCtx.Cluster,
			ComponentName:           scalingCtx.ComponentName,
			VolumeClaimTemplateName: scalingCtx.VolumeTplName,
			Storage:                 scalingCtx.DesiredStorage,
		}

		if err := createVolumeExpansionOpsRequest(ctx, s.client, opsParams); err != nil {
			if err == ErrCreateOpsSkipped {
				return false, nil
			}
			return false, fmt.Errorf("create volume expansion opsrequest: %w", err)
		}

		log.Info("Created volume expansion OpsRequest",
			log.String("cluster", scalingCtx.Cluster.Name),
			log.String("component", scalingCtx.ComponentName),
			log.String("volumeTemplate", scalingCtx.VolumeTplName),
			log.String("desiredStorage", scalingCtx.DesiredStorage.String()))
		return true, nil
	}

	return false, nil
}

// getClusterPods 获取 Cluster 相关的 Pod 状态信息
func (s *ClusterService) getClusterPods(ctx context.Context, clusterName, namespace string) ([]model.PodStatus, error) {
	pods, err := getPodsByIndex(ctx, s.client, clusterName, namespace)
	if err != nil {
		return nil, fmt.Errorf("get pods by index: %w", err)
	}

	result := make([]model.PodStatus, 0, len(pods))
	for _, pod := range pods {
		result = append(result, buildPodStatus(pod))
	}

	return result, nil
}

// getPodsByIndex 使用索引查询 Pod，失败时回退到标签查询
func getPodsByIndex(ctx context.Context, c client.Client, clusterName, namespace string) ([]corev1.Pod, error) {
	var podList corev1.PodList

	indexKey := fmt.Sprintf("%s/%s", namespace, clusterName)
	if err := c.List(ctx, &podList, client.MatchingFields{index.NamespaceInstanceField: indexKey}); err == nil {
		return podList.Items, nil
	}

	selector := client.MatchingLabels{constant.AppInstanceLabelKey: clusterName}
	if err := c.List(ctx, &podList, selector, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list pods for cluster %s: %w", clusterName, err)
	}

	return podList.Items, nil
}

// buildPodStatus 构建 Pod 状态信息
func buildPodStatus(pod corev1.Pod) model.PodStatus {
	ready := false

	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			ready = true
			break
		}
	}

	return model.PodStatus{
		Name:   pod.Name,
		Status: pod.Status.Phase,
		Ready:  ready,
	}
}

// getLastReadyTransition 提取 Cluster 最近一次达到 Ready 且 Status 为 True 的时间点（metav1.Time）
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

// formatToISO8601Time 将标准 time.Time 转为 ISO 8601（RFC3339，UTC）字符串
func formatToISO8601Time(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
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
		return string(status)
	}
}

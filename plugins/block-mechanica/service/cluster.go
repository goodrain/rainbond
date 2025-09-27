package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/furutachiKurea/block-mechanica/internal/index"
	"github.com/furutachiKurea/block-mechanica/internal/log"
	"github.com/furutachiKurea/block-mechanica/internal/model"
	"github.com/furutachiKurea/block-mechanica/internal/mono"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
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
//
// 返回成功创建的 KubeBlocks Cluster 实例
func (s *ClusterService) CreateCluster(ctx context.Context, c model.ClusterInput) (*kbappsv1.Cluster, error) {
	if c.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	clusterAdapter, ok := _clusterRegistry[c.Type]
	if !ok {
		return nil, fmt.Errorf("unsupported cluster type: %s", c.Type)
	}

	cluster, err := clusterAdapter.Builder.BuildCluster(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("build %s cluster: %w", c.Type, err)
	}

	if err := s.client.Create(ctx, cluster); err != nil {
		return nil, fmt.Errorf("create cluster: %w", err)
	}

	c.Name = cluster.Name
	if err := s.associateToKubeBlocksComponent(ctx, cluster, c); err != nil {
		return nil, fmt.Errorf("associate to rainbond component: %w", err)
	}

	return cluster, nil
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
// 从 Kubernetes Secret 中获取数据库账户的用户名和密码
//
// Secret 名称由对应数据库类型的 Coordinator 适配器生成
func (s *ClusterService) GetConnectInfo(ctx context.Context, rbd model.RBDService) ([]model.ConnectInfo, error) {
	cluster, err := getClusterByServiceID(ctx, s.client, rbd.ServiceID)
	if err != nil {
		return nil, fmt.Errorf("get cluster by service_id %s: %w", rbd.ServiceID, err)
	}

	dbType := clusterType(cluster)
	clusterAdapter, ok := _clusterRegistry[dbType]
	if !ok {
		return nil, fmt.Errorf("unsupported cluster type: %s", dbType)
	}
	secretName := clusterAdapter.Coordinator.GetSecretName(cluster.Name)

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

// DeleteCluster 删除 KubeBlocks 数据库集群
//
// 批量删除指定 serviceIDs 对应的 Cluster，忽略找不到的 service_id
// TODO 命名，目前单数但是删除复数个, maybe DeleteCllusters
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

// ManageClustersLifecycle 通过创建 OpsRequest 批量管理多个 Cluster 的生命周期
func (s *ClusterService) ManageClustersLifecycle(ctx context.Context, operation opsv1alpha1.OpsType, serviceIDs []string) *model.BatchOperationResult {
	manageResult := model.NewBatchOperationResult()
	for _, serviceID := range serviceIDs {
		cluster, err := getClusterByServiceID(ctx, s.client, serviceID)
		if errors.Is(err, ErrTargetNotFound) {
			continue
		}
		if err != nil {
			manageResult.AddFailed(serviceID, err)
			continue
		}

		if err = createLifecycleOpsRequest(ctx, s.client, cluster, operation); err == nil {
			manageResult.AddSucceeded(serviceID)
		} else {
			manageResult.AddFailed(serviceID, err)
		}
	}
	return manageResult
}

// GetPodDetail 获取指定 Cluster 的 Pod detail
// 获取指定 service_id 的 Cluster 管理的指定 Pod 的详细信息
func (s *ClusterService) GetPodDetail(ctx context.Context, serviceID string, podName string) (*model.PodDetail, error) {
	cluster, err := getClusterByServiceID(ctx, s.client, serviceID)
	if err != nil && errors.Is(err, ErrTargetNotFound) {
		return nil, fmt.Errorf("get cluster by service_id %s: %w", serviceID, err)
	}

	pods, err := s.getClusterPods(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("get cluster pods: %w", err)
	}

	targetPod := findPodByName(pods, podName)
	if targetPod == nil {
		return nil, ErrTargetNotFound
	}

	pod := &corev1.Pod{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: podName, Namespace: cluster.Namespace}, pod); err != nil {
		return nil, fmt.Errorf("get pod %s: %w", podName, err)
	}

	version := ""
	componentDef := ""
	if len(cluster.Spec.ComponentSpecs) > 0 {
		// Version 字段按注释约定来自 componentDef，若为空则回退 serviceVersion
		if v := cluster.Spec.ComponentSpecs[0].ComponentDef; v != "" {
			version = v
		} else {
			version = cluster.Spec.ComponentSpecs[0].ServiceVersion
		}
		componentDef = cluster.Spec.ComponentSpecs[0].ComponentDef
	}

	status := buildPodDetailStatus(*pod)
	containers := buildContainerDetails(pod.Spec.Containers, pod.Status.ContainerStatuses, componentDef)
	events, err := getPodEvents(ctx, s.client, podName, pod.Namespace)
	if err != nil {
		log.Warn("Failed to get pod events",
			log.String("pod", podName),
			log.String("namespace", pod.Namespace),
			log.Err(err))
		events = []model.Event{}
	}

	startTime := ""
	if pod.Status.StartTime != nil {
		startTime = formatToISO8601Time(pod.Status.StartTime.Time)
	}

	podDetail := &model.PodDetail{
		Name:       pod.Name,
		NodeIP:     pod.Status.HostIP,
		StartTime:  startTime,
		IP:         pod.Status.PodIP,
		Version:    version,
		Namespace:  pod.Namespace,
		Status:     status,
		Containers: containers,
		Events:     events,
	}

	log.Debug("get pod detail",
		log.String("service_id", serviceID),
		log.String("pod", podName),
		log.Any("detail", podDetail))

	return podDetail, nil
}

// GetClusterEvents 获取指定 KubeBlocks Cluster 的运维事件列表
//
// 事件数据来源于与 Cluster 关联的 OpsRequest 资源，按创建时间降序排序
func (s *ClusterService) GetClusterEvents(ctx context.Context, serviceID string, page, pageSize int) ([]model.EventItem, error) {
	// 参数验证
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 6
	}
	if pageSize > 100 {
		pageSize = 100 // 限制最大页面大小
	}

	cluster, err := getClusterByServiceID(ctx, s.client, serviceID)
	if err != nil {
		return nil, fmt.Errorf("get cluster by service_id %s: %w", serviceID, err)
	}

	var opsList opsv1alpha1.OpsRequestList
	selector := labels.SelectorFromSet(labels.Set{
		constant.AppInstanceLabelKey: cluster.Name,
	})
	if err := s.client.List(ctx, &opsList, &client.ListOptions{
		Namespace:     cluster.Namespace,
		LabelSelector: selector,
	}); err != nil {
		return nil, fmt.Errorf("list opsrequests for cluster %s: %w", cluster.Name, err)
	}

	// 转换所有 OpsRequest 为 EventItem
	events := make([]model.EventItem, 0, len(opsList.Items))
	for _, ops := range opsList.Items {
		event := s.convertOpsRequestToEventItem(&ops)
		events = append(events, event)
	}

	// 按创建时间降序
	sort.Slice(events, func(i, j int) bool {
		return events[i].CreateTime > events[j].CreateTime
	})

	startIndex := (page - 1) * pageSize
	endIndex := startIndex + pageSize

	if startIndex >= len(events) {
		// 页码超出范围，返回空结果
		return []model.EventItem{}, nil
	}

	if endIndex > len(events) {
		endIndex = len(events)
	}

	pageEvents := events[startIndex:endIndex]

	return pageEvents, nil
}

// convertOpsRequestToEventItem 将 OpsRequest 转换为 EventItem
func (s *ClusterService) convertOpsRequestToEventItem(ops *opsv1alpha1.OpsRequest) model.EventItem {
	var message, reason, status, finalStatus, endTime string

	if !ops.Status.CompletionTimestamp.IsZero() {
		endTime = formatToISO8601Time(ops.Status.CompletionTimestamp.Time)
	}

	switch ops.Status.Phase {
	case opsv1alpha1.OpsSucceedPhase:
		status = "success"
		finalStatus = "complete"
		message = "Operation completed successfully"
	case opsv1alpha1.OpsFailedPhase:
		status = "failure"
		finalStatus = "complete"
		// 优先从 condition 中获取详细失败信息
		if cond := findFailedCondition(ops.Status.Conditions); cond != nil {
			message = cond.Message
			reason = cond.Reason
		} else {
			message = "Operation failed with unknown reason"
		}
	case opsv1alpha1.OpsCancelledPhase:
		status = "failure"
		finalStatus = "complete"
		message = "Operation was cancelled"
	default:
		status = ""
		finalStatus = ""
		message = "Operation is in progress"
	}

	return model.EventItem{
		OpsName:     ops.Name,
		OpsType:     toRainbondOptType(ops.Spec.Type),
		UserName:    "BlockMechanica",
		Status:      status,
		FinalStatus: finalStatus,
		Message:     message,
		Reason:      reason,
		CreateTime:  formatToISO8601Time(ops.CreationTimestamp.Time),
		EndTime:     endTime,
	}
}

func toRainbondOptType(opsType opsv1alpha1.OpsType) string {
	switch opsType {
	case opsv1alpha1.VerticalScalingType:
		// Vertical Scaling
		return "vertical-service"
	case opsv1alpha1.HorizontalScalingType:
		// Horizontal Scaling
		return "horizontal-service"
	case opsv1alpha1.VolumeExpansionType:
		// Storage Expansion
		return "update-service-volume"
	case opsv1alpha1.BackupType:
		return "backup-database"
	default:
		return ""
	}
}

// findFailedCondition 查找失败状态的 Condition
func findFailedCondition(conditions []metav1.Condition) *metav1.Condition {
	for _, cond := range conditions {
		if cond.Status == metav1.ConditionFalse {
			return &cond
		}
	}
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
func (s *ClusterService) getClusterPods(ctx context.Context, cluster *kbappsv1.Cluster) ([]model.Status, error) {
	componentName, err := extractComponentName(cluster)
	if err != nil {
		return nil, err
	}

	// 通过 InstanceSet 获取 Pod
	instanceSet, err := getInstanceSetByCluster(ctx, s.client, cluster.Name, cluster.Namespace, componentName)
	if err != nil {
		if errors.Is(err, ErrTargetNotFound) {
			// InstanceSet 不存在时返回空列表，而不是错误
			log.Info("InstanceSet not found, returning empty pod list",
				log.String("cluster", cluster.Name),
				log.String("component", componentName))
			return []model.Status{}, nil
		}
		return nil, fmt.Errorf("get instanceset: %w", err)
	}

	var podNames []string
	for _, instanceStatus := range instanceSet.Status.InstanceStatus {
		podNames = append(podNames, instanceStatus.PodName)
	}

	pods, err := getPodsByNames(ctx, s.client, podNames, cluster.Namespace)
	if err != nil {
		return nil, fmt.Errorf("get pods by names: %w", err)
	}

	result := make([]model.Status, 0, len(pods))
	for _, pod := range pods {
		result = append(result, buildPodStatus(pod))
	}

	return result, nil
}

// extractComponentName 从 Cluster 中提取组件名称
func extractComponentName(cluster *kbappsv1.Cluster) (string, error) {
	if len(cluster.Spec.ComponentSpecs) == 0 {
		return "", fmt.Errorf("cluster %s/%s has no componentSpecs", cluster.Namespace, cluster.Name)
	}

	componentName := cluster.Spec.ComponentSpecs[0].Name
	if componentName == "" {
		componentName = cluster.Spec.ClusterDef
	}
	return componentName, nil
}

// getInstanceSetByCluster 通过 cluster 和 component 获取 InstanceSet
func getInstanceSetByCluster(ctx context.Context, c client.Client, clusterName, namespace, componentName string) (*workloadsv1.InstanceSet, error) {
	var instanceSetList workloadsv1.InstanceSetList

	// 优先使用索引查询
	indexKey := fmt.Sprintf("%s/%s/%s", namespace, clusterName, componentName)
	if err := c.List(ctx, &instanceSetList, client.MatchingFields{index.NamespaceClusterComponentField: indexKey}); err == nil {
		switch len(instanceSetList.Items) {
		case 0:
			return nil, ErrTargetNotFound
		case 1:
			return &instanceSetList.Items[0], nil
		default:
			return nil, ErrMultipleFounded
		}
	} else {
		log.Warn("Index query failed, falling back to label query",
			log.String("indexKey", indexKey), log.Err(err))
	}

	// 回退到标签查询
	selector := client.MatchingLabels{
		constant.AppInstanceLabelKey:        clusterName,
		"apps.kubeblocks.io/component-name": componentName,
	}
	if err := c.List(ctx, &instanceSetList, selector, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list instanceset for cluster %s component %s: %w", clusterName, componentName, err)
	}

	switch len(instanceSetList.Items) {
	case 0:
		return nil, ErrTargetNotFound
	case 1:
		return &instanceSetList.Items[0], nil
	default:
		return nil, ErrMultipleFounded
	}
}

// getPodsByNames 根据 Pod 名称列表查询 Pod
func getPodsByNames(ctx context.Context, c client.Client, podNames []string, namespace string) ([]corev1.Pod, error) {
	var pods []corev1.Pod

	for _, podName := range podNames {
		var pod corev1.Pod
		if err := c.Get(ctx, client.ObjectKey{Name: podName, Namespace: namespace}, &pod); err != nil {
			log.Warn("Failed to get pod", log.String("pod", podName), log.Err(err))
			continue
		}
		pods = append(pods, pod)
	}

	return pods, nil
}

// buildPodStatus 构建 Pod 状态信息
func buildPodStatus(pod corev1.Pod) model.Status {
	ready := false

	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			ready = true
			break
		}
	}

	return model.Status{
		Name:   pod.Name,
		Status: pod.Status.Phase,
		Ready:  ready,
	}
}

// buildPodDetailStatus 构建符合注释约定的 PodStatus（包含 type_str/reason/message/advice）
func buildPodDetailStatus(pod corev1.Pod) model.PodStatus {
	typeStr := strings.ToLower(string(pod.Status.Phase))
	reason := ""
	message := ""
	advice := ""

	// 优先取 Waiting 的容器状态
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			reason = cs.State.Waiting.Reason
			message = cs.State.Waiting.Message
			advice = deriveAdvice(reason, message)
			break
		}
	}
	// 其次取 Terminated 的容器状态
	if reason == "" {
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Terminated != nil {
				reason = cs.State.Terminated.Reason
				message = cs.State.Terminated.Message
				advice = deriveAdvice(reason, message)
				break
			}
		}
	}

	return model.PodStatus{
		TypeStr: typeStr,
		Reason:  reason,
		Message: message,
		Advice:  advice,
	}
}

// deriveAdvice 将常见的 reason 映射为建议性结论
func deriveAdvice(reason, message string) string {
	switch reason {
	case "OOMKilled":
		return "OutOfMemory"
	case "ImagePullBackOff", "ErrImagePull":
		return "ImagePullError"
	default:
		_ = message // 预留后续扩展
		return ""
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

// findPodByName 在 Pod 状态列表中查找指定名称的 Pod
func findPodByName(pods []model.Status, podName string) *model.Status {
	for _, pod := range pods {
		if pod.Name == podName {
			return &pod
		}
	}
	return nil
}

// buildContainerDetails 构建容器详情列表，只返回设置了资源限制的工作容器
func buildContainerDetails(containers []corev1.Container, containerStatuses []corev1.ContainerStatus, componentDef string) []model.Container {
	var details []model.Container

	statusMap := make(map[string]corev1.ContainerStatus)
	for _, status := range containerStatuses {
		statusMap[status.Name] = status
	}

	for _, container := range containers {
		if !hasResourceLimits(container.Resources.Limits) {
			continue
		}

		status, exists := statusMap[container.Name]
		if !exists {
			continue
		}

		startedTime := ""
		state := "Unknown"
		reason := ""

		if status.State.Running != nil {
			startedTime = formatToISO8601Time(status.State.Running.StartedAt.Time)
			state = "Running"
		} else if status.State.Waiting != nil {
			state = "Waiting"
			reason = status.State.Waiting.Reason
		} else if status.State.Terminated != nil {
			state = "Terminated"
			reason = status.State.Terminated.Reason
		}

		limitCPU := ""
		if cpu := container.Resources.Limits.Cpu(); cpu != nil {
			limitCPU = cpu.String()
		}

		limitMemory := ""
		if memory := container.Resources.Limits.Memory(); memory != nil {
			limitMemory = memory.String()
		}

		containerDetail := model.Container{
			ComponentDef: componentDef,
			LimitMemory:  limitMemory,
			LimitCPU:     limitCPU,
			Started:      startedTime,
			State:        state,
			Reason:       reason,
		}

		details = append(details, containerDetail)
	}

	return details
}

// hasResourceLimits 检查是否设置了 CPU 或 Memory 资源限制
func hasResourceLimits(limits corev1.ResourceList) bool {
	if limits == nil {
		return false
	}

	cpu, hasCPU := limits[corev1.ResourceCPU]
	memory, hasMemory := limits[corev1.ResourceMemory]

	return (hasCPU && !cpu.IsZero()) || (hasMemory && !memory.IsZero())
}

// getPodEvents 查询并格式化指定 Pod 的相关事件
func getPodEvents(ctx context.Context, c client.Client, podName, namespace string) ([]model.Event, error) {
	var eventList corev1.EventList

	fieldSelector := fields.OneTermEqualSelector("involvedObject.name", podName)
	listOptions := &client.ListOptions{
		FieldSelector: fieldSelector,
		Namespace:     namespace,
	}

	if err := c.List(ctx, &eventList, listOptions); err != nil {
		return nil, fmt.Errorf("list events for pod %s: %w", podName, err)
	}

	sort.Slice(eventList.Items, func(i, j int) bool {
		return eventList.Items[i].FirstTimestamp.After(eventList.Items[j].FirstTimestamp.Time)
	})

	const maxEvents = 10
	endIndex := len(eventList.Items)
	if endIndex > maxEvents {
		endIndex = maxEvents
	}

	events := make([]model.Event, 0, endIndex)
	for i := 0; i < endIndex; i++ {
		event := eventList.Items[i]
		events = append(events, model.Event{
			Type:    event.Type,
			Reason:  event.Reason,
			Age:     formatAge(event.FirstTimestamp),
			Message: event.Message,
		})
	}

	return events, nil
}

// formatAge 将时间差格式化为人类可读的格式 (如 "5m", "2h", "3d")
func formatAge(eventTime metav1.Time) string {
	if eventTime.IsZero() {
		return ""
	}

	duration := time.Since(eventTime.Time)

	if duration < time.Minute {
		return fmt.Sprintf("%.0fs", duration.Seconds())
	} else if duration < time.Hour {
		return fmt.Sprintf("%.0fm", duration.Minutes())
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%.0fh", duration.Hours())
	} else {
		return fmt.Sprintf("%.0fd", duration.Hours()/24)
	}
}

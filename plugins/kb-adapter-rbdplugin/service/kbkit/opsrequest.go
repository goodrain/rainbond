package kbkit

import (
	"context"
	"crypto/md5"
	"fmt"
	"strings"
	"time"

	"github.com/furutachiKurea/block-mechanica/internal/index"
	"github.com/furutachiKurea/block-mechanica/internal/model"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OpsRequest 配置项
var (
	opsTimeoutSecond        int32 = 10 * 60
	opsPreConditionDeadline int32 = 5 * 60

	defaultBackupDeletionPolicy = "Delete"

	// opsCleanupConcurrency 控制并发清理 OpsRequest 时的最大并发数
	opsCleanupConcurrency = 4
)

const (
	preflightProceed           preflightDecision = iota + 1 // 创建
	preflightSkip                                           // 跳过
	preflightCleanupAndProceed                              // 清理阻塞操作后创建
)

type preflightDecision int

type preflightResult struct {
	Decision preflightDecision
}

// preflight 规定 OpsRequest 创建前/后的决策逻辑
type preflight interface {
	// decide 根据创建目标判断是否允许创建
	decide(ctx context.Context, c client.Client, ops *opsv1alpha1.OpsRequest) (preflightResult, error)
}

// uniqueOps 检查是否存在会阻塞新 OpsRequest 的同类型 OpsRequest,
// 如果存在阻塞操作，则跳过创建；否则允许创建
type uniqueOps struct{}

func (uniqueOps) decide(ctx context.Context, c client.Client, ops *opsv1alpha1.OpsRequest) (preflightResult, error) {
	opsList, err := getOpsRequestsByIndex(ctx, c, ops.Namespace, ops.Spec.ClusterName, ops.Spec.Type)
	if err != nil {
		if !errors.IsNotFound(err) {
			return preflightResult{}, fmt.Errorf("list opsrequests for preflight: %w", err)
		}
		return preflightResult{Decision: preflightProceed}, nil
	}

	for _, ops := range opsList {
		if !isOpsRequestNonBlocking(&ops) {
			return preflightResult{Decision: preflightSkip}, nil
		}
	}

	return preflightResult{Decision: preflightProceed}, nil
}

// priorityOps 优先级操作预检策略，用于处理高优先级操作（重启、停止、启动）,
// 当使用此策略时，会主动清理所有阻塞的 OpsRequest，确保优先级操作能够立即执行,
// 清理操作必须完全成功，否则整个创建过程将失败
type priorityOps struct{}

func (priorityOps) decide(ctx context.Context, c client.Client, ops *opsv1alpha1.OpsRequest) (preflightResult, error) {
	// 查询集群的所有非终态 OpsRequest
	blockingOps, err := GetAllNonFinalOpsRequests(ctx, c, ops.Namespace, ops.Spec.ClusterName)
	if err != nil {
		return preflightResult{}, fmt.Errorf("get existing opsrequests: %w", err)
	}

	// 如果没有需要清理的操作，直接创建
	if len(blockingOps) == 0 {
		return preflightResult{Decision: preflightProceed}, nil
	}

	if err := CleanupBlockingOps(ctx, c, blockingOps); err != nil {
		return preflightResult{}, err
	}

	return preflightResult{Decision: preflightCleanupAndProceed}, nil
}

// cancelOps 优雅取消操作预检策略，用于处理伸缩操作（水平伸缩、垂直伸缩）
//
// 取消将同类型会阻塞的 OpsRequest，然后允许创建新的伸缩操作
type cancelOps struct{}

func (cancelOps) decide(ctx context.Context, c client.Client, ops *opsv1alpha1.OpsRequest) (preflightResult, error) {
	// 查找同类型的会阻塞的 OpsRequest
	existingOps, err := getOpsRequestsByIndex(ctx, c, ops.Namespace, ops.Spec.ClusterName, ops.Spec.Type)
	if err != nil {
		if !errors.IsNotFound(err) {
			return preflightResult{}, fmt.Errorf("list opsrequests for preflight: %w", err)
		}
		return preflightResult{Decision: preflightProceed}, nil
	}

	// 收集需要优雅取消的 OpsRequest
	var toCancel []*opsv1alpha1.OpsRequest
	for i := range existingOps {
		opsReq := &existingOps[i]
		if !isOpsRequestNonBlocking(opsReq) {
			toCancel = append(toCancel, opsReq)
		}
	}

	// 如果没有需要取消的 OpsRequest，直接创建
	if len(toCancel) == 0 {
		return preflightResult{Decision: preflightProceed}, nil
	}

	// 取消 OpsRequest
	if err := cancelOpsRequests(ctx, c, toCancel); err != nil {
		return preflightResult{}, fmt.Errorf("failed to gracefully cancel existing operations: %w", err)
	}

	return preflightResult{Decision: preflightProceed}, nil
}

type createOpts struct {
	preflight preflight
}

type createOption func(*createOpts)

func validateCluster(cluster *kbappsv1.Cluster) error {
	if cluster == nil {
		return ErrClusterRequired
	}
	return nil
}

// withPreflight 自定义预检策略
func withPreflight(p preflight) createOption {
	return func(o *createOpts) { o.preflight = p }
}

// CreateLifecycleOpsRequest 创建生命周期管理相关的 OpsRequest，供 Reconciler 使用
// 重启、停止，将被设置为 force: true 和 enqueueOnForce: false, 同时使用 priorityOps 移除正在阻塞的 OpsRequest，确保能够立即执行
func CreateLifecycleOpsRequest(ctx context.Context,
	c client.Client,
	cluster *kbappsv1.Cluster,
	opsType opsv1alpha1.OpsType,
) error {
	if err := validateCluster(cluster); err != nil {
		return err
	}

	opsSpecific := opsv1alpha1.SpecificOpsRequest{}
	if opsType == opsv1alpha1.RestartType {
		opsSpecific.RestartList = []opsv1alpha1.ComponentOps{
			{
				ComponentName: ClusterType(cluster),
			},
		}
	}

	if _, err := createOpsRequest(ctx, c, cluster, opsType, opsSpecific, withPreflight(priorityOps{})); err != nil {
		return err
	}

	return nil
}

// CreateBackupOpsRequest 为指定的 Cluster 创建备份 OpsRequest
//
// backupMethod 为备份方法，不做任何额外预检，支持同时多次备份操作
func CreateBackupOpsRequest(ctx context.Context,
	c client.Client,
	cluster *kbappsv1.Cluster,
	backupMethod string,
) error {
	if err := validateCluster(cluster); err != nil {
		return err
	}

	specificOps := opsv1alpha1.SpecificOpsRequest{
		Backup: &opsv1alpha1.Backup{
			BackupPolicyName: fmt.Sprintf("%s-%s-backup-policy", cluster.Name, ClusterType(cluster)),
			BackupMethod:     backupMethod,
			DeletionPolicy:   defaultBackupDeletionPolicy,
		},
	}

	_, err := createOpsRequest(ctx, c, cluster, opsv1alpha1.BackupType, specificOps)
	return err
}

// CreateHorizontalScalingOpsRequest 为指定的 Cluster 创建水平伸缩 OpsRequest
//
// 将设置 enqueueOnForce: false 并将已经创建了的同类型 OpsRequest 设置为取消状态以避免阻塞
func CreateHorizontalScalingOpsRequest(ctx context.Context,
	c client.Client,
	params model.HorizontalScalingOpsParams,
) error {
	if err := validateCluster(params.Cluster); err != nil {
		return err
	}
	var horizontalScalingList []opsv1alpha1.HorizontalScaling

	// 遍历所有组件，为每个组件创建对应的伸缩配置
	for _, component := range params.Components {
		var scaling opsv1alpha1.HorizontalScaling

		if component.DeltaReplicas > 0 {
			// ScaleOut
			scaling = opsv1alpha1.HorizontalScaling{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: component.Name},
				ScaleOut: &opsv1alpha1.ScaleOut{
					ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: &component.DeltaReplicas},
				},
			}
		} else {
			// ScaleIn
			absReplicas := -component.DeltaReplicas
			scaling = opsv1alpha1.HorizontalScaling{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: component.Name},
				ScaleIn: &opsv1alpha1.ScaleIn{
					ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: &absReplicas},
				},
			}
		}

		horizontalScalingList = append(horizontalScalingList, scaling)
	}

	specificOps := opsv1alpha1.SpecificOpsRequest{
		HorizontalScalingList: horizontalScalingList,
	}

	_, err := createOpsRequest(ctx, c, params.Cluster, opsv1alpha1.HorizontalScalingType, specificOps, withPreflight(cancelOps{}))
	return err
}

// CreateVerticalScalingOpsRequest 为指定的 Cluster 创建垂直伸缩 OpsRequest
//
// 将设置 enqueueOnForce: false 并将已经创建了的同类型 OpsRequest 设置为取消状态以避免阻塞
func CreateVerticalScalingOpsRequest(ctx context.Context,
	c client.Client,
	params model.VerticalScalingOpsParams,
) error {
	if err := validateCluster(params.Cluster); err != nil {
		return err
	}
	var verticalScalingList []opsv1alpha1.VerticalScaling

	// 遍历所有组件，为每个组件创建对应的资源配置
	for _, component := range params.Components {
		verticalScalingList = append(verticalScalingList, opsv1alpha1.VerticalScaling{
			ComponentOps: opsv1alpha1.ComponentOps{ComponentName: component.Name},
			ResourceRequirements: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    component.CPU,
					corev1.ResourceMemory: component.Memory,
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    component.CPU,
					corev1.ResourceMemory: component.Memory,
				},
			},
		})
	}

	specificOps := opsv1alpha1.SpecificOpsRequest{
		VerticalScalingList: verticalScalingList,
	}

	_, err := createOpsRequest(ctx, c, params.Cluster, opsv1alpha1.VerticalScalingType, specificOps, withPreflight(cancelOps{}))
	return err
}

// CreateVolumeExpansionOpsRequest 为指定的 Cluster 创建存储扩容 OpsRequest
//
// 使用 uniqueOps 预检策略，确保不会创建重复的存储扩容 OpsRequest(VolumeExpansion 不支持 cancel 的折中方案)
func CreateVolumeExpansionOpsRequest(ctx context.Context,
	c client.Client,
	params model.VolumeExpansionOpsParams,
) error {
	if err := validateCluster(params.Cluster); err != nil {
		return err
	}
	var volumeExpansionList []opsv1alpha1.VolumeExpansion

	// 遍历所有组件，为每个组件创建对应的存储扩容配置
	for _, component := range params.Components {
		volumeExpansionList = append(volumeExpansionList, opsv1alpha1.VolumeExpansion{
			ComponentOps: opsv1alpha1.ComponentOps{ComponentName: component.Name},
			VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{
				{
					Name:    component.VolumeClaimTemplateName,
					Storage: component.Storage,
				},
			},
		})
	}

	specificOps := opsv1alpha1.SpecificOpsRequest{
		VolumeExpansionList: volumeExpansionList,
	}

	_, err := createOpsRequest(ctx, c, params.Cluster, opsv1alpha1.VolumeExpansionType, specificOps, withPreflight(uniqueOps{}))
	return err
}

// CreateParameterChangeOpsRequest 创建参数变更 OpsRequest
//
// 不做任何额外预检，由 KubeBlocks 处理
func CreateParameterChangeOpsRequest(ctx context.Context,
	c client.Client,
	cluster *kbappsv1.Cluster,
	parameters []model.ParameterEntry,
) error {
	if err := validateCluster(cluster); err != nil {
		return err
	}
	specificOps := opsv1alpha1.SpecificOpsRequest{
		Reconfigures: []opsv1alpha1.Reconfigure{
			{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: ClusterType(cluster)},
			},
		},
	}

	var parameterPairs []opsv1alpha1.ParameterPair
	for _, parameter := range parameters {
		if strValue, ok := parameter.Value.(*string); ok {
			parameterPairs = append(parameterPairs, opsv1alpha1.ParameterPair{
				Key:   parameter.Name,
				Value: strValue,
			})
		}
	}

	specificOps.Reconfigures[0].Parameters = parameterPairs

	_, err := createOpsRequest(ctx, c, cluster, opsv1alpha1.ReconfiguringType, specificOps)
	return err
}

// CreateRestoreOpsRequest 使用 backupName 指定一个 backup 创建 Restore OpsRequest，从备份中恢复 cluster
//
// 通过 backup 恢复的 Cluster 的名称格式为 {cluster.Name(去除四位后缀)}-restore-{四位随机后缀},
// 串行恢复卷声明，在集群进行 running 状态后执行 PostReady
//
// 不需要做任何额外预检，由 KubeBlocks 处理
func CreateRestoreOpsRequest(ctx context.Context,
	c client.Client,
	cluster *kbappsv1.Cluster,
	backupName string,
) (*opsv1alpha1.OpsRequest, error) {
	if err := validateCluster(cluster); err != nil {
		return nil, err
	}
	specificOps := opsv1alpha1.SpecificOpsRequest{
		Restore: &opsv1alpha1.Restore{
			BackupName:                        backupName,
			VolumeRestorePolicy:               "Serial",
			DeferPostReadyUntilClusterRunning: true,
		},
	}

	return createOpsRequest(ctx, c, cluster, opsv1alpha1.RestoreType, specificOps)
}

// createOpsRequest 创建 OpsRequest
//
// OpsRequest 的名称格式为 {clustername}-{opsType}-{timestamp}，
// 使用时间戳确保每次操作都有唯一的名称
func createOpsRequest(
	ctx context.Context,
	c client.Client,
	cluster *kbappsv1.Cluster,
	opsType opsv1alpha1.OpsType,
	specificOps opsv1alpha1.SpecificOpsRequest,
	opts ...createOption,
) (*opsv1alpha1.OpsRequest, error) {
	if err := validateCluster(cluster); err != nil {
		return nil, err
	}
	options := applyCreateOptions(opts...)

	ops := buildOpsRequest(cluster, opsType, specificOps)

	if options.preflight != nil {
		res, err := options.preflight.decide(ctx, c, ops)
		if err != nil {
			return nil, fmt.Errorf("preflight check for opsruqest %s failed: %w", ops.Name, err)
		}

		if res.Decision == preflightSkip {
			return nil, ErrCreateOpsSkipped
		}
	}

	if err := c.Create(ctx, ops); err != nil {
		if errors.IsAlreadyExists(err) {
			return nil, ErrCreateOpsSkipped
		}
		return nil, fmt.Errorf("create opsrequest %s: %w", ops.Name, err)
	}

	return ops, nil
}

// buildOpsRequest 构造 OpsRequest 对象
func buildOpsRequest(
	cluster *kbappsv1.Cluster,
	opsType opsv1alpha1.OpsType,
	specificOps opsv1alpha1.SpecificOpsRequest,
) *opsv1alpha1.OpsRequest {
	name := makeOpsRequestName(cluster.Name, opsType)

	serviceID := cluster.GetLabels()[index.ServiceIDLabel]

	labels := map[string]string{
		constant.AppInstanceLabelKey:    cluster.Name,
		constant.OpsRequestTypeLabelKey: string(opsType),
		index.ServiceIDLabel:            serviceID,
	}

	ops := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cluster.Namespace,
			Labels:    labels,
		},
		Spec: opsv1alpha1.OpsRequestSpec{
			ClusterName:                 cluster.Name,
			Type:                        opsType,
			TimeoutSeconds:              &opsTimeoutSecond,
			PreConditionDeadlineSeconds: &opsPreConditionDeadline,

			SpecificOpsRequest: specificOps,
		},
	}

	// 依据 opsType 设置不同的 spec 字段
	switch opsType {
	case opsv1alpha1.RestoreType:
		// Restore 中 ClusterName 为通过备份恢复的 Cluster 的名称，会创建一个新的 Cluster，
		// 应当按照 {cluster.Name(去除后缀)}-restore-{四位随机后缀}" 的格式
		ops.Spec.ClusterName = generateRestoredClusterName(cluster.Name)
	case opsv1alpha1.VerticalScalingType, opsv1alpha1.HorizontalScalingType:
		// 对于伸缩操作，应当绕过系统预检查
		ops.Spec.Force = true
	case opsv1alpha1.RestartType, opsv1alpha1.StopType:
		// 对于生命周期操作，应当绕过系统预检查并且不再排队(启动不支持 Force)
		ops.Spec.Force = true
		ops.Spec.EnqueueOnForce = false
	}

	return ops
}

func applyCreateOptions(opts ...createOption) *createOpts {
	o := &createOpts{}
	for _, f := range opts {
		if f != nil {
			f(o)
		}
	}
	return o
}

// makeOpsRequestName 生成 OpsRequest 名称
// 格式：{clustername}-{opsType}-{timestamp}
func makeOpsRequestName(clusterName string, opsType opsv1alpha1.OpsType) string {
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s-%s-%x", clusterName, strings.ToLower(string(opsType)), timestamp)
}

// getOpsRequestsByIndex 使用索引查询 OpsRequest，失败时回退到标签查询
func getOpsRequestsByIndex(ctx context.Context, c client.Client, namespace, clusterName string, opsType opsv1alpha1.OpsType) ([]opsv1alpha1.OpsRequest, error) {
	var list opsv1alpha1.OpsRequestList

	indexKey := fmt.Sprintf("%s/%s/%s", namespace, clusterName, opsType)
	if err := c.List(ctx, &list, client.MatchingFields{index.NamespaceClusterOpsTypeField: indexKey}); err == nil {
		return list.Items, nil
	}

	if err := c.List(ctx, &list,
		client.InNamespace(namespace),
		client.MatchingLabels(map[string]string{
			constant.AppInstanceLabelKey:    clusterName,
			constant.OpsRequestTypeLabelKey: string(opsType),
		}),
	); err != nil {
		return nil, fmt.Errorf("list opsrequests for preflight: %w", err)
	}

	return list.Items, nil
}

// isOpsRequestNonBlocking 检查 OpsRequest 是否会阻塞其他 OpsRequest
func isOpsRequestNonBlocking(ops *opsv1alpha1.OpsRequest) bool {
	phase := ops.Status.Phase
	return phase == opsv1alpha1.OpsSucceedPhase ||
		phase == opsv1alpha1.OpsCancelledPhase ||
		phase == opsv1alpha1.OpsFailedPhase ||
		phase == opsv1alpha1.OpsAbortedPhase ||
		phase == opsv1alpha1.OpsCancellingPhase
}

// generateRestoredClusterName 生成 restore cluster 的名称
// 格式：{cluster.Name(去除后缀)}-restore-{四位随机后缀}
func generateRestoredClusterName(originalClusterName string) string {
	var baseName string

	// 避免重复叠加 restore 后缀
	if strings.Contains(originalClusterName, "-restore-") {
		restoreIndex := strings.LastIndex(originalClusterName, "-restore-")
		baseName = originalClusterName[:restoreIndex]
	} else {
		lastDash := strings.LastIndex(originalClusterName, "-")
		baseName = originalClusterName[:lastDash]
	}

	// 生成4位随机后缀
	timestamp := time.Now().UnixNano()
	input := fmt.Sprintf("%s-restore-%d", baseName, timestamp)
	hash := md5.Sum([]byte(input))
	hashSuffix := fmt.Sprintf("%x", hash[:2])

	return fmt.Sprintf("%s-restore-%s", baseName, hashSuffix)
}

// GetAllNonFinalOpsRequests 获取指定集群的所有非终态 OpsRequest
// 不限制操作类型，返回所有可能阻塞的 OpsRequest
func GetAllNonFinalOpsRequests(ctx context.Context, c client.Client, namespace, clusterName string) ([]opsv1alpha1.OpsRequest, error) {
	var list opsv1alpha1.OpsRequestList

	if err := c.List(ctx, &list,
		client.InNamespace(namespace),
		client.MatchingLabels(map[string]string{
			constant.AppInstanceLabelKey: clusterName,
		}),
	); err != nil {
		return nil, fmt.Errorf("list all opsrequests for cluster %s/%s: %w", namespace, clusterName, err)
	}

	var nonFinalOps []opsv1alpha1.OpsRequest
	for _, ops := range list.Items {
		if !isOpsRequestNonBlocking(&ops) {
			nonFinalOps = append(nonFinalOps, ops)
		}
	}

	return nonFinalOps, nil
}

// classifyBlockingOps 按照是否支持 cancel 将阻塞的 OpsRequest 分成两组
// 横向/纵向伸缩 OpsRequest 支持 cancel，其他类型需要通过 expire 处理
func classifyBlockingOps(blockingOps []opsv1alpha1.OpsRequest) (toCancel, toExpire []*opsv1alpha1.OpsRequest) {

	for i := range blockingOps {
		op := &blockingOps[i]
		switch op.Spec.Type {
		case opsv1alpha1.HorizontalScalingType, opsv1alpha1.VerticalScalingType:
			toCancel = append(toCancel, op)
		default:
			toExpire = append(toExpire, op)
		}
	}

	return toCancel, toExpire
}

// CleanupBlockingOps 清理阻塞的 OpsRequest，先取消可优雅处理的，再缩短其余超时
func CleanupBlockingOps(
	ctx context.Context,
	c client.Client,
	blockingOps []opsv1alpha1.OpsRequest,
) error {
	toCancel, toExpire := classifyBlockingOps(blockingOps)

	group, gctx := errgroup.WithContext(ctx)

	if len(toCancel) > 0 {
		group.Go(func() error {
			if err := cancelOpsRequests(gctx, c, toCancel); err != nil {
				return fmt.Errorf("cancel blocking scaling operations: %w", err)
			}
			return nil
		})
	}

	if len(toExpire) > 0 {
		group.Go(func() error {
			if err := expireOpsRequests(gctx, c, toExpire); err != nil {
				return fmt.Errorf("expire blocking operations: %w", err)
			}
			return nil
		})
	}

	return group.Wait()
}

// cancelOpsRequests 取消所有给定的 OpsRequest
func cancelOpsRequests(ctx context.Context, c client.Client, toCancel []*opsv1alpha1.OpsRequest) error {
	if len(toCancel) == 0 {
		return nil
	}

	group, gctx := errgroup.WithContext(ctx)
	group.SetLimit(opsCleanupConcurrency)

	for i := range toCancel {
		op := toCancel[i]
		group.Go(func() error {
			if err := setOpsRequestToCancel(gctx, c, op); err != nil {
				return fmt.Errorf("failed to cancel opsrequest %s: %w", op.Name, err)
			}
			return nil
		})
	}

	return group.Wait()
}

// expireOpsRequests 将给定的 OpsRequest 的 timeoutSeconds 缩短到 1 秒，使其快速结束
func expireOpsRequests(ctx context.Context, c client.Client, toExpire []*opsv1alpha1.OpsRequest) error {
	if len(toExpire) == 0 {
		return nil
	}

	group, gctx := errgroup.WithContext(ctx)
	group.SetLimit(opsCleanupConcurrency)

	for i := range toExpire {
		op := toExpire[i]
		group.Go(func() error {
			if err := shortenOpsRequestTimeout(gctx, c, op); err != nil {
				return fmt.Errorf("failed to shorten timeout for opsrequest %s: %w", op.Name, err)
			}
			return nil
		})
	}

	return group.Wait()
}

// setOpsRequestToCancel 设置单个 OpsRequest 为 cancel: true
//
// <https://kubeblocks.io/docs/release-1_0_1/user_docs/references/api-reference/operations#operations.kubeblocks.io/v1alpha1.OpsRequest>
func setOpsRequestToCancel(ctx context.Context, c client.Client, ops *opsv1alpha1.OpsRequest) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &opsv1alpha1.OpsRequest{}
		if err := c.Get(ctx, client.ObjectKeyFromObject(ops), current); err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}

		// 检查操作是否已经处于终态或已经被取消
		if isOpsRequestNonBlocking(current) || current.Spec.Cancel {
			return nil
		}

		// 构造 Strategic Merge Patch，参考 associateToKubeBlocksComponent 的方式
		patchData := `{
			"spec": {
				"cancel": true
			}
		}`

		return c.Patch(ctx, current, client.RawPatch(types.MergePatchType, []byte(patchData)))
	})
}

// shortenOpsRequestTimeout 缩短 OpsRequest 的 timeoutSeconds 到 1 秒，使其快速结束
//
// <https://kubeblocks.io/docs/release-1_0_1/user_docs/references/api-reference/operations#operations.kubeblocks.io/v1alpha1.OpsRequest>
func shortenOpsRequestTimeout(ctx context.Context, c client.Client, ops *opsv1alpha1.OpsRequest) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &opsv1alpha1.OpsRequest{}
		if err := c.Get(ctx, client.ObjectKeyFromObject(ops), current); err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}

		if isOpsRequestNonBlocking(current) {
			return nil
		}

		if current.Spec.TimeoutSeconds != nil && *current.Spec.TimeoutSeconds <= 1 {
			return nil
		}

		patchData := `{
			"spec": {
				"timeoutSeconds": 1
			}
		}`

		return c.Patch(ctx, current, client.RawPatch(types.MergePatchType, []byte(patchData)))
	})
}

package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/furutachiKurea/block-mechanica/internal/index"
	"github.com/furutachiKurea/block-mechanica/internal/model"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OpsRequest 配置项
var (
	opsTimeoutSecond      int32 = 24 * 60 * 60
	opsLifeAfterUnsuccess int32 = 1 * 60 * 60
	opsLifeAfterSucceed   int32 = 24 * 60 * 60

	defaultBackupDeletionPolicy string = "Retain"
)

const (
	preflightProceed preflightDecision = iota + 1 // 创建
	preflightSkip                                 // 跳过
)

type preflightDecision int

type preflightResult struct {
	Decision preflightDecision
}

// preflight 规定 OpsRequest 创建前/后的决策逻辑
type preflight interface {
	// decide 根据创建目标判断是否允许创建
	decide(ctx context.Context, c client.Client, ops *opv1alpha1.OpsRequest) (preflightResult, error)
}

// uniqueOps 检查是否存在处在非终态的同类型同目标的 OpsRequest，
// 确保不针对一次状态维护操作创建多个 OpsRequest，用于 controller
type uniqueOps struct{}

func (uniqueOps) decide(ctx context.Context, c client.Client, ops *opv1alpha1.OpsRequest) (preflightResult, error) {
	opsList, err := getOpsRequestsByIndex(ctx, c, ops.Namespace, ops.Spec.ClusterName, ops.Spec.Type)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return preflightResult{}, fmt.Errorf("list opsrequests for preflight: %w", err)
		}
		return preflightResult{Decision: preflightProceed}, nil
	}

	for _, ops := range opsList {
		if !isOpsRequestInFinalPhase(&ops) {
			return preflightResult{Decision: preflightSkip}, nil
		}
	}

	return preflightResult{Decision: preflightProceed}, nil
}

type createOpts struct {
	preflight preflight
}

type createOption func(*createOpts)

// withPreflight 自定义预检策略
func withPreflight(p preflight) createOption {
	return func(o *createOpts) { o.preflight = p }
}

// createLifecycleOpsRequest 创建生命周期管理相关的 OpsRequest，供 Reconciler 使用
func createLifecycleOpsRequest(ctx context.Context,
	c client.Client,
	cluster *kbappsv1.Cluster,
	opsType opv1alpha1.OpsType,
) error {
	opsSpecific := opv1alpha1.SpecificOpsRequest{}

	if err := createOpsRequest(ctx, c, cluster, opsType, opsSpecific, withPreflight(uniqueOps{})); err != nil {
		return err
	}

	return nil
}

// createBackupOpsRequest 为指定的 Cluster 创建备份 OpsRequest
//
// backupMethod 为备份方法，取决于数据库类型
func createBackupOpsRequest(ctx context.Context,
	c client.Client,
	cluster *kbappsv1.Cluster,
	backupMethod string,
) error {

	specificOps := opv1alpha1.SpecificOpsRequest{
		Backup: &opv1alpha1.Backup{
			BackupPolicyName: fmt.Sprintf("%s-%s-backup-policy", cluster.Name, clusterType(cluster)),
			BackupMethod:     backupMethod,
			DeletionPolicy:   defaultBackupDeletionPolicy,
		},
	}

	return createOpsRequest(ctx, c, cluster, opv1alpha1.BackupType, specificOps)
}

// createHorizontalScalingOpsRequest 为指定的 Cluster 创建水平伸缩 OpsRequest
func createHorizontalScalingOpsRequest(ctx context.Context,
	c client.Client,
	params model.HorizontalScalingOpsParams,
) error {
	var specificOps opv1alpha1.SpecificOpsRequest

	if params.DeltaReplicas > 0 {
		// ScaleOut
		specificOps = opv1alpha1.SpecificOpsRequest{
			HorizontalScalingList: []opv1alpha1.HorizontalScaling{
				{
					ComponentOps: opv1alpha1.ComponentOps{ComponentName: params.ComponentName},
					ScaleOut: &opv1alpha1.ScaleOut{
						ReplicaChanger: opv1alpha1.ReplicaChanger{ReplicaChanges: &params.DeltaReplicas},
					},
				},
			},
		}
	} else {
		// ScaleIn
		absReplicas := -params.DeltaReplicas
		specificOps = opv1alpha1.SpecificOpsRequest{
			HorizontalScalingList: []opv1alpha1.HorizontalScaling{
				{
					ComponentOps: opv1alpha1.ComponentOps{ComponentName: params.ComponentName},
					ScaleIn: &opv1alpha1.ScaleIn{
						ReplicaChanger: opv1alpha1.ReplicaChanger{ReplicaChanges: &absReplicas},
					},
				},
			},
		}
	}

	return createOpsRequest(ctx, c, params.Cluster, opv1alpha1.HorizontalScalingType, specificOps)
}

// createVerticalScalingOpsRequest 为指定的 Cluster 创建垂直伸缩 OpsRequest
func createVerticalScalingOpsRequest(ctx context.Context,
	c client.Client,
	params model.VerticalScalingOpsParams,
) error {
	specificOps := opv1alpha1.SpecificOpsRequest{
		VerticalScalingList: []opv1alpha1.VerticalScaling{
			{
				ComponentOps: opv1alpha1.ComponentOps{ComponentName: params.ComponentName},
				ResourceRequirements: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    params.CPU,
						corev1.ResourceMemory: params.Memory,
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    params.CPU,
						corev1.ResourceMemory: params.Memory,
					},
				},
			},
		},
	}

	return createOpsRequest(ctx, c, params.Cluster, opv1alpha1.VerticalScalingType, specificOps)
}

// createVolumeExpansionOpsRequest 为指定的 Cluster 创建存储扩容 OpsRequest
func createVolumeExpansionOpsRequest(ctx context.Context,
	c client.Client,
	params model.VolumeExpansionOpsParams,
) error {
	specificOps := opv1alpha1.SpecificOpsRequest{
		VolumeExpansionList: []opv1alpha1.VolumeExpansion{
			{
				ComponentOps: opv1alpha1.ComponentOps{ComponentName: params.ComponentName},
				VolumeClaimTemplates: []opv1alpha1.OpsRequestVolumeClaimTemplate{
					{
						Name:    params.VolumeClaimTemplateName,
						Storage: params.Storage,
					},
				},
			},
		},
	}

	return createOpsRequest(ctx, c, params.Cluster, opv1alpha1.VolumeExpansionType, specificOps)
}

// createOpsRequest 创建 OpsRequest
//
// OpsRequest 的名称格式为 {clustername}-{opsType}-ops-{timestamp}，
// 使用时间戳确保每次操作都有唯一的名称
func createOpsRequest(
	ctx context.Context,
	c client.Client,
	cluster *kbappsv1.Cluster,
	opsType opv1alpha1.OpsType,
	specificOps opv1alpha1.SpecificOpsRequest,
	opts ...createOption,
) error {
	options := applyCreateOptions(opts...)

	ops := buildOpsRequest(cluster, opsType, specificOps)

	res, err := options.preflight.decide(ctx, c, ops)
	if err != nil {
		return fmt.Errorf("preflight check for ops %s failed: %w", ops.Name, err)
	}

	if res.Decision == preflightSkip {
		return ErrCreateOpsSkipped
	}

	if err := c.Create(ctx, ops); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return ErrCreateOpsSkipped
		}
		return fmt.Errorf("create opsrequest %s: %w", ops.Name, err)
	}

	return nil
}

// buildOpsRequest 构造 OpsRequest 对象
func buildOpsRequest(
	cluster *kbappsv1.Cluster,
	opsType opv1alpha1.OpsType,
	specificOps opv1alpha1.SpecificOpsRequest,
) *opv1alpha1.OpsRequest {
	name := makeOpsRequestName(cluster.Name, opsType)

	serviceID := cluster.GetLabels()[index.ServiceIDLabel]

	labels := map[string]string{
		constant.AppInstanceLabelKey:    cluster.Name,
		constant.OpsRequestTypeLabelKey: string(opsType),
		index.ServiceIDLabel:            serviceID,
	}

	ops := &opv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cluster.Namespace,
			Labels:    labels,
		},
		Spec: opv1alpha1.OpsRequestSpec{
			ClusterName:                           cluster.Name,
			Type:                                  opsType,
			TimeoutSeconds:                        &opsTimeoutSecond,
			TTLSecondsAfterUnsuccessfulCompletion: opsLifeAfterUnsuccess,
			TTLSecondsAfterSucceed:                opsLifeAfterSucceed,

			SpecificOpsRequest: specificOps,
		},
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
	applyDefaultCreateOptions(o)
	return o
}

func applyDefaultCreateOptions(o *createOpts) {
	if o.preflight == nil {
		o.preflight = uniqueOps{}
	}
}

// makeOpsRequestName 生成 OpsRequest 名称
// 格式：{clustername}-{opsType}-ops-{timestamp}
func makeOpsRequestName(clusterName string, opsType opv1alpha1.OpsType) string {
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s-%s-ops-%d", clusterName, strings.ToLower(string(opsType)), timestamp)
}

// getOpsRequestsByIndex 使用索引查询 OpsRequest，失败时回退到标签查询
func getOpsRequestsByIndex(ctx context.Context, c client.Client, namespace, clusterName string, opsType opv1alpha1.OpsType) ([]opv1alpha1.OpsRequest, error) {
	var list opv1alpha1.OpsRequestList

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

// isOpsRequestInFinalPhase 检查操作请求是否处于终态
func isOpsRequestInFinalPhase(ops *opv1alpha1.OpsRequest) bool {
	phase := ops.Status.Phase
	return phase == opv1alpha1.OpsSucceedPhase ||
		phase == opv1alpha1.OpsCancelledPhase ||
		phase == opv1alpha1.OpsFailedPhase ||
		phase == opv1alpha1.OpsAbortedPhase
}

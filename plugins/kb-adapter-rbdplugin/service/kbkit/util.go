package kbkit

import (
	"context"
	"fmt"

	"github.com/furutachiKurea/block-mechanica/internal/index"
	"github.com/furutachiKurea/block-mechanica/service/registry"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetClusterByServiceID 通过 service_id 获取对应的 KubeBlocks Cluster
// 优先使用 MatchingFields 索引查询，失败时回退到 MatchingLabels
func GetClusterByServiceID(ctx context.Context, c client.Client, serviceID string) (*kbappsv1.Cluster, error) {
	var list kbappsv1.ClusterList

	// 使用 index
	if err := c.List(ctx, &list, client.MatchingFields{index.ServiceIDField: serviceID}); err == nil {
		switch len(list.Items) {
		case 0:
			return nil, ErrTargetNotFound
		case 1:
			return &list.Items[0], nil
		default:
			return nil, ErrMultipleFounded
		}
	}

	// 回退到 MatchingLabels
	list = kbappsv1.ClusterList{}
	if err := c.List(ctx, &list, client.MatchingLabels{index.ServiceIDLabel: serviceID}); err != nil {
		return nil, fmt.Errorf("list clusters by service_id %s: %w", serviceID, err)
	}

	switch len(list.Items) {
	case 0:
		return nil, ErrTargetNotFound
	case 1:
		return &list.Items[0], nil
	default:
		return nil, ErrMultipleFounded
	}
}

// Paginate 分页, 从 items 中提取指定页的数据
func Paginate[T any](items []T, page, pageSize int) []T {
	if page < 1 || pageSize < 1 || len(items) == 0 {
		return nil
	}

	offset := (page - 1) * pageSize
	if offset >= len(items) {
		return nil
	}

	end := min(offset+pageSize, len(items))
	return items[offset:end:end]
}

// ClusterType 获取 Cluster 对应的数据库类型
func ClusterType(cluster *kbappsv1.Cluster) string {
	if cluster == nil {
		return ""
	}
	return cluster.Spec.ClusterDef
}

// IsSupportBackup 判定给定的数据库类型是否支持备份
func IsSupportBackup(addon string) bool {
	adapter, ok := registry.Cluster[addon]
	if !ok {
		return false
	}
	return adapter.Coordinator.GetBackupMethod() != ""
}

func IsSupportParameter(addon string) bool {
	adapter, ok := registry.Cluster[addon]
	if !ok {
		return false
	}
	return adapter.Coordinator.GetParametersConfigMap("not support") != nil
}

// GetAllOpsRequestsByCluster 获取指定集群的所有 OpsRequest
// 包括所有状态的 OpsRequest，用于彻底清理资源或审计目的
func GetAllOpsRequestsByCluster(ctx context.Context, c client.Client, namespace, clusterName string) ([]opsv1alpha1.OpsRequest, error) {
	var list opsv1alpha1.OpsRequestList

	if err := c.List(ctx, &list,
		client.InNamespace(namespace),
		client.MatchingLabels(map[string]string{
			constant.AppInstanceLabelKey: clusterName,
		}),
	); err != nil {
		return nil, fmt.Errorf("list all opsrequests for cluster %s/%s: %w", namespace, clusterName, err)
	}

	return list.Items, nil
}

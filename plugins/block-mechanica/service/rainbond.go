package service

import (
	"context"
	"fmt"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/furutachiKurea/block-mechanica/internal/index"
	"github.com/furutachiKurea/block-mechanica/internal/model"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// KubeBlocksComponentInfo 包含 KubeBlocks Component 的详细信息
type KubeBlocksComponentInfo struct {
	IsKubeBlocksComponent bool   `json:"isKubeBlocksComponent"`
	DatabaseType          string `json:"databaseType,omitempty"`
}

// RainbondService 提供 Rainbond 相关资源与 BlockMechanica 关联操作
type RainbondService struct {
	client client.Client
}

func NewRainbondService(c client.Client) *RainbondService {
	return &RainbondService{client: c}
}

// CheckKubeBlocksComponent 依据 ServiceIdentifier 判定该 Rainbond 组件是否为 KubeBlocks Component，如果是，则还返回 KubeBlocks Component 对应的 Cluster 的数据库类型
//
// 如果给定的 req.ServiceIdentifier.ID 能够匹配到一个 KubeBlocks Cluster，则说明该 Rainbond 组件为 KubeBlocks Component
func (s *RainbondService) CheckKubeBlocksComponent(ctx context.Context, rbd model.RBDService) (*KubeBlocksComponentInfo, error) {
	cluster, err := getClusterByServiceID(ctx, s.client, rbd.ServiceID)
	info := &KubeBlocksComponentInfo{IsKubeBlocksComponent: err == nil}
	if err == nil {
		info.DatabaseType = cluster.Spec.ClusterDef
	}

	return info, nil
}

// GetClusterByServiceID 通过 service_id 获取对应的 KubeBlocks Cluster
//
// 封装 GetClusterByServiceID 方法
func (s *RainbondService) GetClusterByServiceID(ctx context.Context, serviceID string) (*kbappsv1.Cluster, error) {
	return getClusterByServiceID(ctx, s.client, serviceID)
}

// GetKubeBlocksComponentByServiceID 通过 service_id 获取对应的 KubeBlocks Component（Rainbond 侧的 Deployment）
//
// 封装 getComponentByServiceID 方法
func (s *RainbondService) GetKubeBlocksComponentByServiceID(ctx context.Context, serviceID string) (*appsv1.Deployment, error) {
	return getComponentByServiceID(ctx, s.client, serviceID)
}

// GetClusterPort 返回指定数据库在 KubeBlocks service 中的目标端口
func (s *RainbondService) GetClusterPort(ctx context.Context, serviceID string) int {
	cluster, err := getClusterByServiceID(ctx, s.client, serviceID)
	if err != nil {
		return -1
	}
	adapter, ok := _clusterRegistry[cluster.Spec.ClusterDef]
	if !ok {
		return -1
	}
	return adapter.Coordinator.TargetPort()
}

// getClusterByServiceID 通过 service_id 获取对应的 KubeBlocks Cluster
//
// 优先 MatchingFields，失败回退到 MatchingLabels
func getClusterByServiceID(ctx context.Context, c client.Client, serviceID string) (*kbappsv1.Cluster, error) {
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

// getComponentByServiceID 通过 service_id 获取对应的 KubeBlocks Component（Rainbond 侧的 Deployment）
//
// 优先使用 MatchingFields，失败回退到 MatchingLabels
func getComponentByServiceID(ctx context.Context, c client.Client, serviceID string) (*appsv1.Deployment, error) {
	var list appsv1.DeploymentList

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

	list = appsv1.DeploymentList{}
	if err := c.List(ctx, &list, client.MatchingLabels{index.ServiceIDLabel: serviceID}); err != nil {
		return nil, fmt.Errorf("list deployments by service_id %s: %w", serviceID, err)
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

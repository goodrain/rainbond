// Package adapter 提供 KubeBlocks 的适配器实现
package adapter

import (
	"context"
	"errors"
	"fmt"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/furutachiKurea/block-mechanica/internal/model"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrAdapterNotImplemented = errors.New("adapter not implemented")
)

// ClusterBuilder 用于在 Rainbond 中 KubeBlocks Cluster 的创建
type ClusterBuilder interface {
	// BuildCluster 构建 Cluster struct
	BuildCluster(ctx context.Context, req model.ClusterInput) (*kbappsv1.Cluster, error)

	// AssociateToKubeBlocksComponent 将 KubeBlocks 组件和 Cluster 通过 service_id 关联
	AssociateToKubeBlocksComponent(ctx context.Context, c client.Client, req model.ClusterInput) error
}

// Coordinator 用于协调 KubeBlocks 和 Rainbond
type Coordinator interface {
	// TargetPort 返回 KubeBlocks Cluster 的连接端口，
	// 用于配置 KubeBlocksComponent 将连接转发至 Cluster 的 service
	TargetPort() int

	// GetSecretName 返回该数据库类型的 Secret 命名格式
	GetSecretName(clusterName string) string
}

// Backuper 提供数据库类型特化的备份配置
type Backuper interface {
	// GetBackupMethod 返回该数据库类型支持的备份方法
	GetBackupMethod() string
}

// ClusterAdapter
//
// 必须实现的字段：
//
// - Builder
//
// - Coordinator
//
// - Backup
type ClusterAdapter struct {
	Builder     ClusterBuilder
	Coordinator Coordinator
	Backup      Backuper
}

// Validate 验证 ClusterAdapter 的完整性，
// 确保所有必须实现的接口字段都被正确设置
func (ca *ClusterAdapter) Validate() error {
	if ca.Builder == nil {
		return fmt.Errorf("ClusterBuilder: %w", ErrAdapterNotImplemented)
	}

	if ca.Coordinator == nil {
		return fmt.Errorf("Coordinator: %w", ErrAdapterNotImplemented)
	}

	if ca.Backup == nil {
		return fmt.Errorf("BackupConfig: %w", ErrAdapterNotImplemented)
	}

	return nil
}

// Package adapter 提供 KubeBlocks 的适配器实现
package adapter

import (
	"errors"
	"fmt"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/furutachiKurea/block-mechanica/internal/model"
)

var (
	ErrAdapterNotImplemented = errors.New("adapter not implemented")
)

// ClusterBuilder 用于在 Rainbond 中 KubeBlocks Cluster 的创建
type ClusterBuilder interface {
	// BuildCluster 构建 Cluster struct
	BuildCluster(input model.ClusterInput) (*kbappsv1.Cluster, error)
}

// Coordinator 用于协调 KubeBlocks 和 Rainbond
type Coordinator interface {
	// TargetPort 返回 KubeBlocks Cluster 的连接端口，
	// 用于配置 KubeBlocksComponent 将连接转发至 Cluster 的 service
	TargetPort() int

	// GetSecretName 返回该数据库类型的 Secret 命名格式
	GetSecretName(clusterName string) string

	// GetBackupMethod 返回该数据库类型支持的备份方法
	GetBackupMethod() string

	// GetParametersConfigMap 返回该类型的 Cluster 用于储存参数配置的 ConfigMap 名称，
	// 并非所有的数据库类型都支持参数配置，不支持则返回 nil
	GetParametersConfigMap(clusterName string) *string

	// ParseParameters 解析 ConfigMap 中的配置文件参数
	// configData 为 ConfigMap 的 data 字段，包含各种配置文件内容
	ParseParameters(configData map[string]string) ([]model.ParameterEntry, error)

	// SystemAccount 返回该数据库类型在使用 custom secret 时使用的 systemAccount.name 和数据库账户名称,
	// 返回 nil 则表示不启用 custom secret
	SystemAccount() *string
}

// ClusterAdapter
//
// 必须实现的字段：
//
// - Builder
//
// - Coordinator
type ClusterAdapter struct {
	Builder     ClusterBuilder
	Coordinator Coordinator
}

// Validate 验证 ClusterAdapter 的完整性，
// 确保所有必须实现的接口字段都被正确设置
func (ca *ClusterAdapter) Validate() error {
	if ca.Builder == nil {
		return fmt.Errorf("ClusterBuilder: %w", ErrAdapterNotImplemented)
	}

	if ca.Coordinator == nil {
		return fmt.Errorf("coordinator: %w", ErrAdapterNotImplemented)
	}

	return nil
}

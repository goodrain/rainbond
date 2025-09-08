// Package coordinator 提供 adapter.Coordinator 的实现
//
// Coordinator 用于协调 KubeBlocks 和 Rainbond
package coordinator

import (
	"fmt"

	"github.com/furutachiKurea/block-mechanica/service/adapter"
)

var _ adapter.Coordinator = &Base{}

// Base 实现 Coordinator 接口，所有的 Coordinator 都应基于 Base 实现
type Base struct {
}

func (c *Base) TargetPort() int {
	return -1
}

func (c *Base) GetSecretName(clusterName string) string {
	// Base 实现使用通用的 root 账户格式，但实际不应被直接使用
	// 每个具体的 Coordinator 都应该重写此方法
	return fmt.Sprintf("%s-account-root", clusterName)
}

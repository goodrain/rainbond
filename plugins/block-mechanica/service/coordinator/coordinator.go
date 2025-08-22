// Package coordinator 提供 adapter.Coordinator 的实现
//
// Coordinator 用于协调 KubeBlocks 和 Rainbond
package coordinator

import "github.com/furutachiKurea/block-mechanica/service/adapter"

var _ adapter.Coordinator = &Base{}

// Base 实现 Coordinator 接口，所有的 Coordinator 都应基于 Base 实现
type Base struct {
}

func (c *Base) TargetPort() int {
	return -1
}

package coordinator

import "github.com/furutachiKurea/block-mechanica/service/adapter"

var _ adapter.Coordinator = &MySQLCoordinator{}

// MySQLCoordinator 实现 Coordinator 接口
type MySQLCoordinator struct {
	Base
}

func (c *MySQLCoordinator) TargetPort() int {
	return 3306
}

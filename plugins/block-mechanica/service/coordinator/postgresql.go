package coordinator

import "github.com/furutachiKurea/block-mechanica/service/adapter"

var _ adapter.Coordinator = &PostgreSQLCoordinator{}

// PostgreSQLCoordinator 实现 Coordinator 接口
type PostgreSQLCoordinator struct {
	Base
}

func (c *PostgreSQLCoordinator) TargetPort() int {
	return 5432
}

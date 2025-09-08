package coordinator

import (
	"fmt"

	"github.com/furutachiKurea/block-mechanica/service/adapter"
)

var _ adapter.Coordinator = &PostgreSQLCoordinator{}

// PostgreSQLCoordinator 实现 Coordinator 接口
type PostgreSQLCoordinator struct {
	Base
}

func (c *PostgreSQLCoordinator) TargetPort() int {
	return 6432
}

func (c *PostgreSQLCoordinator) GetSecretName(clusterName string) string {
	// PostgreSQL 使用 postgresql 作为中间部分和 postgres 作为账户类型
	return fmt.Sprintf("%s-postgresql-account-postgres", clusterName)
}

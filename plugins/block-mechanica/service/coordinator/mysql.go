package coordinator

import (
	"fmt"

	"github.com/furutachiKurea/block-mechanica/service/adapter"
)

var _ adapter.Coordinator = &MySQLCoordinator{}

// MySQLCoordinator 实现 Coordinator 接口
type MySQLCoordinator struct {
	Base
}

func (c *MySQLCoordinator) TargetPort() int {
	return 3306
}

func (c *MySQLCoordinator) GetSecretName(clusterName string) string {
	// MySQL 使用 mysql 作为中间部分和 root 作为账户类型
	return fmt.Sprintf("%s-mysql-account-root", clusterName)
}

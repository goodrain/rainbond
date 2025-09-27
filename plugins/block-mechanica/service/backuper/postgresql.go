package backuper

import (
	"github.com/furutachiKurea/block-mechanica/service/adapter"
)

var _ adapter.Backuper = &PostgreSQLBackuper{}

// PostgreSQLBackuper 实现 PostgreSQL 的备份配置
type PostgreSQLBackuper struct {
	BaseBackuper
}

// GetBackupMethod 返回 PostgreSQL 的备份方法
func (b PostgreSQLBackuper) GetBackupMethod() string {
	return "pg-basebackup"
}

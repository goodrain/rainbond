package backuper

import (
	"github.com/furutachiKurea/block-mechanica/service/adapter"
)

var _ adapter.Backuper = &MySQLBackuper{}

// MySQLBackuper 实现 MySQL 的备份配置
type MySQLBackuper struct {
	BaseBackuper
}

// GetBackupMethod 返回 MySQL 的备份方法
func (b MySQLBackuper) GetBackupMethod() string {
	return "xtrabackup"
}

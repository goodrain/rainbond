// Package backuper 提供数据库备份配置的实现
//
// 参考 builder 包的结构，为每种数据库类型提供特化的备份配置
package backuper

import (
	"github.com/furutachiKurea/block-mechanica/service/adapter"
)

var _ adapter.Backuper = &BaseBackuper{}

// BaseBackuper 实现 BackupConfig 接口的基础结构
type BaseBackuper struct{}

// GetBackupMethod 不会被用到
func (b BaseBackuper) GetBackupMethod() string {
	return "default"
}

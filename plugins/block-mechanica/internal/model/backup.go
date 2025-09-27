package model

// BackupScheduleInput 用于更新备份设置
type BackupScheduleInput struct {
	RBDService
	ClusterBackup
}

// BackupInput 用于创建备份
type BackupInput struct {
	RBDService
}

// BackupListQuerry 用于获取备份列表
type BackupListQuerry struct {
	RBDService
}

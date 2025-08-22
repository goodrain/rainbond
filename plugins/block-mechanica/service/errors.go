package service

import "errors"

var (
	// ErrTargetNotFound 目标资源不存在
	ErrTargetNotFound = errors.New("resource not found")

	// ErrMultipleFounded 表示存在多个同类型资源
	ErrMultipleFounded = errors.New("multiple resources found")

	// ErrCreateOpsSkipped 表示创建 OpsRequest 因预检检查而被跳过
	ErrCreateOpsSkipped = errors.New("operation skipped by preflight check")
)

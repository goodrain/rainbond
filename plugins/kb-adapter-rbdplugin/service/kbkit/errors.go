package kbkit

import (
	"errors"
	"fmt"
)

var (
	// ErrTargetNotFound 目标资源不存在
	ErrTargetNotFound = errors.New("resource not found")

	// ErrMultipleFounded 表示存在多个同类型资源
	ErrMultipleFounded = errors.New("multiple resources found")

	// ErrCreateOpsSkipped 表示创建 OpsRequest 因预检检查而被跳过
	ErrCreateOpsSkipped = errors.New("operation skipped by preflight check")

	// ErrClusterRequired 表示集群信息缺失
	ErrClusterRequired = errors.New("cluster is required")
)

// 参数变更操作中错误常量
const (
	// ParamNotExist 参数在集群定义中不存在
	ParamNotExist ParameterErrCode = "NOT_EXIST"

	// ParamImmutable 参数不可变，无法进行热更新
	ParamImmutable ParameterErrCode = "IMMUTABLE"

	// ParamInvalidType 参数类型校验失败
	ParamInvalidType ParameterErrCode = "INVALID_TYPE"

	// ParamOutOfRange 数值参数超出允许范围
	ParamOutOfRange ParameterErrCode = "OUT_OF_RANGE"

	// ParamInvalidEnum 枚举参数值不在允许列表中
	ParamInvalidEnum ParameterErrCode = "INVALID_ENUM"

	// ParamRequiredMissing 必需参数缺失
	ParamRequiredMissing ParameterErrCode = "REQUIRED_MISSING"
)

type ParameterErrCode string

// ParameterValidationError 参数验证错误的结构化表示
// 提供详细的错误信息和标准化的错误码
type ParameterValidationError struct {
	ParameterName string           // 参数名称
	ErrorCode     ParameterErrCode // 用于向 Rainbond 展示的错误码
	ErrorMessage  string           // 详细错误信息
	Cause         error            // 底层错误
}

// Error -
func (e *ParameterValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.ErrorMessage, e.Cause)
	}
	return e.ErrorMessage
}

// Is supports errors.Is
func (e *ParameterValidationError) Is(target error) bool {
	var t *ParameterValidationError
	if errors.As(target, &t) {
		return e.ErrorCode == t.ErrorCode && e.ParameterName == t.ParameterName
	}
	return false
}

// Unwrap supports errors.Unwrap
func (e *ParameterValidationError) Unwrap() error {
	return e.Cause
}

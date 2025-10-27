package kbkit

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/furutachiKurea/block-mechanica/internal/log"
	"github.com/furutachiKurea/block-mechanica/internal/model"
)

// ParameterValidator 参数验证器
// 负责集中处理参数验证逻辑，支持类型校验、范围校验、枚举校验等
type ParameterValidator struct {
	constraints map[string]model.Parameter
}

// NewParameterValidator -
func NewParameterValidator(constraints []model.Parameter) *ParameterValidator {
	constraintMap := make(map[string]model.Parameter, len(constraints))
	for _, param := range constraints {
		constraintMap[param.Name] = param
	}
	return &ParameterValidator{constraints: constraintMap}
}

// Validate 验证单个参数变更请求
// 返回 nil 表示验证通过，否则返回具体的验证错误信息
func (v *ParameterValidator) Validate(entry model.ParameterEntry) *ParameterValidationError {
	// 检查参数是否存在
	constraint, exists := v.constraints[entry.Name]
	if !exists {
		log.Error("parameter not Exist", log.String("parameter_name", entry.Name), log.Any("constraints", v.constraints[entry.Name]))
		return &ParameterValidationError{
			ParameterName: entry.Name,
			ErrorCode:     ParamNotExist,
			ErrorMessage:  fmt.Sprintf("parameter '%s' not found in cluster definition", entry.Name),
		}
	}

	// 检查参数可变性：仅当明确为不可变时拒绝
	// 当允许用户手动添加参数时才会生效，否则现有设计下不会出现 IsImmutable 为 true 的参数
	if constraint.IsImmutable {
		return &ParameterValidationError{
			ParameterName: entry.Name,
			ErrorCode:     ParamImmutable,
			ErrorMessage:  fmt.Sprintf("parameter '%s' is immutable and cannot be changed", entry.Name),
		}
	}

	// 检查必需参数
	if constraint.IsRequired && entry.Value == nil {
		return &ParameterValidationError{
			ParameterName: entry.Name,
			ErrorCode:     ParamRequiredMissing,
			ErrorMessage:  fmt.Sprintf("parameter '%s' is required and cannot be empty", entry.Name),
		}
	}

	// 如果没有类型信息（Type 为空），说明参数只在列表中声明但没有 schema 定义
	// 跳过类型、范围、枚举校验，只进行 immutable 检查
	if constraint.Type == "" {
		log.Debug("parameter has no schema definition, skipping type/range/enum validation",
			log.String("parameter", entry.Name))
		return nil
	}

	// 验证参数类型
	if err := v.validateType(constraint, entry.Value); err != nil {
		return err
	}

	// 验证数值范围
	if err := v.validateRange(constraint, entry.Value); err != nil {
		log.Error("value out of range", log.Err(err))
		return err
	}

	// 验证枚举值
	if err := v.validateEnum(constraint, entry.Value); err != nil {
		log.Error("invalid enum value", log.Err(err))
		return err
	}

	return nil
}

// validateType 验证参数类型
func (v *ParameterValidator) validateType(param model.Parameter, value any) *ParameterValidationError {
	if value == nil {
		return nil
	}

	switch param.Type {
	case model.ParameterTypeString:
		if _, ok := value.(string); !ok {
			return &ParameterValidationError{
				ParameterName: param.Name,
				ErrorCode:     ParamInvalidType,
				ErrorMessage:  fmt.Sprintf("parameter '%s' expects string type, got %T", param.Name, value),
			}
		}

	case model.ParameterTypeInteger, "int32", "int64":
		switch v := value.(type) {
		case int, int32, int64, uint64, float64:
			// 数值类型直接通过
		case string:
			// 首先尝试解析为有符号整数
			if _, err := strconv.ParseInt(v, 10, 64); err != nil {
				// 如果有符号整数解析失败，尝试无符号整数
				if _, err2 := strconv.ParseUint(v, 10, 64); err2 != nil {
					return &ParameterValidationError{
						ParameterName: param.Name,
						ErrorCode:     ParamInvalidType,
						ErrorMessage:  fmt.Sprintf("parameter '%s' cannot parse '%s' as integer", param.Name, v),
						Cause:         err,
					}
				}
			}
		default:
			return &ParameterValidationError{
				ParameterName: param.Name,
				ErrorCode:     ParamInvalidType,
				ErrorMessage:  fmt.Sprintf("parameter '%s' expects integer type, got %T", param.Name, value),
			}
		}

	case model.ParameterTypeNumber:
		switch v := value.(type) {
		case int, int32, int64, float32, float64:
			// 数值类型直接通过
		case string:
			// 尝试解析字符串为浮点数
			if _, err := strconv.ParseFloat(v, 64); err != nil {
				return &ParameterValidationError{
					ParameterName: param.Name,
					ErrorCode:     ParamInvalidType,
					ErrorMessage:  fmt.Sprintf("parameter '%s' cannot parse '%s' as number", param.Name, v),
					Cause:         err,
				}
			}
		default:
			return &ParameterValidationError{
				ParameterName: param.Name,
				ErrorCode:     ParamInvalidType,
				ErrorMessage:  fmt.Sprintf("parameter '%s' expects number type, got %T", param.Name, value),
			}
		}

	case model.ParameterTypeBoolean:
		switch v := value.(type) {
		case bool:
			// 布尔类型直接通过
		case string:
			// 尝试解析字符串为布尔值
			if _, err := strconv.ParseBool(v); err != nil {
				return &ParameterValidationError{
					ParameterName: param.Name,
					ErrorCode:     ParamInvalidType,
					ErrorMessage:  fmt.Sprintf("parameter '%s' cannot parse '%s' as boolean", param.Name, v),
					Cause:         err,
				}
			}
		default:
			return &ParameterValidationError{
				ParameterName: param.Name,
				ErrorCode:     ParamInvalidType,
				ErrorMessage:  fmt.Sprintf("parameter '%s' expects boolean type, got %T", param.Name, value),
			}
		}
	}

	return nil
}

// validateRange 验证数值参数的范围约束
func (v *ParameterValidator) validateRange(param model.Parameter, value any) *ParameterValidationError {
	var maxVal, minVal any
	if param.MaxValue != nil {
		maxVal = *param.MaxValue
	}
	if param.MinValue != nil {
		minVal = *param.MinValue
	}
	log.Debug("validateRange",
		log.Any("maxValue", maxVal),
		log.Any("minValue", minVal),
		log.Any("value", value),
	)
	if value == nil || (param.MinValue == nil && param.MaxValue == nil) {
		return nil
	}

	// 将值转换为 float64 以便统一比较
	numValue, parseErr := v.convertToFloat64(value)
	if parseErr != nil {
		return nil
	}

	if err := v.validateMinValue(param, numValue); err != nil {
		return err
	}

	if err := v.validateMaxValue(param, numValue); err != nil {
		return err
	}

	return nil
}

// convertToFloat64 将各种数值类型转换为 float64
func (v *ParameterValidator) convertToFloat64(value any) (float64, error) {
	switch v := value.(type) {
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("unsupported type: %T", value)
	}
}

// validateMinValue 验证最小值约束
func (v *ParameterValidator) validateMinValue(param model.Parameter, numValue float64) *ParameterValidationError {
	if param.MinValue == nil {
		return nil
	}

	minVal := *param.MinValue
	if numValue < minVal {
		return &ParameterValidationError{
			ParameterName: param.Name,
			ErrorCode:     ParamOutOfRange,
			ErrorMessage:  fmt.Sprintf("parameter '%s' value %v is less than minimum %v", param.Name, numValue, minVal),
		}
	}
	return nil
}

// validateMaxValue 验证最大值约束
func (v *ParameterValidator) validateMaxValue(param model.Parameter, numValue float64) *ParameterValidationError {
	if param.MaxValue == nil {
		return nil
	}

	maxVal := *param.MaxValue
	if numValue > maxVal {
		return &ParameterValidationError{
			ParameterName: param.Name,
			ErrorCode:     ParamOutOfRange,
			ErrorMessage:  fmt.Sprintf("parameter '%s' value %v is greater than maximum %v", param.Name, numValue, maxVal),
		}
	}
	return nil
}

// validateEnum 验证枚举参数的有效性
func (v *ParameterValidator) validateEnum(param model.Parameter, value any) *ParameterValidationError {
	if value == nil || len(param.EnumValues) == 0 {
		return nil
	}

	// 将值转换为字符串进行比较
	var valueStr string
	switch v := value.(type) {
	case string:
		valueStr = v
	default:
		valueStr = fmt.Sprintf("%v", v)
	}

	// 检查是否在枚举列表中
	for _, enumValue := range param.EnumValues {
		// 去除 JSON 字符串的引号进行比较
		cleanEnum := strings.Trim(enumValue, "\"")
		if valueStr == cleanEnum || valueStr == enumValue {
			return nil
		}
	}

	return &ParameterValidationError{
		ParameterName: param.Name,
		ErrorCode:     ParamInvalidEnum,
		ErrorMessage:  fmt.Sprintf("parameter '%s' value '%s' is not in allowed values: %v", param.Name, valueStr, param.EnumValues),
	}
}

// ConvertToStringValue 将验证通过的值转换为 *string 格式，供 OpsRequest 使用
func (v *ParameterValidator) ConvertToStringValue(value any) *string {
	if value == nil {
		return nil
	}

	var strValue string
	switch v := value.(type) {
	case string:
		strValue = v
	case bool:
		strValue = strconv.FormatBool(v)
	case int:
		strValue = strconv.Itoa(v)
	case int32:
		strValue = strconv.FormatInt(int64(v), 10)
	case int64:
		strValue = strconv.FormatInt(v, 10)
	case uint64:
		strValue = strconv.FormatUint(v, 10)
	case float32:
		strValue = strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		strValue = strconv.FormatFloat(v, 'f', -1, 64)
	default:
		strValue = fmt.Sprintf("%v", v)
	}

	return &strValue
}

package model

const (
	ParameterTypeString  ParameterType = "string"
	ParameterTypeInteger ParameterType = "integer"
	ParameterTypeNumber  ParameterType = "number"
	ParameterTypeBoolean ParameterType = "boolean"
)

type ParameterType string

type StorageClasses []string

// Addon 表示 KubeBlocks 支持的数据库类型及其版本
type Addon struct {
	Type            string   `json:"type"`
	Version         []string `json:"version"`
	IsSupportBackup bool     `json:"support_backup"`
}

// ParameterSets 保存静态/动态/不可变参数集合。
type ParameterSets struct {
	Static    map[string]bool
	Dynamic   map[string]bool
	Immutable map[string]bool
}

// ParameterEntry 通过 configmap 获取到的实际被设置的 parameter
type ParameterEntry struct {
	Name  string `json:"name"`  // 参数名称
	Value any    `json:"value"` // Value 参数值
}

// Parameter 参数信息
//
// 标明参数的名称、值、数据类型、最小值、最大值、枚举值、描述、是否为动态参数、是否为必填项、是否为 immutable
//
// 字段 Value 需要遵守 Parameter 中的约束
//
// 在提供给 Rainbond 使用时，需要将来自 ParametersDefinition 的默认值使用实际的 ParameterEntry 覆盖
type Parameter struct {
	ParameterEntry
	Type        ParameterType `json:"type"`                   // Type 参数数据类型（受限集合）
	MinValue    *float64      `json:"min_value,omitempty"`    // MinValue 参数最小值, 仅数值类型有效，除此之外的为 nil
	MaxValue    *float64      `json:"max_value,omitempty"`    // MaxValue 参数最大值, 仅数值类型有效，除此之外的为 nil
	EnumValues  []string      `json:"enum_values,omitempty"`  // EnumValues 参数枚举值, 仅枚举类型有效，除此之外的为 nil
	Description string        `json:"description"`            // Description 参数描述
	IsDynamic   bool          `json:"is_dynamic"`             // IsDynamic 是否为动态参数, 动态参数支持热更新，静态参数需要重启数据库
	IsRequired  bool          `json:"is_required"`            // IsRequired 是否为必填参数
	IsImmutable bool          `json:"is_immutable,omitempty"` // IsImmutable 是否为不可变参数（只在内部使用/校验）
}

type ClusterParametersChange struct {
	RBDService
	Parameters []ParameterEntry `json:"changes"`
}

// ParameterChangeResult 参数变更结果
type ParameterChangeResult struct {
	Applied  []string               `json:"applied"`  // 成功应用的参数名称列表
	Invalids []ParameterChangeError `json:"invalids"` // 校验失败的参数
}

// ParameterChangeError 参数变更错误
type ParameterChangeError struct {
	Name string `json:"name"` // 参数名称
	Code string `json:"code"` // 错误码
}

type ClusterParametersQuery struct {
	RBDService
	Pagination
	Search
}

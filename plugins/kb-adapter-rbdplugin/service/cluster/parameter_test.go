package cluster

import (
	"testing"

	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/model"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/testutil"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestMergeEntriesAndConstraints(t *testing.T) {
	tests := []struct {
		name        string
		entries     []model.ParameterEntry
		constraints map[string]model.Parameter
		expected    []model.Parameter
		description string
	}{
		{
			name:        "empty_entries_empty_constraints",
			entries:     []model.ParameterEntry{},
			constraints: map[string]model.Parameter{},
			expected:    []model.Parameter{},
			description: "两个输入都为空应返回空结果",
		},
		{
			name:    "empty_entries_has_constraints",
			entries: []model.ParameterEntry{},
			constraints: map[string]model.Parameter{
				"max_connections": testutil.NewParameterConstraint("max_connections").WithType(model.ParameterTypeInteger).Build(),
			},
			expected:    []model.Parameter{},
			description: "entries为空时应返回空结果，即使有约束定义",
		},
		{
			name: "has_entries_empty_constraints",
			entries: []model.ParameterEntry{
				testutil.NewParameterEntry("max_connections", 100),
			},
			constraints: map[string]model.Parameter{},
			expected:    []model.Parameter{},
			description: "constraints为空时应过滤掉所有entries",
		},
		{
			name: "entries_filtered_by_constraints",
			entries: []model.ParameterEntry{
				testutil.NewParameterEntry("max_connections", 100),
				testutil.NewParameterEntry("unknown_param", "should_be_filtered"),
				testutil.NewParameterEntry("another_unknown", 123),
			},
			constraints: map[string]model.Parameter{
				"max_connections": testutil.NewParameterConstraint("max_connections").WithType(model.ParameterTypeInteger).Build(),
			},
			expected: []model.Parameter{
				{
					ParameterEntry: testutil.NewParameterEntry("max_connections", 100),
					Type:           model.ParameterTypeInteger,
				},
			},
			description: "只保留在constraints中定义的参数，过滤未知参数",
		},
		{
			name: "no_type_inference_when_constraint_has_type",
			entries: []model.ParameterEntry{
				testutil.NewParameterEntry("max_connections", "100"),
			},
			constraints: map[string]model.Parameter{
				"max_connections": testutil.NewParameterConstraint("max_connections").WithType(model.ParameterTypeInteger).Build(),
			},
			expected: []model.Parameter{
				{
					ParameterEntry: testutil.NewParameterEntry("max_connections", "100"),
					Type:           model.ParameterTypeInteger, // 直接使用constraint中的类型
				},
			},
			description: "constraints有类型信息时，直接使用不进行推断",
		},
		{
			name: "complete_constraint_information_transfer",
			entries: []model.ParameterEntry{
				testutil.NewParameterEntry("max_connections", 100),
			},
			constraints: map[string]model.Parameter{
				"max_connections": testutil.NewParameterConstraint("max_connections").
					WithType(model.ParameterTypeInteger).
					WithRange(ptr.To(1.0), ptr.To(100000.0)).
					WithDynamic(true).
					Build(),
			},
			expected: []model.Parameter{
				{
					ParameterEntry: testutil.NewParameterEntry("max_connections", 100),
					Type:           model.ParameterTypeInteger,
					MinValue:       ptr.To(1.0),
					MaxValue:       ptr.To(100000.0),
					IsDynamic:      true,
				},
			},
			description: "应完整传递约束信息：类型、描述、范围、动态标记",
		},
		{
			name: "nil_value_handling",
			entries: []model.ParameterEntry{
				testutil.NewParameterEntry("empty_param", nil),
			},
			constraints: map[string]model.Parameter{
				"empty_param": testutil.NewParameterConstraint("empty_param").WithType("").Build(),
			},
			expected: []model.Parameter{
				{
					ParameterEntry: testutil.NewParameterEntry("empty_param", nil),
					Type:           "", // nil值无法推断类型
				},
			},
			description: "nil值无法进行类型推断，Type应保持为空",
		},
		{
			name:        "realistic_mysql_scenario",
			entries:     testutil.CreateTypicalMySQLParameterEntries(),
			constraints: testutil.CreateTypicalMySQLParameterConstraints(),
			expected: []model.Parameter{
				{
					ParameterEntry: testutil.NewParameterEntry("max_connections", 100),
					Type:           model.ParameterTypeInteger,
					MinValue:       ptr.To(1.0),
					MaxValue:       ptr.To(100000.0),
					IsDynamic:      true,
				},
				{
					ParameterEntry: testutil.NewParameterEntry("innodb_buffer_pool_size", "128M"),
					Type:           model.ParameterTypeString,
					IsImmutable:    true,
				},
				{
					ParameterEntry: testutil.NewParameterEntry("sql_mode", "STRICT_TRANS_TABLES"),
					Type:           model.ParameterTypeString,
					EnumValues:     []string{`"STRICT_TRANS_TABLES"`, `"NO_ZERO_DATE"`},
					IsDynamic:      true,
				},
				{
					ParameterEntry: testutil.NewParameterEntry("autocommit", "ON"),
					Type:           model.ParameterTypeBoolean,
					IsDynamic:      true,
				},
				// query_cache_size 在 entries 中存在但不在 constraints 中，应被过滤
			},
			description: "真实MySQL参数场景：包含各种类型、约束和过滤逻辑",
		},
		{
			name: "type_inference_boundary_cases",
			entries: []model.ParameterEntry{
				testutil.NewParameterEntry("bool_on", "ON"),
				testutil.NewParameterEntry("bool_off", "OFF"),
				testutil.NewParameterEntry("bool_false", "false"),
				testutil.NewParameterEntry("negative_int", "-123"),
				testutil.NewParameterEntry("negative_float", "-45.67"),
				testutil.NewParameterEntry("zero", "0"),
				testutil.NewParameterEntry("empty_string", ""),
				testutil.NewParameterEntry("mixed_string", "abc123"),
			},
			constraints: func() map[string]model.Parameter {
				result := make(map[string]model.Parameter)
				for _, name := range []string{"bool_on", "bool_off", "bool_false", "negative_int", "negative_float", "zero", "empty_string", "mixed_string"} {
					result[name] = testutil.NewParameterConstraint(name).WithType("").Build()
				}
				return result
			}(),
			expected: []model.Parameter{
				{ParameterEntry: testutil.NewParameterEntry("bool_on", "ON"), Type: model.ParameterTypeBoolean},
				{ParameterEntry: testutil.NewParameterEntry("bool_off", "OFF"), Type: model.ParameterTypeBoolean},
				{ParameterEntry: testutil.NewParameterEntry("bool_false", "false"), Type: model.ParameterTypeBoolean},
				{ParameterEntry: testutil.NewParameterEntry("negative_int", "-123"), Type: model.ParameterTypeInteger},
				{ParameterEntry: testutil.NewParameterEntry("negative_float", "-45.67"), Type: model.ParameterTypeNumber},
				{ParameterEntry: testutil.NewParameterEntry("zero", "0"), Type: model.ParameterTypeInteger},
				{ParameterEntry: testutil.NewParameterEntry("empty_string", ""), Type: model.ParameterTypeString},
				{ParameterEntry: testutil.NewParameterEntry("mixed_string", "abc123"), Type: model.ParameterTypeString},
			},
			description: "类型推断的各种边界情况：布尔值变体、负数、零值、空字符串",
		},
		{
			name: "native_go_type_inference",
			entries: []model.ParameterEntry{
				testutil.NewParameterEntry("native_int", int(42)),
				testutil.NewParameterEntry("native_int32", int32(100)),
				testutil.NewParameterEntry("native_int64", int64(999)),
				testutil.NewParameterEntry("native_float32", float32(3.14)),
				testutil.NewParameterEntry("native_float64", float64(2.718)),
				testutil.NewParameterEntry("native_bool_true", true),
				testutil.NewParameterEntry("native_bool_false", false),
				testutil.NewParameterEntry("invalid_float", "1.2.3"),             // 包含多个小数点
				testutil.NewParameterEntry("slice_type", []string{"a"}),          // 非基础类型
				testutil.NewParameterEntry("map_type", map[string]int{"key": 1}), // 非基础类型
			},
			constraints: func() map[string]model.Parameter {
				result := make(map[string]model.Parameter)
				for _, name := range []string{"native_int", "native_int32", "native_int64", "native_float32", "native_float64", "native_bool_true", "native_bool_false", "invalid_float", "slice_type", "map_type"} {
					result[name] = testutil.NewParameterConstraint(name).WithType("").Build()
				}
				return result
			}(),
			expected: []model.Parameter{
				{ParameterEntry: testutil.NewParameterEntry("native_int", int(42)), Type: model.ParameterTypeInteger},
				{ParameterEntry: testutil.NewParameterEntry("native_int32", int32(100)), Type: model.ParameterTypeInteger},
				{ParameterEntry: testutil.NewParameterEntry("native_int64", int64(999)), Type: model.ParameterTypeInteger},
				{ParameterEntry: testutil.NewParameterEntry("native_float32", float32(3.14)), Type: model.ParameterTypeInteger},
				{ParameterEntry: testutil.NewParameterEntry("native_float64", float64(2.718)), Type: model.ParameterTypeInteger},
				{ParameterEntry: testutil.NewParameterEntry("native_bool_true", true), Type: model.ParameterTypeBoolean},
				{ParameterEntry: testutil.NewParameterEntry("native_bool_false", false), Type: model.ParameterTypeBoolean},
				{ParameterEntry: testutil.NewParameterEntry("invalid_float", "1.2.3"), Type: model.ParameterTypeString},             // 无效浮点数回退为string
				{ParameterEntry: testutil.NewParameterEntry("slice_type", []string{"a"}), Type: model.ParameterTypeString},          // 其他类型回退为string
				{ParameterEntry: testutil.NewParameterEntry("map_type", map[string]int{"key": 1}), Type: model.ParameterTypeString}, // 其他类型回退为string
			},
			description: "原生Go类型推断和边界情况：int、float、bool原生类型，以及无效值的处理",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeEntriesAndConstraints(tt.entries, tt.constraints)

			assert.ElementsMatch(t, tt.expected, result, tt.description)

			assert.Len(t, result, len(tt.expected), "结果数量应匹配期望")
		})
	}
}

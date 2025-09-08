// Package mono contains some useful utilities.
package mono

import (
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
)

// Filter 过滤切片
func Filter[T any](in []T, fn func(T) bool) []T {
	out := make([]T, 0, len(in))
	for _, v := range in {
		if fn(v) {
			out = append(out, v)
		}
	}
	return out
}

// Sorted 对字符串切片进行排序并返回新切片
// 确保返回确定性顺序，不修改原切片
func Sorted(slice []string) []string {
	result := make([]string, len(slice))
	copy(result, slice)
	sort.Strings(result)
	return result
}

// FilterThenSort 先过滤再排序，返回确定性顺序的结果
//
// 参数顺序：数据 -> 过滤条件 -> 排序条件
func FilterThenSort[T any](in []T, filterFn func(T) bool, lessFn func(T, T) bool) []T {
	filtered := Filter(in, filterFn)
	sort.Slice(filtered, func(i, j int) bool {
		return lessFn(filtered[i], filtered[j])
	})
	return filtered
}

// GetSecretField 从 Secret 中获取指定字段的值
func GetSecretField(secret *corev1.Secret, field string) (string, error) {
	data, exists := secret.Data[field]
	if !exists {
		return "", fmt.Errorf("field %s not found in secret %s/%s", field, secret.Namespace, secret.Name)
	}

	// 检查数据是否为空
	if len(data) == 0 {
		return "", fmt.Errorf("field %s is empty in secret %s/%s", field, secret.Namespace, secret.Name)
	}

	return string(data), nil
}

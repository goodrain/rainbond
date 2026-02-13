package handler

import (
	"os"
	"testing"
)

func TestSelectAvailablePort(t *testing.T) {
	// 设置环境变量
	os.Setenv("MIN_LB_PORT", "30000")
	os.Setenv("MAX_LB_PORT", "65535")

	tests := []struct {
		name     string
		used     []int
		expected int
	}{
		{
			name:     "空列表，返回最小端口",
			used:     []int{},
			expected: 30000,
		},
		{
			name:     "连续端口，返回下一个",
			used:     []int{30000, 30001, 30002},
			expected: 30003,
		},
		{
			name:     "有间隙，返回第一个空闲端口",
			used:     []int{30000, 30002, 30003},
			expected: 30001,
		},
		{
			name:     "大间隙，返回第一个空闲端口",
			used:     []int{30000, 32077},
			expected: 30001,
		},
		{
			name:     "乱序输入，返回第一个空闲端口",
			used:     []int{30002, 30000, 30003},
			expected: 30001,
		},
		{
			name:     "从中间开始有间隙",
			used:     []int{30000, 30001, 30002, 30005, 30006},
			expected: 30003,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selectAvailablePort(tt.used)
			if result != tt.expected {
				t.Errorf("selectAvailablePort(%v) = %d, expected %d", tt.used, result, tt.expected)
			}
		})
	}
}

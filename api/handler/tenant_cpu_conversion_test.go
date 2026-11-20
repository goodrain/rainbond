package handler

import "testing"

func TestConvertCPUPreservesMillicores(t *testing.T) {
	for _, tc := range []struct {
		input    int
		expected string
	}{{0, "0m"}, {500, "500m"}, {1000, "1"}, {1500, "1500m"}, {2500, "2500m"}, {4000, "4"}} {
		if got := ConvertCPU(tc.input); got != tc.expected {
			t.Errorf("ConvertCPU(%d) = %s, want %s", tc.input, got, tc.expected)
		}
	}
}

func TestConvertMemoryPreservesMiB(t *testing.T) {
	for _, tc := range []struct {
		input    int
		expected string
	}{{512, "512Mi"}, {1024, "1Gi"}, {1536, "1536Mi"}, {2048, "2Gi"}, {2560, "2560Mi"}} {
		if got := ConvertMemory(tc.input); got != tc.expected {
			t.Errorf("ConvertMemory(%d) = %s, want %s", tc.input, got, tc.expected)
		}
	}
}

package coordinator

import (
	"testing"
)

func TestBase_ParseParameters(t *testing.T) {
	testCases := []struct {
		name          string
		configData    map[string]string
		expectedLen   int
		expectedError bool
	}{
		{
			name:          "empty config data",
			configData:    map[string]string{},
			expectedLen:   0,
			expectedError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			base := &Coordinator{}
			params, err := base.ParseParameters(tc.configData)

			if tc.expectedError && err == nil {
				t.Fatalf("expected error, got nil")
			}

			if !tc.expectedError && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if len(params) != tc.expectedLen {
				t.Fatalf("expected %d parameters, got %d", tc.expectedLen, len(params))
			}
		})
	}
}

func TestParseParameterValue_EmptyString(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected any
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := convParameterValue(tc.input)
			if result != tc.expected {
				t.Fatalf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestParseParameterValue_Boolean(t *testing.T) {
	testCases := []struct {
		input    string
		expected any
	}{
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"false", false},
		{"False", false},
		{"FALSE", false},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := convParameterValue(tc.input)
			if result != tc.expected {
				t.Fatalf("expected %v for input %s, got %v", tc.expected, tc.input, result)
			}
		})
	}
}

func TestParseParameterValue_Integer(t *testing.T) {
	testCases := []struct {
		input    string
		expected any
	}{
		{"123", int64(123)},
		{"-456", int64(-456)},
		{"0", int64(0)},
		{"2147483647", int64(2147483647)},   // max int32
		{"-2147483648", int64(-2147483648)}, // min int32
		{"2147483648", int64(2147483648)},   // beyond int32, should be int64
		{"-2147483649", int64(-2147483649)}, // beyond int32, should be int64
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := convParameterValue(tc.input)
			if result != tc.expected {
				t.Fatalf("expected %v for input %s, got %v", tc.expected, tc.input, result)
			}
		})
	}
}

func TestParseParameterValue_Float(t *testing.T) {
	testCases := []struct {
		input    string
		expected float64
	}{
		{"3.14", 3.14},
		{"-2.5", -2.5},
		{"0.0", 0.0},
		{"1.23e-4", 1.23e-4},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := convParameterValue(tc.input)
			if result != tc.expected {
				t.Fatalf("expected %v for input %s, got %v", tc.expected, tc.input, result)
			}
		})
	}
}

func TestParseParameterValue_SizeUnits(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"128M", "128M"},
		{"1G", "1G"},
		{"512K", "512K"},
		{"2T", "2T"},
		{"64.5M", "64.5M"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := convParameterValue(tc.input)
			if result != tc.expected {
				t.Fatalf("expected %v for input %s, got %v", tc.expected, tc.input, result)
			}
		})
	}
}

func TestParseParameterValue_TimeUnits(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"30s", "30s"},
		{"5m", "5m"},
		{"1h", "1h"},
		{"100ms", "100ms"},
		{"50us", "50us"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := convParameterValue(tc.input)
			if result != tc.expected {
				t.Fatalf("expected %v for input %s, got %v", tc.expected, tc.input, result)
			}
		})
	}
}

func TestParseParameterValue_QuotedStrings(t *testing.T) {
	testCases := []struct {
		input    string
		expected any
	}{
		{"'true'", true},       // PostgreSQL style quoted boolean
		{"\"false\"", false},   // Double quoted boolean
		{"'123'", int64(123)},  // Quoted integer should still parse as int64
		{"'hello'", "hello"},   // Regular quoted string
		{"\"world\"", "world"}, // Double quoted string
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := convParameterValue(tc.input)
			if result != tc.expected {
				t.Fatalf("expected %v for input %s, got %v", tc.expected, tc.input, result)
			}
		})
	}
}

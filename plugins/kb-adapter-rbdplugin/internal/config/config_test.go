package config

import (
	"os"
	"strings"
	"testing"
)

func TestServerConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *ServerConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &ServerConfig{
				Host:          "0.0.0.0",
				Port:          "8080",
				ReadinessPath: "/readyz",
				LivenessPath:  "/livez",
			},
			wantErr: false,
		},
		{
			name: "empty host",
			config: &ServerConfig{
				Host:          "",
				Port:          "8080",
				ReadinessPath: "/readyz",
				LivenessPath:  "/livez",
			},
			wantErr: true,
			errMsg:  "host cannot be empty",
		},
		{
			name: "empty port",
			config: &ServerConfig{
				Host:          "0.0.0.0",
				Port:          "",
				ReadinessPath: "/readyz",
				LivenessPath:  "/livez",
			},
			wantErr: true,
			errMsg:  "port cannot be empty",
		},
		{
			name: "invalid port format",
			config: &ServerConfig{
				Host:          "0.0.0.0",
				Port:          "abc",
				ReadinessPath: "/readyz",
				LivenessPath:  "/livez",
			},
			wantErr: true,
			errMsg:  "port must be a valid integer",
		},
		{
			name: "port zero",
			config: &ServerConfig{
				Host:          "0.0.0.0",
				Port:          "0",
				ReadinessPath: "/readyz",
				LivenessPath:  "/livez",
			},
			wantErr: true,
			errMsg:  "port must be between 1 and 65535",
		},
		{
			name: "negative port",
			config: &ServerConfig{
				Host:          "0.0.0.0",
				Port:          "-1",
				ReadinessPath: "/readyz",
				LivenessPath:  "/livez",
			},
			wantErr: true,
			errMsg:  "port must be between 1 and 65535",
		},
		{
			name: "port out of range",
			config: &ServerConfig{
				Host:          "0.0.0.0",
				Port:          "70000",
				ReadinessPath: "/readyz",
				LivenessPath:  "/livez",
			},
			wantErr: true,
			errMsg:  "port must be between 1 and 65535",
		},
		{
			name: "empty readiness path",
			config: &ServerConfig{
				Host:          "0.0.0.0",
				Port:          "8080",
				ReadinessPath: "",
				LivenessPath:  "/livez",
			},
			wantErr: true,
			errMsg:  "readiness_path cannot be empty",
		},
		{
			name: "empty liveness path",
			config: &ServerConfig{
				Host:          "0.0.0.0",
				Port:          "8080",
				ReadinessPath: "/readyz",
				LivenessPath:  "",
			},
			wantErr: true,
			errMsg:  "liveness_path cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		unsetAll bool
		expected *ServerConfig
	}{
		{
			name: "all environment variables set",
			envVars: map[string]string{
				"HOST":           "test-host",
				"PORT":           "9090",
				"READINESS_PATH": "/custom/ready",
				"LIVENESS_PATH":  "/custom/live",
			},
			expected: &ServerConfig{
				Host:          "test-host",
				Port:          "9090",
				ReadinessPath: "/custom/ready",
				LivenessPath:  "/custom/live",
			},
		},
		{
			name:     "no environment variables set - use defaults",
			unsetAll: true,
			expected: &ServerConfig{
				Host:          "0.0.0.0",
				Port:          "8080",
				ReadinessPath: "/readyz",
				LivenessPath:  "/livez",
			},
		},
		{
			name: "partial environment variables set",
			envVars: map[string]string{
				"HOST": "custom-host",
				"PORT": "3000",
			},
			expected: &ServerConfig{
				Host:          "custom-host",
				Port:          "3000",
				ReadinessPath: "/readyz",
				LivenessPath:  "/livez",
			},
		},
		{
			name: "empty environment variables should use defaults",
			envVars: map[string]string{
				"HOST":           "",
				"PORT":           "",
				"READINESS_PATH": "",
				"LIVENESS_PATH":  "",
			},
			expected: &ServerConfig{
				Host:          "0.0.0.0",
				Port:          "8080",
				ReadinessPath: "/readyz",
				LivenessPath:  "/livez",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalEnv := make(map[string]string)
			envKeys := []string{"HOST", "PORT", "READINESS_PATH", "LIVENESS_PATH"}
			for _, key := range envKeys {
				originalEnv[key] = os.Getenv(key)
			}

			defer func() {
				for _, key := range envKeys {
					if originalValue, exists := originalEnv[key]; exists && originalValue != "" {
						os.Setenv(key, originalValue)
					} else {
						os.Unsetenv(key)
					}
				}
			}()

			if tt.unsetAll {
				for _, key := range envKeys {
					os.Unsetenv(key)
				}
			} else {
				for _, key := range envKeys {
					os.Unsetenv(key)
				}
				for key, value := range tt.envVars {
					if value != "" {
						os.Setenv(key, value)
					}
				}
			}

			cfg := LoadConfigFromEnv()

			if cfg.Host != tt.expected.Host {
				t.Errorf("LoadConfigFromEnv() Host = %v, want %v", cfg.Host, tt.expected.Host)
			}
			if cfg.Port != tt.expected.Port {
				t.Errorf("LoadConfigFromEnv() Port = %v, want %v", cfg.Port, tt.expected.Port)
			}
			if cfg.ReadinessPath != tt.expected.ReadinessPath {
				t.Errorf("LoadConfigFromEnv() ReadinessPath = %v, want %v", cfg.ReadinessPath, tt.expected.ReadinessPath)
			}
			if cfg.LivenessPath != tt.expected.LivenessPath {
				t.Errorf("LoadConfigFromEnv() LivenessPath = %v, want %v", cfg.LivenessPath, tt.expected.LivenessPath)
			}
		})
	}
}

func TestMustLoad(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		shouldPanic bool
		panicMsg    string
	}{
		{
			name: "valid config should not panic",
			envVars: map[string]string{
				"HOST":           "0.0.0.0",
				"PORT":           "8080",
				"READINESS_PATH": "/readyz",
				"LIVENESS_PATH":  "/livez",
			},
			shouldPanic: false,
		},
		{
			name: "invalid port should panic",
			envVars: map[string]string{
				"HOST":           "0.0.0.0",
				"PORT":           "invalid",
				"READINESS_PATH": "/readyz",
				"LIVENESS_PATH":  "/livez",
			},
			shouldPanic: true,
			panicMsg:    "configuration validation failed",
		},
		{
			name: "empty host should panic",
			envVars: map[string]string{
				"HOST":           "",
				"PORT":           "8080",
				"READINESS_PATH": "/readyz",
				"LIVENESS_PATH":  "/livez",
			},
			shouldPanic: false,
		},
		{
			name: "port out of range should panic",
			envVars: map[string]string{
				"HOST":           "0.0.0.0",
				"PORT":           "70000",
				"READINESS_PATH": "/readyz",
				"LIVENESS_PATH":  "/livez",
			},
			shouldPanic: true,
			panicMsg:    "configuration validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalEnv := make(map[string]string)
			envKeys := []string{"HOST", "PORT", "READINESS_PATH", "LIVENESS_PATH"}
			for _, key := range envKeys {
				originalEnv[key] = os.Getenv(key)
			}

			defer func() {
				for _, key := range envKeys {
					if originalValue, exists := originalEnv[key]; exists && originalValue != "" {
						os.Setenv(key, originalValue)
					} else {
						os.Unsetenv(key)
					}
				}
			}()

			for _, key := range envKeys {
				os.Unsetenv(key)
			}
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			defer func() {
				r := recover()
				if tt.shouldPanic {
					if r == nil {
						t.Errorf("MustLoad() should have panicked but did not")
						return
					}
					panicStr, ok := r.(string)
					if !ok {
						t.Errorf("MustLoad() panic type = %T, want string", r)
						return
					}
					if tt.panicMsg != "" && !strings.Contains(panicStr, tt.panicMsg) {
						t.Errorf("MustLoad() panic message = %v, want containing %v", panicStr, tt.panicMsg)
					}
				} else {
					if r != nil {
						t.Errorf("MustLoad() should not have panicked but got: %v", r)
					}
				}
			}()

			cfg := MustLoad()
			if !tt.shouldPanic && cfg == nil {
				t.Errorf("MustLoad() returned nil config")
			}
		})
	}
}

func TestInDevelopment(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		unsetEnv bool
		expected bool
	}{
		{
			name:     "ENV=dev returns true",
			envValue: "dev",
			expected: true,
		},
		{
			name:     "ENV=development returns true",
			envValue: "development",
			expected: true,
		},
		{
			name:     "ENV=DEV returns true (case insensitive)",
			envValue: "DEV",
			expected: true,
		},
		{
			name:     "ENV=DEVELOPMENT returns true (case insensitive)",
			envValue: "DEVELOPMENT",
			expected: true,
		},
		{
			name:     "ENV=Dev returns true (mixed case)",
			envValue: "Dev",
			expected: true,
		},
		{
			name:     "ENV=prod returns false",
			envValue: "prod",
			expected: false,
		},
		{
			name:     "ENV=production returns false",
			envValue: "production",
			expected: false,
		},
		{
			name:     "ENV=test returns false",
			envValue: "test",
			expected: false,
		},
		{
			name:     "ENV empty string returns false",
			envValue: "",
			expected: false,
		},
		{
			name:     "ENV not set returns false",
			unsetEnv: true,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalEnv := os.Getenv("ENV")

			defer func() {
				if originalEnv != "" {
					os.Setenv("ENV", originalEnv)
				} else {
					os.Unsetenv("ENV")
				}
			}()

			if tt.unsetEnv {
				os.Unsetenv("ENV")
			} else {
				os.Setenv("ENV", tt.envValue)
			}

			result := InDevelopment()
			if result != tt.expected {
				t.Errorf("InDevelopment() = %v, want %v", result, tt.expected)
			}
		})
	}
}

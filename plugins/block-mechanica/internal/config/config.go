// Package config
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ServerConfig echo server config
type ServerConfig struct {
	Host          string `json:"host" yaml:"host"`
	Port          string `json:"port" yaml:"port"`
	ReadinessPath string `json:"readiness_path" yaml:"readiness_path"`
	LivenessPath  string `json:"liveness_path" yaml:"liveness_path"`
}

// LoadConfigFromEnv 从环境变量加载配置
func LoadConfigFromEnv() *ServerConfig {
	config := &ServerConfig{
		Host:          getEnvOrDefault("HOST", "0.0.0.0"),
		Port:          getEnvOrDefault("PORT", "8080"),
		ReadinessPath: getEnvOrDefault("READINESS_PATH", "/readyz"),
		LivenessPath:  getEnvOrDefault("LIVENESS_PATH", "/livez"),
	}
	return config
}

// Validate 验证配置
func (c *ServerConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}
	if c.Port == "" {
		return fmt.Errorf("port cannot be empty")
	}

	// Validate port is a valid number
	portNum, err := strconv.Atoi(c.Port)
	if err != nil {
		return fmt.Errorf("port must be a valid integer: %w", err)
	}

	// Validate port range
	if portNum < 1 || portNum > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", portNum)
	}

	if c.ReadinessPath == "" {
		return fmt.Errorf("readiness_path cannot be empty")
	}
	if c.LivenessPath == "" {
		return fmt.Errorf("liveness_path cannot be empty")
	}

	return nil
}

// MustLoad 加载配置
func MustLoad() *ServerConfig {
	cfg := LoadConfigFromEnv()
	if err := cfg.Validate(); err != nil {
		panic(fmt.Sprintf("configuration validation failed: %v", err))
	}
	return cfg
}

// getEnvOrDefault 获取环境变量，如果不存在则返回默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// InDevelopment 是否为开发环境
func InDevelopment() bool {
	env := strings.ToLower(os.Getenv("ENV"))
	return env == "dev" || env == "development"
}

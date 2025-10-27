package coordinator

import (
	"fmt"
	"strings"

	"github.com/furutachiKurea/block-mechanica/internal/log"
	"github.com/furutachiKurea/block-mechanica/internal/model"
	"github.com/furutachiKurea/block-mechanica/service/adapter"
	"k8s.io/utils/ptr"
)

var _ adapter.Coordinator = &Redis{}

type Redis struct {
}

func (r *Redis) TargetPort() int {
	return 6379
}

func (r *Redis) GetSecretName(clusterName string) string {
	return fmt.Sprintf("%s-redis-account-default", clusterName)
}

func (r *Redis) GetBackupMethod() string {
	return "datafile"
}

func (r *Redis) GetParametersConfigMap(clusterName string) *string {
	cmName := fmt.Sprintf("%s-redis-redis-replication-config", clusterName)
	return &cmName
}

// SystemAccount 来自 componentDefinition
func (r *Redis) SystemAccount() *string {
	return ptr.To("default")
}

// ParseParameters 解析 Redis ConfigMap 中的 redis.conf 配置参数
// 基于实际的 ConfigMap 格式: data.redis.conf 包含 Redis 配置格式的内容
func (r *Redis) ParseParameters(configData map[string]string) ([]model.ParameterEntry, error) {
	// 获取 redis.conf 配置内容
	redisConfContent, exists := configData["redis.conf"]
	if !exists {
		log.Warn("redis.conf not found in ConfigMap data")
		return []model.ParameterEntry{}, nil
	}

	if strings.TrimSpace(redisConfContent) == "" {
		log.Info("redis.conf content is empty")
		return []model.ParameterEntry{}, nil
	}

	var parameters []model.ParameterEntry

	// 逐行解析配置文件
	lines := strings.SplitSeq(redisConfContent, "\n")
	for line := range lines {
		entry := r.parseConfigLine(line)
		if entry != nil {
			parameters = append(parameters, *entry)
		}
	}

	return parameters, nil
}

// parseConfigLine 解析单行 Redis 配置
// 返回 nil 表示该行应被跳过（注释或空行）
func (r *Redis) parseConfigLine(line string) *model.ParameterEntry {
	line = strings.TrimSpace(line)

	if line == "" {
		return nil
	}

	if strings.HasPrefix(line, "#") {
		return nil
	}

	if commentPos := strings.Index(line, "#"); commentPos >= 0 {
		line = strings.TrimSpace(line[:commentPos])
		// 如果移除注释后变为空行，跳过
		if line == "" {
			return nil
		}
	}

	parts := strings.Fields(line)
	if len(parts) < 2 {
		// 没有值的参数，跳过
		return nil
	}

	paramName := parts[0]
	paramValue := strings.Join(parts[1:], " ")

	paramValue = r.cleanQuotedValue(paramValue)

	return &model.ParameterEntry{
		Name:  paramName,
		Value: paramValue,
	}
}

// cleanQuotedValue 清理带引号的值，去除外层引号但保持内容
func (r *Redis) cleanQuotedValue(value string) string {
	value = strings.TrimSpace(value)

	if len(value) >= 2 && strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		return value[1 : len(value)-1]
	}

	if len(value) >= 2 && strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
		return value[1 : len(value)-1]
	}

	return value
}

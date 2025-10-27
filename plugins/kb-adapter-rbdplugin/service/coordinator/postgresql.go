package coordinator

import (
	"fmt"
	"strings"

	"github.com/furutachiKurea/block-mechanica/internal/log"
	"github.com/furutachiKurea/block-mechanica/internal/model"
	"github.com/furutachiKurea/block-mechanica/service/adapter"
)

var _ adapter.Coordinator = &PostgreSQL{}

// PostgreSQL 实现 Coordinator 接口
type PostgreSQL struct {
	Coordinator
}

func (c *PostgreSQL) TargetPort() int {
	return 6432
}

func (c *PostgreSQL) GetSecretName(clusterName string) string {
	// PostgreSQL 使用 postgresql 作为中间部分和 postgres 作为账户类型
	return fmt.Sprintf("%s-postgresql-account-postgres", clusterName)
}

func (c *PostgreSQL) GetBackupMethod() string {
	return "pg-basebackup"
}

func (c *PostgreSQL) GetParametersConfigMap(clusterName string) *string {
	cmName := fmt.Sprintf("%s-postgresql-postgresql-configuration", clusterName)
	return &cmName
}

// ParseParameters 解析 PostgreSQL ConfigMap 中的 postgresql.conf 配置参数
// 基于实际的 ConfigMap 格式: data.postgresql.conf 包含键值对格式的配置内容
func (c *PostgreSQL) ParseParameters(configData map[string]string) ([]model.ParameterEntry, error) {
	// 获取 postgresql.conf 配置内容
	pgConfContent, exists := configData["postgresql.conf"]
	if !exists {
		log.Warn("postgresql.conf not found in ConfigMap data")
		return []model.ParameterEntry{}, nil
	}

	if strings.TrimSpace(pgConfContent) == "" {
		log.Info("postgresql.conf content is empty")
		return []model.ParameterEntry{}, nil
	}

	lines := strings.Split(pgConfContent, "\n")
	parameters := make([]model.ParameterEntry, 0, len(lines)/2)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		line = removeInlineComment(line)
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" {
			continue
		}

		param := model.ParameterEntry{
			Name:  key,
			Value: convParameterValue(value),
		}
		parameters = append(parameters, param)
	}

	return parameters, nil
}

// removeInlineComment 移除行尾注释,但保留引号内的 # 字符
//
// 例如: "key = 'value # not comment' # this is comment" -> "key = 'value # not comment'"
func removeInlineComment(line string) string {
	inSingleQuote := false
	inDoubleQuote := false

	for i, ch := range line {
		switch ch {
		case '\'':
			if !inDoubleQuote {
				inSingleQuote = !inSingleQuote
			}
		case '"':
			if !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
			}
		case '#':
			// 如果不在引号内,这是注释的开始
			if !inSingleQuote && !inDoubleQuote {
				return strings.TrimSpace(line[:i])
			}
		}
	}

	return line
}

package coordinator

import (
	"fmt"

	"github.com/furutachiKurea/kb-adapter-rbdplugin/service/adapter"
)

var _ adapter.Coordinator = &RabbitMQ{}

// RabbitMQ 实现 RabbitMQ 的 Coordinator
//
// - 不支持备份功能
//
// - 不支持参数配置
type RabbitMQ struct {
	Coordinator
}

func (r *RabbitMQ) TargetPort() int {
	return 5672
}

func (r *RabbitMQ) GetSecretName(clusterName string) string {
	return fmt.Sprintf("%s-rabbitmq-account-root", clusterName)
}

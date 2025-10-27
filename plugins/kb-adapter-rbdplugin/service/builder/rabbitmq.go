package builder

import (
	"fmt"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/furutachiKurea/block-mechanica/internal/log"
	"github.com/furutachiKurea/block-mechanica/internal/model"
	"github.com/furutachiKurea/block-mechanica/service/adapter"
)

var _ adapter.ClusterBuilder = &RabbitMQ{}

type RabbitMQ struct {
	Builder
}

func (r *RabbitMQ) BuildCluster(input model.ClusterInput) (*kbappsv1.Cluster, error) {
	cluster, err := r.Builder.BuildCluster(input)
	if err != nil {
		return nil, fmt.Errorf("build base cluster: %w", err)
	}

	// RabbitMQ 不支持备份
	cluster.Spec.Backup = nil

	log.Debug("Build rabbitmq cluster", log.Any("cluster", cluster))

	return cluster, nil
}

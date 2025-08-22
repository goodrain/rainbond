package builder

import (
	"context"
	"fmt"

	"github.com/furutachiKurea/block-mechanica/internal/log"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/furutachiKurea/block-mechanica/internal/model"
	"github.com/furutachiKurea/block-mechanica/service/adapter"
)

var _ adapter.ClusterBuilder = &MySQLBuilder{}

// MySQLBuilder 实现 MySQL 的 Builder
type MySQLBuilder struct {
	BaseBuilder
}

func (b MySQLBuilder) BuildCluster(ctx context.Context, req model.ClusterInput) (*kbappsv1.Cluster, error) {
	cluster, err := b.BaseBuilder.BuildCluster(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("build base cluster: %w", err)
	}

	// Backup
	if cluster.Spec.Backup != nil {
		cluster.Spec.Backup.Method = "xtrabackup"
	}

	log.Debug("Build mysql cluster", log.Any("cluster", cluster))

	return cluster, nil
}

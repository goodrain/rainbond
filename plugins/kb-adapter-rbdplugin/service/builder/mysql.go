package builder

import (
	"fmt"

	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/log"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/model"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/service/adapter"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

var _ adapter.ClusterBuilder = &MySQL{}

// MySQL 实现 MySQL 的 Builder
type MySQL struct {
	Builder
}

func (b *MySQL) BuildCluster(input model.ClusterInput) (*kbappsv1.Cluster, error) {
	cluster, err := b.Builder.BuildCluster(input)
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

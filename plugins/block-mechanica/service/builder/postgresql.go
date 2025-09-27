package builder

import (
	"context"
	"fmt"

	"github.com/furutachiKurea/block-mechanica/internal/log"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/furutachiKurea/block-mechanica/internal/model"
	"github.com/furutachiKurea/block-mechanica/service/adapter"
)

var _ adapter.ClusterBuilder = &PostgreBuilder{}

// PostgreBuilder 实现 PostgreSQL 的 Builder
type PostgreBuilder struct {
	BaseBuilder
}

func (b PostgreBuilder) BuildCluster(ctx context.Context, req model.ClusterInput) (*kbappsv1.Cluster, error) {
	cluster, err := b.BaseBuilder.BuildCluster(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("build base cluster: %w", err)
	}

	// Postgresql 需要额外添加 lables
	//
	// PostgreSQL's CMPD specifies `KUBERNETES_SCOPE_LABEL=apps.kubeblocks.postgres.patroni/scope` through ENVs
	// The KUBERNETES_SCOPE_LABEL is used to define the label key that Patroni will use to tag Kubernetes resources.
	// This helps Patroni identify which resources belong to the specified scope (or cluster) used to define the label key
	// that Patroni will use to tag Kubernetes resources.
	// This helps Patroni identify which resources belong to the specified scope (or cluster).
	// Note: DO NOT REMOVE THIS LABEL
	// update the value w.r.t your cluster name
	// the value must follow the format <cluster.metadata.name>-postgresql
	// which is pg-cluster-postgresql in this examples
	// replace `pg-cluster` with your cluster name
	if cluster.Spec.ComponentSpecs[0].Labels == nil {
		cluster.Spec.ComponentSpecs[0].Labels = make(map[string]string)
	}
	cluster.Spec.ComponentSpecs[0].Labels["apps.kubeblocks.postgres.patroni/scope"] = fmt.Sprintf("%s-postgresql", cluster.Name)

	// Backup
	if cluster.Spec.Backup != nil {
		cluster.Spec.Backup.Method = "pg-basebackup"
	}

	log.Debug("Build postgresql cluster", log.Any("cluster", cluster))

	return cluster, nil
}

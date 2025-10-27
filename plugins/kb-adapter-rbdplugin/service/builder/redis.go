package builder

import (
	"github.com/furutachiKurea/block-mechanica/internal/log"
	"github.com/furutachiKurea/block-mechanica/internal/model"
	"github.com/furutachiKurea/block-mechanica/service/adapter"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

var _ adapter.ClusterBuilder = &Redis{}

// Redis 实现 replication Redis cluster
type Redis struct {
	Builder
}

func (r *Redis) BuildCluster(input model.ClusterInput) (*kbappsv1.Cluster, error) {
	cluster, err := r.Builder.BuildCluster(input)
	if err != nil {
		return nil, err
	}

	resource, err := input.ParseResources()
	if err != nil {
		return nil, err
	}

	cluster.Spec.Topology = "replication"

	// Redis replication cluster 需要额外配置 sentinel，资源分配与 redis 一致
	sentinel := kbappsv1.ClusterComponentSpec{
		Name:     "redis-sentinel",
		Replicas: input.Replicas,
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.CPU,
				corev1.ResourceMemory: resource.Memory,
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.CPU,
				corev1.ResourceMemory: resource.Memory,
			},
		},
		VolumeClaimTemplates: []kbappsv1.ClusterComponentVolumeClaimTemplate{
			{
				Name: "data",
				Spec: kbappsv1.PersistentVolumeClaimSpec{
					StorageClassName: ptr.To(input.StorageClass),
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.Storage,
						},
					},
				},
			},
		},
	}

	cluster.Spec.ComponentSpecs = append(cluster.Spec.ComponentSpecs, sentinel)

	// redis use datafile backup method
	if cluster.Spec.Backup != nil {
		cluster.Spec.Backup.Method = "datafile"
	}

	log.Debug("Build redis replication cluster", log.Any("cluster", cluster))
	return cluster, nil
}

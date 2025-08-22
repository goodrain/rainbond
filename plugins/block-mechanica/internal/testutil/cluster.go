package testutil

import (
	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/furutachiKurea/block-mechanica/internal/index"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewTestCluster 创建一个用于测试的通用 Cluster 对象
func NewTestCluster(name, namespace, clusterType string) *kbappsv1.Cluster {
	return &kbappsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				index.ServiceIDLabel: "test-service-id",
			},
		},
		Spec: kbappsv1.ClusterSpec{
			ClusterDef: clusterType,
			ComponentSpecs: []kbappsv1.ClusterComponentSpec{
				{
					Name:           clusterType,
					ServiceVersion: "v1.0.0",
					Replicas:       1,
				},
			},
		},
	}
}

// NewPostgreSQLTestCluster 创建一个 PostgreSQL Cluster 对象
func NewPostgreSQLTestCluster(name, namespace string) *kbappsv1.Cluster {
	return NewTestCluster(name, namespace, "postgresql")
}

// NewMySQLTestCluster 创建一个 MySQL Cluster 对象
func NewMySQLTestCluster(name, namespace string) *kbappsv1.Cluster {
	return NewTestCluster(name, namespace, "mysql")
}

// NewMySQLTestClusterForBackup 创建一个用于备份测试的 MySQL Cluster
func NewMySQLTestClusterForBackup(name, namespace string) *kbappsv1.Cluster {
	cluster := NewTestCluster(name, namespace, "mysql")
	cluster.Labels[index.ServiceIDLabel] = "svc-id"
	return cluster
}

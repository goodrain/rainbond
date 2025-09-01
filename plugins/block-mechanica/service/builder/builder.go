// Package builder 提供构建 adapter.ClusterBuilder 的 Builder 实现
//
// ClusterBuilder 用于在 Rainbond 中 KubeBlocks Cluster 的创建
package builder

import (
	"context"
	"crypto/md5"
	"fmt"
	"time"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/furutachiKurea/block-mechanica/internal/index"
	"github.com/furutachiKurea/block-mechanica/internal/log"
	"github.com/furutachiKurea/block-mechanica/internal/model"
	"github.com/furutachiKurea/block-mechanica/service/adapter"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ adapter.ClusterBuilder = &BaseBuilder{}

// BaseBuilder 实现 ClusterBuilder 接口，所有的 Builder 都应基于 BaseBuilder 实现
type BaseBuilder struct{}

// generateShortName 生成基于哈希的短名称，确保唯一性且长度可控
// 格式：{originalName}-{hash4}，其中 hash4 是 MD5 哈希的前4位十六进制字符
func (b BaseBuilder) generateShortName(originalName string) string {
	timestamp := time.Now().UnixNano()
	input := fmt.Sprintf("%s-%d", originalName, timestamp)

	hash := md5.Sum([]byte(input))

	hashSuffix := fmt.Sprintf("%x", hash[:2])

	return fmt.Sprintf("%s-%s", originalName, hashSuffix)
}

func (b BaseBuilder) BuildCluster(ctx context.Context, clusterInput model.ClusterInput) (*kbappsv1.Cluster, error) {
	cpuQuantity, err := resource.ParseQuantity(clusterInput.CPU)
	if err != nil {
		return nil, fmt.Errorf("invalid CPU quantity: %w", err)
	}
	memoryQuantity, err := resource.ParseQuantity(clusterInput.Memory)
	if err != nil {
		return nil, fmt.Errorf("invalid memory quantity: %w", err)
	}
	diskQuantity, err := resource.ParseQuantity(clusterInput.Storage)
	if err != nil {
		return nil, fmt.Errorf("invalid disk quantity: %w", err)
	}

	// 生成短名称，避免同团队内重名
	clusterName := b.generateShortName(clusterInput.Name)

	cluster := &kbappsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: clusterInput.Namespace,
		},
		Spec: kbappsv1.ClusterSpec{
			TerminationPolicy: clusterInput.TerminationPolicy,
			ClusterDef:        clusterInput.Type,
			ComponentSpecs: []kbappsv1.ClusterComponentSpec{
				{
					Name:           clusterInput.Type,
					ServiceVersion: clusterInput.Version,
					Replicas:       clusterInput.Replicas,
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    cpuQuantity,
							corev1.ResourceMemory: memoryQuantity,
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    cpuQuantity,
							corev1.ResourceMemory: memoryQuantity,
						},
					},
					VolumeClaimTemplates: []kbappsv1.ClusterComponentVolumeClaimTemplate{
						{
							Name: "data",
							Spec: kbappsv1.PersistentVolumeClaimSpec{
								StorageClassName: ptr.To(clusterInput.StorageClass),
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
								},
								Resources: corev1.VolumeResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceStorage: diskQuantity,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if clusterInput.BackupRepo != "" {
		cluster.Spec.Backup = &kbappsv1.ClusterBackup{
			RepoName:        clusterInput.BackupRepo,
			Enabled:         ptr.To(true),
			CronExpression:  clusterInput.Schedule.Cron(),
			RetentionPeriod: clusterInput.RetentionPeriod,
		}
	}

	return cluster, nil
}

// AssociateToKubeBlocksComponent 关联 Cluster 到 Rainbond 组件
func (b BaseBuilder) AssociateToKubeBlocksComponent(ctx context.Context, c client.Client, clusterInput model.ClusterInput) error {
	log.Debug("start associate cluster to rainbond component", log.String("service_id", clusterInput.RBDService.ServiceID), log.String("cluster", clusterInput.Name))

	const labelServiceID = index.ServiceIDLabel

	err := wait.PollUntilContextCancel(ctx, 500*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		var cluster kbappsv1.Cluster
		if err := c.Get(ctx, types.NamespacedName{
			Name:      clusterInput.Name,
			Namespace: clusterInput.Namespace,
		}, &cluster); err != nil {
			log.Debug("Cluster not found yet, waiting", log.String("cluster", clusterInput.Name), log.String("namespace", clusterInput.Namespace))
			return false, nil
		}

		// 检查是否有正确的 service_id 标签
		if cluster.Labels != nil && cluster.Labels[index.ServiceIDLabel] == clusterInput.RBDService.ServiceID {
			log.Debug("Cluster already has correct service_id label", log.String("service_id", clusterInput.RBDService.ServiceID))
			return true, nil
		}

		patchData := fmt.Sprintf(`{
			"metadata": {
				"labels": {
					"%s": "%s"
				}
			}
		}`, labelServiceID, clusterInput.RBDService.ServiceID)

		if err := c.Patch(ctx, &kbappsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterInput.Name,
				Namespace: clusterInput.Namespace,
			},
		}, client.RawPatch(types.MergePatchType, []byte(patchData))); err != nil {
			log.Debug("Patch operation failed, retrying", log.String("cluster", clusterInput.Name), log.Err(err))
			return false, nil
		}

		log.Debug("Successfully added service_id label to cluster", log.String("service_id", clusterInput.RBDService.ServiceID), log.String("cluster", clusterInput.Name))
		return true, nil
	})

	if err != nil {
		return fmt.Errorf("failed to associate cluster %s/%s with service_id label after retries: %w", clusterInput.Namespace, clusterInput.Name, err)
	}

	log.Info("Associated KubeBlocks Cluster to Rainbond component", log.String("service_id", clusterInput.RBDService.ServiceID), log.String("cluster", clusterInput.Name))

	return nil
}

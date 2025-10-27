package cluster

import (
	"context"
	"errors"
	"fmt"

	"github.com/furutachiKurea/block-mechanica/internal/index"
	"github.com/furutachiKurea/block-mechanica/internal/log"
	"github.com/furutachiKurea/block-mechanica/internal/model"
	"github.com/furutachiKurea/block-mechanica/internal/mono"
	"github.com/furutachiKurea/block-mechanica/service/adapter"
	"github.com/furutachiKurea/block-mechanica/service/kbkit"
	"github.com/furutachiKurea/block-mechanica/service/registry"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateCluster 依据 req 创建 KubeBlocks Cluster
//
// 通过将 service_id 添加至 Cluster 的 labels 中以关联 KubeBlocks Component 与 Cluster,
// 同时，Rainbond 也通过这层关系来判断 Rainbond 组件是否为 KubeBlocks Component
//
// 返回成功创建的 KubeBlocks Cluster 实例
func (s *Service) CreateCluster(ctx context.Context, input model.ClusterInput) (*kbappsv1.Cluster, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	clusterAdapter, ok := registry.Cluster[input.Type]
	if !ok {
		return nil, fmt.Errorf("unsupported cluster type: %s", input.Type)
	}

	cluster, err := clusterAdapter.Builder.BuildCluster(input)
	if err != nil {
		return nil, fmt.Errorf("build %s cluster: %w", input.Type, err)
	}

	var (
		// 为启用了 custom secret 的 Addon 设置 systemAccount
		systemAccount *string
		// custom secret name，不为空说明创建了 custom secret，需要回滚
		customSecretName string
	)

	systemAccount = clusterAdapter.Coordinator.SystemAccount()
	if systemAccount != nil {
		s.configureSystemAccount(cluster, clusterAdapter.Coordinator, *systemAccount)
		customSecretName = clusterAdapter.Coordinator.GetSecretName(cluster.Name)

		if err := s.createSystemAccountSecret(
			ctx,
			customSecretName,
			cluster.Namespace,
			*systemAccount,
			input.RBDService.ServiceID,
		); err != nil {
			log.Debug("failed to create system account secret",
				log.String("secret_name", customSecretName),
				log.Err(err))
			return nil, fmt.Errorf("create system account secret: %w", err)
		}
		log.Debug("created system account secret", log.String("secret", customSecretName))
	}

	if err := s.client.Create(ctx, cluster); err != nil {
		// rollback
		if customSecretName != "" {
			s.deleteSecretByName(ctx, customSecretName, cluster.Namespace)
		}
		return nil, fmt.Errorf("create cluster: %w", err)
	}

	log.Debug("created cluster", log.String("cluster", cluster.Name))

	input.Name = cluster.Name
	if err := s.associateToKubeBlocksComponent(ctx, cluster, input.RBDService.ServiceID); err != nil {
		// rollbacl
		if delErr := s.deleteCluster(ctx, cluster, true); delErr != nil {
			log.Error("failed to cleanup cluster after association failure",
				log.String("cluster", cluster.Name),
				log.Err(delErr))
		}
		return nil, fmt.Errorf("associate to rainbond component: %w", err)
	}

	log.Info("Successfully created cluster",
		log.String("cluster", cluster.Name),
		log.String("namespace", cluster.Namespace),
		log.String("service_id", input.RBDService.ServiceID))

	return cluster, nil
}

// configureSystemAccount 为  设置 systemAccount
func (s *Service) configureSystemAccount(
	cluster *kbappsv1.Cluster,
	coordinator adapter.Coordinator,
	systemAccountName string) {
	secretName := coordinator.GetSecretName(cluster.Name)

	for i := range cluster.Spec.ComponentSpecs {
		if cluster.Spec.ComponentSpecs[i].Name != kbkit.ClusterType(cluster) {
			continue
		}

		cluster.Spec.ComponentSpecs[i].SystemAccounts = []kbappsv1.ComponentSystemAccount{
			{
				Name: systemAccountName,
				SecretRef: &kbappsv1.ProvisionSecretRef{
					Name:      secretName,
					Namespace: cluster.Namespace,
				},
			},
		}

	}

}

// createSystemAccountSecret 创建 cluster 使用的 custom secret
func (s *Service) createSystemAccountSecret(
	ctx context.Context,
	secretName string,
	namespace string,
	accountName string,
	serviceID string,
) error {
	password := mono.GeneratePWD(16)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				index.ServiceIDLabel: serviceID,
			},
		},
		Immutable: ptr.To(true),
		Data: map[string][]byte{
			"username": []byte(accountName),
			"password": []byte(password),
		},
	}

	return s.client.Create(ctx, secret)
}

// DeleteClusters 删除 KubeBlocks 数据库集群
//
// 批量删除指定 serviceIDs 对应的 Cluster，忽略找不到的 service_id
func (s *Service) DeleteClusters(ctx context.Context, serviceIDs []string) error {
	for _, serviceID := range serviceIDs {
		cluster, err := kbkit.GetClusterByServiceID(ctx, s.client, serviceID)
		if err != nil {
			if errors.Is(err, kbkit.ErrTargetNotFound) {
				continue
			}
			return fmt.Errorf("get cluster by service_id %s: %w", serviceID, err)
		}

		if err := s.deleteCluster(ctx, cluster, false); err != nil {
			return fmt.Errorf("delete cluster for service_id %s: %w", serviceID, err)
		}
	}
	return nil
}

// CancelClusterCreate 取消集群创建
//
// 在删除前将 TerminationPolicy 调整为 WipeOut，确保 PVC/PV 等存储资源一并清理，避免脏数据残留
func (s *Service) CancelClusterCreate(ctx context.Context, rbd model.RBDService) error {
	cluster, err := kbkit.GetClusterByServiceID(ctx, s.client, rbd.ServiceID)
	if err != nil {
		return fmt.Errorf("get cluster by service_id %s: %w", rbd.ServiceID, err)
	}
	return s.deleteCluster(ctx, cluster, true)
}

// deleteCluster 内部删除方法，提供是否将 TerminationPolicy 设置为 WipeOut 的选项
func (s *Service) deleteCluster(ctx context.Context, cluster *kbappsv1.Cluster, isCancel bool) error {
	log.Info("Found cluster for deletion",
		log.String("cluster_name", cluster.Name),
		log.String("namespace", cluster.Namespace),
		log.String("current_termination_policy", string(cluster.Spec.TerminationPolicy)),
		log.Bool("wipe_out", isCancel))

	// 清理 Cluster 的 OpsRequest
	if err := s.cleanupClusterOpsRequests(ctx, cluster); err != nil {
		log.Warn("Failed to cleanup OpsRequests, proceeding with cluster deletion",
			log.String("cluster_name", cluster.Name),
			log.Err(err))
	}

	if isCancel && cluster.Spec.TerminationPolicy != kbappsv1.WipeOut {
		log.Info("Updating TerminationPolicy to WipeOut before deletion",
			log.String("cluster_name", cluster.Name),
			log.String("namespace", cluster.Namespace))

		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Spec.TerminationPolicy = kbappsv1.WipeOut

		if err := s.client.Patch(ctx, cluster, patch); err != nil {
			return fmt.Errorf("patch cluster %s/%s terminationPolicy to WipeOut: %w",
				cluster.Namespace, cluster.Name, err)
		}

		log.Info("Successfully updated TerminationPolicy to WipeOut",
			log.String("cluster_name", cluster.Name),
			log.String("namespace", cluster.Namespace))
	}

	policy := metav1.DeletePropagationForeground
	deleteOptions := &client.DeleteOptions{
		PropagationPolicy: &policy,
	}

	if err := s.client.Delete(ctx, cluster, deleteOptions); err != nil {
		return fmt.Errorf("delete cluster %s/%s: %w", cluster.Namespace, cluster.Name, err)
	}

	log.Info("Successfully initiated cluster deletion",
		log.String("cluster_name", cluster.Name),
		log.String("namespace", cluster.Namespace),
		log.Bool("wipe_out", isCancel))

	if err := s.deleteSecretsByCluster(ctx, cluster); err != nil {
		log.Warn("Failed to cleanup secrets, but cluster deletion succeeded",
			log.String("cluster", cluster.Name),
			log.Err(err))
	}

	return nil
}

// ManageClustersLifecycle 通过创建 OpsRequest 批量管理多个 Cluster 的生命周期
func (s *Service) ManageClustersLifecycle(ctx context.Context, operation opsv1alpha1.OpsType, serviceIDs []string) *model.BatchOperationResult {
	manageResult := model.NewBatchOperationResult()
	for _, serviceID := range serviceIDs {
		cluster, err := kbkit.GetClusterByServiceID(ctx, s.client, serviceID)
		if errors.Is(err, kbkit.ErrTargetNotFound) {
			continue
		}
		if err != nil {
			manageResult.AddFailed(serviceID, err)
			continue
		}

		if err = kbkit.CreateLifecycleOpsRequest(ctx, s.client, cluster, operation); err == nil {
			manageResult.AddSucceeded(serviceID)
		} else {
			manageResult.AddFailed(serviceID, err)
		}
	}
	return manageResult
}

// cleanupClusterOpsRequests 清理指定 Cluster 的所有 OpsRequest
func (s *Service) cleanupClusterOpsRequests(ctx context.Context, cluster *kbappsv1.Cluster) error {
	// 获取并清理所有非终态 OpsRequest，使其进入终态
	blockingOps, err := kbkit.GetAllNonFinalOpsRequests(ctx, s.client, cluster.Namespace, cluster.Name)
	if err != nil {
		return fmt.Errorf("get existing opsrequests: %w", err)
	}

	if len(blockingOps) > 0 {
		log.Debug("Found blocking OpsRequests, initiating cleanup",
			log.String("cluster", cluster.Name),
			log.Int("blocking_count", len(blockingOps)))

		if err := kbkit.CleanupBlockingOps(ctx, s.client, blockingOps); err != nil {
			return fmt.Errorf("cleanup blocking ops: %w", err)
		}
	}

	// 获取并删除所有 OpsRequest
	allOps, err := kbkit.GetAllOpsRequestsByCluster(ctx, s.client, cluster.Namespace, cluster.Name)
	if err != nil {
		return fmt.Errorf("get all opsrequests: %w", err)
	}

	if len(allOps) == 0 {
		log.Debug("No OpsRequests found for cluster",
			log.String("cluster", cluster.Name))
		return nil
	}

	log.Info("Deleting all OpsRequests for complete cleanup",
		log.String("cluster", cluster.Name),
		log.Int("total_count", len(allOps)))

	// 并发删除所有 OpsRequest，避免孤儿资源
	if err := s.deleteAllOpsRequestsConcurrently(ctx, allOps); err != nil {
		return fmt.Errorf("delete all ops: %w", err)
	}

	log.Info("Successfully cleaned up all OpsRequests",
		log.String("cluster", cluster.Name),
		log.Int("deleted_count", len(allOps)))

	return nil
}

// deleteAllOpsRequestsConcurrently 并发删除所有 OpsRequest
func (s *Service) deleteAllOpsRequestsConcurrently(ctx context.Context, allOps []opsv1alpha1.OpsRequest) error {
	if len(allOps) == 0 {
		return nil
	}

	group, gctx := errgroup.WithContext(ctx)
	for i := range allOps {
		op := &allOps[i]
		group.Go(func() error {
			if err := s.client.Delete(gctx, op); err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return fmt.Errorf("failed to delete opsrequest %s: %w", op.Name, err)
			}
			return nil
		})
	}

	return group.Wait()
}

// deleteSecretsByCluster 删除 Cluster 引用的 SystemAccount secrets
//
// 只有当 secret 仅被当前 cluster 引用时才会删除，避免误删 restored cluster 共享的 secret
func (s *Service) deleteSecretsByCluster(ctx context.Context, cluster *kbappsv1.Cluster) error {
	secretNames := extractSecretRefs(cluster)
	if len(secretNames) == 0 {
		log.Debug("no systemAccount secrets setted, skip",
			log.String("cluster", cluster.Name),
		)
		return nil
	}

	var clusterList kbappsv1.ClusterList
	if err := s.client.List(ctx, &clusterList, client.InNamespace(cluster.Namespace)); err != nil {
		return fmt.Errorf("list clusters in namespace %s: %w", cluster.Namespace, err)
	}

	var deletionErrors []error

	for _, secretName := range secretNames {
		refCount := countSecretReferences(clusterList.Items, secretName)

		if refCount > 1 {
			log.Debug("skipping secret deletion, still in use by other clusters",
				log.String("secret", secretName),
				log.Int("reference_count", refCount))
			continue
		}

		if err := s.deleteSecret(ctx, secretName, cluster.Namespace); err != nil {
			log.Error("failed to delete secret",
				log.String("secret", secretName),
				log.Err(err))
			deletionErrors = append(deletionErrors,
				fmt.Errorf("delete secret %s: %w", secretName, err))
			continue
		}

		log.Debug("deleted systemAccount secret",
			log.String("cluster", cluster.Name),
			log.String("secret", secretName),
			log.Int("final_reference_count(should be 0)", refCount))
	}

	if len(deletionErrors) > 0 {
		return fmt.Errorf("failed to delete %d secret(s)", len(deletionErrors))
	}

	return nil
}

// deleteSecretByName 按名字删除 Secret
// 用于 Cluster 创建失败的场景
// 不返回错误，只记录日志
func (s *Service) deleteSecretByName(ctx context.Context, secretName, namespace string) {
	log.Warn("cleaning up secret after operation failure",
		log.String("secret", secretName),
		log.String("namespace", namespace))

	if err := s.deleteSecret(ctx, secretName, namespace); err != nil {
		log.Error("failed to cleanup secret",
			log.String("secret", secretName),
			log.Err(err))
		return
	}

	log.Info("successfully cleaned up secret", log.String("secret", secretName))
}

// deleteSecret 删除指定的 Secret，IsNotFound 不视为错误
func (s *Service) deleteSecret(ctx context.Context, name, namespace string) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	if err := s.client.Delete(ctx, secret); err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("secret already deleted or not found",
				log.String("secret", name),
				log.String("namespace", namespace))
			return nil
		}
		return fmt.Errorf("delete secret %s/%s: %w", namespace, name, err)
	}

	log.Debug("successfully deleted secret",
		log.String("secret", name),
		log.String("namespace", namespace))
	return nil
}

// extractSecretRefs 从 cluster 中提取所有 SystemAccount 的 secretRef 名称
func extractSecretRefs(cluster *kbappsv1.Cluster) []string {
	if cluster == nil {
		return nil
	}

	// 用于储存已出现的 secret 名称，避免重复
	seen := make(map[string]struct{})
	var secretNames []string

	for _, component := range cluster.Spec.ComponentSpecs {
		for _, systemAccount := range component.SystemAccounts {
			if systemAccount.SecretRef == nil || systemAccount.SecretRef.Name == "" {
				continue
			}

			secretName := systemAccount.SecretRef.Name
			if _, exists := seen[secretName]; !exists {
				seen[secretName] = struct{}{}
				secretNames = append(secretNames, secretName)
			}
		}
	}

	return secretNames
}

// countSecretReferences 计算 secret 的被引用次数
func countSecretReferences(clusters []kbappsv1.Cluster, secretName string) int {
	if secretName == "" {
		return 0
	}

	count := 0
	for i := range clusters {
		if clusterReferencesSecret(&clusters[i], secretName) {
			count++
		}
	}
	return count
}

func clusterReferencesSecret(cluster *kbappsv1.Cluster, secretName string) bool {
	if cluster == nil || secretName == "" {
		return false
	}

	for _, comp := range cluster.Spec.ComponentSpecs {
		for _, sysAcct := range comp.SystemAccounts {
			if sysAcct.SecretRef != nil && sysAcct.SecretRef.Name == secretName {
				return true
			}
		}
	}

	return false
}

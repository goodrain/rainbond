package cluster

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/furutachiKurea/block-mechanica/internal/log"
	"github.com/furutachiKurea/block-mechanica/service/kbkit"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RestoreFromBackup 从用户通过 backupName 指定的备份中 restore cluster，
// 返回 restored cluster 的名称 + clusterDef, 用于 Rainbond 更新 KubeBlocks Component 信息
//
// 该方法将为恢复的 cluster 通过 newServiceID 绑定到一个新的 KubeBlocks Component 中
func (s *Service) RestoreFromBackup(ctx context.Context, oldServiceID, newServiceID, backupName string) (string, error) {
	cluster, err := kbkit.GetClusterByServiceID(ctx, s.client, oldServiceID)
	if err != nil {
		return "", fmt.Errorf("get cluster by service_id: %w", err)
	}

	log.Debug("starting cluster restore from backup",
		log.String("backup_name", backupName),
		log.String("service_id", oldServiceID),
		log.String("old_cluster", cluster.Name),
	)

	// 创建 Restore OpsRequest
	ops, err := kbkit.CreateRestoreOpsRequest(ctx, s.client, cluster, backupName)
	if err != nil {
		return "", fmt.Errorf("create restore opsrequest: %w", err)
	}

	log.Debug("restore opsrequest created, waiting for restored cluster",
		log.String("ops_request", ops.Name),
		log.String("new_cluster_name", ops.Spec.ClusterName),
	)

	// 等待新 cluster 创建，同时监控 OpsRequest 状态
	newCluster, err := s.waitForRestoredCluster(ctx, ops, cluster.Name)
	if err != nil {
		return "", fmt.Errorf("wait for restored cluster: %w", err)
	}

	// 为新 cluster 添加 service_id 标签，建立与 KubeBlocks Component 的关联
	if err := s.associateToKubeBlocksComponent(ctx, newCluster, newServiceID); err != nil {
		return "", fmt.Errorf("associate cluster to kubeblocks component: %w", err)
	}

	log.Debug("cluster restore from backup completed successfully",
		log.String("old_cluster", cluster.Name),
		log.String("backup_name", backupName),
		log.String("new_cluster", newCluster.Name),
	)

	return fmt.Sprintf("%s-%s", newCluster.Name, newCluster.Spec.ClusterDef), nil
}

// waitForRestoredCluster 等待由 Restore OpsRequest 创建的新 cluster 出现在集群中
//
// 该函数会轮询检查新 cluster 是否存在，超时时间为 20 秒
// 同时监控 OpsRequest 状态，如果 OpsRequest 失败则立即退出。
func (s *Service) waitForRestoredCluster(ctx context.Context, ops *opsv1alpha1.OpsRequest, oldClusterName string) (*kbappsv1.Cluster, error) {
	newClusterName := ops.Spec.ClusterName
	namespace := ops.Namespace

	log.Debug("waiting for restored cluster to be created",
		log.String("new_cluster", newClusterName),
		log.String("namespace", namespace),
		log.String("ops_request", ops.Name),
	)

	// 20 秒超时
	timeoutCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	var newCluster *kbappsv1.Cluster
	err := wait.PollUntilContextCancel(timeoutCtx, 500*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		// 检查 OpsRequest 状态，如果失败则立即退出
		var latestOps opsv1alpha1.OpsRequest
		if err := s.client.Get(ctx, client.ObjectKey{
			Name:      ops.Name,
			Namespace: ops.Namespace,
		}, &latestOps); err != nil {
			log.Debug("failed to get opsrequest status", log.Err(err))
			return false, nil
		}

		// 检查 OpsRequest 是否失败
		if latestOps.Status.Phase == opsv1alpha1.OpsFailedPhase ||
			latestOps.Status.Phase == opsv1alpha1.OpsCancelledPhase ||
			latestOps.Status.Phase == opsv1alpha1.OpsAbortedPhase {

			// 处理失败的 OpsRequest, 将失败的 Ops 标记为旧 cluster 所属
			if handleErr := s.handleFailedRestoreOps(ctx, &latestOps, oldClusterName); handleErr != nil {
				log.Error("failed to handle failed restore kbkit", log.Err(handleErr))
			}
			return false, fmt.Errorf("restore opsrequest failed with phase: %s", latestOps.Status.Phase)
		}

		// 检查新 cluster 是否存在
		var cluster kbappsv1.Cluster
		if err := s.client.Get(ctx, client.ObjectKey{
			Name:      newClusterName,
			Namespace: namespace,
		}, &cluster); err != nil {
			log.Debug("restored cluster not found yet, continuing to wait",
				log.String("cluster", newClusterName),
				log.String("namespace", namespace),
			)
			return false, nil
		}

		log.Debug("restored cluster found",
			log.String("cluster", cluster.Name),
			log.String("namespace", cluster.Namespace),
		)
		newCluster = &cluster
		return true, nil
	})

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// 超时时清理 OpsRequest
			log.Warn("timeout detected, cleaning up opsrequest",
				log.String("ops_request", ops.Name),
				log.String("new_cluster", newClusterName),
				log.String("namespace", namespace),
			)
			if cleanupErr := s.cleanupOpsRequest(ctx, ops, "timeout"); cleanupErr != nil {
				log.Error("failed to cleanup timed out opsrequest",
					log.String("ops_request", ops.Name),
					log.Err(cleanupErr))
			}
			return nil, fmt.Errorf("timeout waiting for restored cluster %s/%s to be created", namespace, newClusterName)
		}
		return nil, fmt.Errorf("error waiting for restored cluster: %w", err)
	}

	log.Info("restored cluster successfully created and found",
		log.String("cluster", newCluster.Name),
		log.String("namespace", newCluster.Namespace),
	)

	return newCluster, nil
}

// handleFailedRestoreOps 处理失败的 Restore OpsRequest
//
// 修改失败的 OpsRequest 的 app.kubernetes.io/instance 标签值为旧 cluster 名称
func (s *Service) handleFailedRestoreOps(ctx context.Context, ops *opsv1alpha1.OpsRequest, oldClusterName string) error {
	log.Debug("handling failed restore opsrequest",
		log.String("ops_request", ops.Name),
		log.String("old_cluster", oldClusterName),
	)

	patchData := fmt.Sprintf(`{
		"metadata": {
			"labels": {
				"%s": "%s"
			}
		}
	}`, constant.AppInstanceLabelKey, oldClusterName)

	if err := s.client.Patch(ctx, &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ops.Name,
			Namespace: ops.Namespace,
		},
	}, client.RawPatch(types.MergePatchType, []byte(patchData))); err != nil {
		return fmt.Errorf("patch failed restore opsrequest %s/%s app instance label: %w", ops.Namespace, ops.Name, err)
	}

	log.Debug("updated failed restore opsrequest app instance label",
		log.String("ops_request", ops.Name),
		log.String("old_cluster", oldClusterName),
	)

	return nil
}

// cleanupOpsRequest 清理指定的 OpsRequest
//
// 用于清理超时的 OpsRequest，防止资源泄漏和状态不一致
func (s *Service) cleanupOpsRequest(ctx context.Context, ops *opsv1alpha1.OpsRequest, reason string) error {
	log.Debug("cleaning up opsrequest",
		log.String("ops_request", ops.Name),
		log.String("namespace", ops.Namespace),
		log.String("reason", reason),
	)

	// 删除 OpsRequest
	if err := s.client.Delete(ctx, &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ops.Name,
			Namespace: ops.Namespace,
		},
	}); err != nil {
		return fmt.Errorf("delete opsrequest %s/%s: %w", ops.Namespace, ops.Name, err)
	}

	log.Debug("successfully cleaned up opsrequest",
		log.String("ops_request", ops.Name),
		log.String("namespace", ops.Namespace),
		log.String("reason", reason),
	)

	return nil
}

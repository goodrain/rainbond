package controller

import (
	"context"
	"errors"
	"fmt"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/furutachiKurea/block-mechanica/internal/log"
	"github.com/furutachiKurea/block-mechanica/service"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	clusterCreating = kbappsv1.CreatingClusterPhase
	clusterRunning  = kbappsv1.RunningClusterPhase
	clusterUpdating = kbappsv1.UpdatingClusterPhase
	clusterStopping = kbappsv1.StoppingClusterPhase
	clusterStopped  = kbappsv1.StoppedClusterPhase
	clusterFailed   = kbappsv1.FailedClusterPhase
)

const (
	start opsType = iota + 1
	stop
)

type opsType int

func (o opsType) String() string {
	switch o {
	case start:
		return "start"
	case stop:
		return "stop"
	default:
		return fmt.Sprintf("unknown(%d)", int(o))
	}
}

// ClusterReconciler 负责根据 service_id 关联 KubeBlocks Component 并驱动 Cluster 的启停
//
// 该 Reconcile 判断 Cluster 是否存在 service_id 标签，如果存在则说明该 Cluster 由 Block Mechanica 管理
// 当 Cluster 由 Block Mechanica 管理时，判断是否存在匹配 service_id 的 KubeBlocks Component：
//
// - 如果不存在，则说明 KubeBlocks Component 被 Rainbond 关闭或是正在重启，需要使用 OpsRequest Stop Cluster，
//
// - 如果存在，则说明 KubeBlocks Component 正在运行，需要使用 OpsRequest Start Cluster。
type ClusterReconciler struct {
	client client.Client
	scheme *runtime.Scheme
	svc    service.Services
}

func NewClusterReconciler(c client.Client, s *runtime.Scheme, svcs service.Services) *ClusterReconciler {
	return &ClusterReconciler{client: c, scheme: s, svc: svcs}
}

func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kbappsv1.Cluster{}).
		Watches(
			&appsv1.Deployment{},
			r.mapDeploymentToAssociatedCluster(),
			builder.WithPredicates(r.onAssociationChange()),
		).
		WithOptions(controller.Options{MaxConcurrentReconciles: 2}).
		Complete(r)
}

func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.With(
		log.String("controller", "Cluster"),
		log.String("cluster", req.String()),
	)

	var cluster kbappsv1.Cluster
	if err := r.client.Get(ctx, req.NamespacedName, &cluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 仅处理带有 service_id 标签的 Cluster，这表示该 Cluster 由 Block Mechanica 管理
	svcID := cluster.GetLabels()[service.ServiceIDLabel]
	if svcID == "" {
		return ctrl.Result{}, nil
	}

	// 基于 service_id 查找匹配的 KubeBlocks Component
	// 如果存在则设置为 Start，否则设置为 Stop，确保 cluster 的状态与 KubeBlocks Component 一致
	var targetOps opsType
	if _, err := r.svc.GetKubeBlocksComponentByServiceID(ctx, svcID); err != nil {
		if !errors.Is(err, service.ErrTargetNotFound) {
			return ctrl.Result{}, err
		}
		targetOps = stop
	} else {
		targetOps = start
	}

	// 如果集群处于停止或正在停止状态，则需要进行额外操作
	switch cluster.Status.Phase {
	case clusterStopped, clusterStopping:
		if targetOps == stop {
			return ctrl.Result{}, nil
		}
	case clusterRunning, clusterCreating:
		if targetOps == start {
			return ctrl.Result{}, nil
		}
	}

	// targetOps 必然会为 stop 或 start
	err := func() error {
		switch targetOps {
		case start:
			return r.svc.StartCluster(ctx, &cluster)
		case stop:
			return r.svc.StopCluster(ctx, &cluster)
		default:
			return nil
		}
	}()
	isSkipped := errors.Is(err, service.ErrCreateOpsSkipped)
	if err != nil && !isSkipped {
		return ctrl.Result{}, fmt.Errorf("maintain lifecycle of cluster %s: %w", cluster.Name, err)
	}

	logger.Debug("maintained lifecycle of cluster",
		log.String("cluster", cluster.Name),
		log.Bool("skipped ops", isSkipped),
		log.String("ops type", targetOps.String()),
	)

	return ctrl.Result{}, nil
}

// mapDeploymentToAssociatedCluster 将 Deployment 通过 service_id 映射到对应的 Cluster
//
// 只有 KubeBlocks Component 会触发
func (r *ClusterReconciler) mapDeploymentToAssociatedCluster() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		if obj == nil {
			return nil
		}

		labels := obj.GetLabels()
		if labels == nil {
			return nil
		}

		svcID, ok := labels[service.ServiceIDLabel]
		if !ok || svcID == "" {
			return nil
		}

		cluster, err := r.svc.GetClusterByServiceID(ctx, svcID)
		if err != nil || cluster == nil {
			return nil
		}
		return []reconcile.Request{{NamespacedName: client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name}}}
	})
}

// onAssociationChange 只在可能的 KubeBlocks Component(带 service_id 标签的 Deployment)被创建或删除时触发
func (r *ClusterReconciler) onAssociationChange() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if e.Object == nil {
				return false
			}
			_, ok := e.Object.GetLabels()[service.ServiceIDLabel]
			return ok
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			if e.Object == nil {
				return false
			}
			_, ok := e.Object.GetLabels()[service.ServiceIDLabel]
			return ok
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

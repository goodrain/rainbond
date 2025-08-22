// Package controller 提供 Reconciler 的相关实现和集中注册入口
package controller

import (
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/furutachiKurea/block-mechanica/service"
)

// Register 集中注册本包内的所有 controller
func Register(mgr ctrl.Manager, svcs service.Services) error {
	client := mgr.GetClient()
	scheme := mgr.GetScheme()

	// KubeBlocksComponentReconciler
	if err := NewBlocksComponentReconciler(client, scheme, svcs).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setup KubeBlocksComponentReconciler: %w", err)
	}

	// ClusterReconciler
	if err := NewClusterReconciler(client, scheme, svcs).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setup ClusterReconciler: %w", err)
	}

	return nil
}

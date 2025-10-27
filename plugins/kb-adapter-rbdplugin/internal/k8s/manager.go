// Package k8s 负责与 Kubernetes controller-runtime 的集成与管理
package k8s

import (
	"context"
	"fmt"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	datav1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/furutachiKurea/block-mechanica/api"
	"github.com/furutachiKurea/block-mechanica/internal/config"
	"github.com/furutachiKurea/block-mechanica/internal/index"
	"github.com/furutachiKurea/block-mechanica/service"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var _scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(corev1.AddToScheme(_scheme))
	utilruntime.Must(storagev1.AddToScheme(_scheme))
	utilruntime.Must(kbappsv1.AddToScheme(_scheme))
	utilruntime.Must(datav1alpha1.AddToScheme(_scheme))
	utilruntime.Must(opsv1alpha1.AddToScheme(_scheme))
	utilruntime.Must(parametersv1alpha1.AddToScheme(_scheme))
	utilruntime.Must(appsv1.AddToScheme(_scheme))
	utilruntime.Must(workloadsv1.AddToScheme(_scheme))
}

// NewManager 创建 ctrl.Manager 实例
//
// 设置 logger 为 zap
func NewManager() (ctrl.Manager, error) {
	enableLeaderElection := !config.InDevelopment()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  _scheme,
		LeaderElection:          enableLeaderElection,
		LeaderElectionID:        "block-mechanica-leader-election",
		LeaderElectionNamespace: "rbd-system",
		Metrics: server.Options{
			BindAddress: ":9090",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create manager: %w", err)
	}

	return mgr, nil
}

// Setup 初始化 manage
func Setup(ctx context.Context, mgr ctrl.Manager, svcs service.Services) error {
	if err := index.Register(ctx, mgr); err != nil {
		return fmt.Errorf("register indexes: %w", err)
	}

	// 注册 API server runnable（由 Setup 统一收口）
	if err := api.RegisterServer(ctx, mgr, svcs); err != nil {
		return fmt.Errorf("register api server: %w", err)
	}

	return nil
}

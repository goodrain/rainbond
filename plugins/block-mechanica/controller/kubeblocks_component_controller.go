package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/furutachiKurea/block-mechanica/internal/log"
	"github.com/furutachiKurea/block-mechanica/service"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

// KubeBlocksComponentReconciler 负责为 KubeBlocks Component 幂等补齐容器 args。
//
// 该 Reconcile 监听 KubeBlocks Component 的变化，判断是否需要补齐容器的 args，
// 确保 args 的设置能正确的将连接转发至 KubeBlocks Cluster 的 service。
//
// 如果存在一个 Cluster，其 service_id label 与该 Deployment 的 label 相同，
// 则该 Deployment 为 KubeBlocks Component。
type KubeBlocksComponentReconciler struct {
	client client.Client
	scheme *runtime.Scheme
	svc    service.Services
}

func NewBlocksComponentReconciler(c client.Client, s *runtime.Scheme, svcs service.Services) *KubeBlocksComponentReconciler {
	return &KubeBlocksComponentReconciler{client: c, scheme: s, svc: svcs}
}

// SetupWithManager 注册 KubeBlocksComponentReconciler
func (r *KubeBlocksComponentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 2}).
		Complete(r)
}

func (r *KubeBlocksComponentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.With(
		log.String("controller", "KubeBlocks Component"),
		log.String("deployment", req.String()),
	)

	var deploy appsv1.Deployment
	if err := r.client.Get(ctx, req.NamespacedName, &deploy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	svcID := deploy.GetLabels()[service.ServiceIDLabel]
	if svcID == "" {
		return ctrl.Result{}, nil
	}
	cluster, err := r.svc.GetClusterByServiceID(ctx, svcID)
	if err != nil {
		// 并非 KubeBlocks Component
		if errors.Is(err, service.ErrTargetNotFound) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	clusterType := cluster.Spec.ClusterDef
	if !r.svc.IsLegalType(clusterType) {
		logger.Warn("KubeBlocks Component Found, but not supported",
			log.String("service_id", svcID),
			log.String("cluster_type", clusterType),
		)
		return ctrl.Result{}, nil
	}

	logger.Debug("KubeBlocks Component Founded",
		log.String("cluster", cluster.Name),
		log.String("service_id", svcID),
	)

	// 幂等 patch：确保目标容器存在 args 对应的字段
	if len(deploy.Spec.Template.Spec.Containers) == 0 {
		return ctrl.Result{}, nil
	}
	targetContainer := deploy.Spec.Template.Spec.Containers[0]

	// 监听端口：来自第一个容器的环境变量 _PORT
	// 目标地址：{cluster-name}-{cluster-def}.{namespace}.svc.cluster.local:{targetPort}
	listenPort, ok := findListenPortFromContainer(targetContainer)
	if !ok {
		return ctrl.Result{}, nil
	}

	targetPort := r.svc.GetTargetPort(clusterType)
	targetService := fmt.Sprintf("%s-%s.%s.svc.cluster.local:%d", cluster.Name, clusterType, cluster.Namespace, targetPort)
	newArgs := []string{
		fmt.Sprintf("TCP-LISTEN:%d,fork,reuseaddr", listenPort),
		fmt.Sprintf("TCP4:%s", targetService),
	}

	if stringSliceIsEqual(deploy.Spec.Template.Spec.Containers[0].Args, newArgs) {
		return ctrl.Result{}, nil
	}

	patchBody := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []map[string]any{
						{
							"name": targetContainer.Name,
							"args": newArgs,
						},
					},
				},
			},
		},
	}

	patchBytes, err := json.Marshal(patchBody)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("marshal patch: %w", err)
	}

	if err := r.client.Patch(ctx, &appsv1.Deployment{
		ObjectMeta: deploy.ObjectMeta,
	}, client.RawPatch(types.StrategicMergePatchType, patchBytes)); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch deployment args: %w", err)
	}

	logger.Info("Patched args for target container",
		log.Any("args", newArgs),
	)

	return ctrl.Result{}, nil
}

// findListenPortFromContainer 从环境变量中解析 _PORT 为监听端口
//
// 没有找到则说明 Rainbond 还没有为组件创建对外/内端口，应当继续等待
func findListenPortFromContainer(container corev1.Container) (int, bool) {
	for _, env := range container.Env {
		if port, ok := parsePortFromEnv(env); ok {
			return port, true
		}
	}
	return 0, false
}

func parsePortFromEnv(env corev1.EnvVar) (int, bool) {
	if env.Name != "_PORT" || env.Value == "" {
		return 0, false
	}

	var p int
	if _, err := fmt.Sscanf(env.Value, "%d", &p); err != nil || p <= 0 {
		return 0, false
	}
	return p, true
}

// stringSliceIsEqual 判断两个字符串切片是否相等
func stringSliceIsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

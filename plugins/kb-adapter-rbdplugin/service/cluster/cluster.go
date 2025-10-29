package cluster

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/index"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/log"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/model"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/service/kbkit"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	MiB = 1024 * 1024
	GiB = 1024 * 1024 * 1024
)

// Service 提供针对 Cluster 相关操作
type Service struct {
	client client.Client
}

func NewService(c client.Client) *Service {
	return &Service{
		client: c,
	}
}

// formatToISO8601Time 将标准 time.Time 转为 ISO 8601（RFC3339，UTC）字符串
func formatToISO8601Time(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// AssociateToKubeBlocksComponent 将 KubeBlocks 组件和 Cluster 通过 service_id 关联
func (s *Service) associateToKubeBlocksComponent(ctx context.Context, cluster *kbappsv1.Cluster, serviceID string) error {
	log.Debug("start associate cluster to rainbond component",
		log.String("service_id", serviceID),
		log.String("cluster", cluster.Name),
	)

	const labelServiceID = index.ServiceIDLabel

	err := wait.PollUntilContextCancel(ctx, 500*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		var latest kbappsv1.Cluster
		if err := s.client.Get(ctx, client.ObjectKey{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
		}, &latest); err != nil {
			log.Debug("Cluster not found yet, waiting",
				log.String("cluster", cluster.Name),
				log.String("namespace", cluster.Namespace),
			)
			return false, nil
		}

		if latest.Labels != nil && latest.Labels[labelServiceID] == serviceID {
			log.Debug("Cluster already has correct service_id label",
				log.String("service_id", serviceID),
			)
			return true, nil
		}

		patchData := fmt.Sprintf(`{
			"metadata": {
				"labels": {
					"%s": "%s"
				}
			}
		}`, labelServiceID, serviceID)

		if err := s.client.Patch(ctx, &kbappsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cluster.Name,
				Namespace: cluster.Namespace,
			},
		}, client.RawPatch(types.MergePatchType, []byte(patchData))); err != nil {
			log.Debug("Patch operation failed, retrying",
				log.String("cluster", cluster.Name),
				log.Err(err),
			)
			return false, nil
		}

		log.Debug("Successfully added service_id label to cluster",
			log.String("service_id", serviceID),
			log.String("cluster", cluster.Name),
		)
		return true, nil
	})

	if err != nil {
		return fmt.Errorf("failed to associate cluster %s/%s with service_id label after retries: %w", cluster.Namespace, cluster.Name, err)
	}

	log.Info("Associated KubeBlocks Cluster to Rainbond component",
		log.String("service_id", serviceID),
		log.String("cluster", cluster.Name),
	)

	return nil
}

// getClusterPods 获取 Cluster 相关的 Pod 状态信息,
// 会将 Cluster 的多个组件的 Pod 状态信息合并返回
func (s *Service) getClusterPods(ctx context.Context, cluster *kbappsv1.Cluster) ([]model.Status, error) {
	if len(cluster.Spec.ComponentSpecs) == 0 {
		return nil, fmt.Errorf("cluster %s/%s has no componentSpecs", cluster.Namespace, cluster.Name)
	}

	var (
		namespace   = cluster.Namespace
		clusterName = cluster.Name
	)

	var (
		// podComponent 用于记录 Pod 所属的 Component 名称
		podComponent = make(map[string]string)
		// podNames 用于记录 Pod 名称列表
		podNames = make([]string, 0)
	)

	for _, component := range cluster.Spec.ComponentSpecs {
		componentName := component.Name
		// 如果为空，则跳过，应该不会出现这种情况
		if componentName == "" {
			log.Warn("Component name is empty, skip",
				log.String("cluster", clusterName),
				log.String("component", componentName),
			)
			continue
		}

		instanceSet, err := getInstanceSetByCluster(ctx, s.client, clusterName, namespace, componentName)
		if err != nil {
			if errors.Is(err, kbkit.ErrTargetNotFound) {
				log.Info("InstanceSet not found, skip component",
					log.String("cluster", clusterName),
					log.String("component", componentName))
				continue
			}
			return nil, fmt.Errorf("get instanceset for component %s: %w", componentName, err)
		}

		for _, instanceStatus := range instanceSet.Status.InstanceStatus {
			if instanceStatus.PodName == "" {
				continue
			}
			// 如果 Pod 名称已经存在，则跳过应该也不会出现这种情况
			if _, exists := podComponent[instanceStatus.PodName]; exists {
				continue
			}

			podComponent[instanceStatus.PodName] = componentName
			podNames = append(podNames, instanceStatus.PodName)
		}
	}

	if len(podNames) == 0 {
		return []model.Status{}, nil
	}

	pods, err := getPodsByNames(ctx, s.client, podNames, namespace)
	if err != nil {
		return nil, fmt.Errorf("get pods by names: %w", err)
	}

	result := make([]model.Status, 0, len(pods))
	for _, pod := range pods {
		componentName := podComponent[pod.Name]
		if componentName == "" {
			componentName = pod.Labels["apps.kubeblocks.io/component-name"]
		}
		result = append(result, buildPodStatus(pod, componentName))
	}

	return result, nil
}

// getPodsByNames 根据 Pod 名称列表查询 Pod
func getPodsByNames(ctx context.Context, c client.Client, podNames []string, namespace string) ([]corev1.Pod, error) {
	var pods []corev1.Pod

	for _, podName := range podNames {
		var pod corev1.Pod
		if err := c.Get(ctx, client.ObjectKey{Name: podName, Namespace: namespace}, &pod); err != nil {
			log.Warn("Failed to get pod", log.String("pod", podName), log.Err(err))
			continue
		}
		pods = append(pods, pod)
	}

	return pods, nil
}

// buildPodStatus 构建 Pod 状态信息
func buildPodStatus(pod corev1.Pod, componentName string) model.Status {
	ready := false

	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			ready = true
			break
		}
	}

	return model.Status{
		Name:       pod.Name,
		Component:  componentName,
		Status:     pod.Status.Phase,
		Ready:      ready,
		Containers: buildReplicaContainers(&pod),
	}
}

// buildReplicaContainers build []ReplicaContainer
func buildReplicaContainers(pod *corev1.Pod) []model.ReplicaContainer {
	if len(pod.Spec.Containers) == 0 {
		return nil
	}

	containers := make([]model.ReplicaContainer, 0, len(pod.Spec.Containers))
	for _, container := range pod.Spec.Containers {
		containers = append(containers, model.ReplicaContainer{
			Name: container.Name,
		})
	}

	return containers
}

// getInstanceSetByCluster 通过 cluster 和 component 获取 InstanceSet
func getInstanceSetByCluster(
	ctx context.Context,
	c client.Client,
	clusterName,
	namespace,
	componentName string,
) (*workloadsv1.InstanceSet, error) {
	var instanceSetList workloadsv1.InstanceSetList

	// 优先使用索引查询
	indexKey := fmt.Sprintf("%s/%s/%s", namespace, clusterName, componentName)
	if err := c.List(ctx, &instanceSetList, client.MatchingFields{index.NamespaceClusterComponentField: indexKey}); err == nil {
		switch len(instanceSetList.Items) {
		case 0:
			return nil, kbkit.ErrTargetNotFound
		case 1:
			return &instanceSetList.Items[0], nil
		default:
			return nil, kbkit.ErrMultipleFounded
		}
	} else {
		log.Warn("Index query failed, falling back to label query",
			log.String("indexKey", indexKey), log.Err(err))
	}

	// 回退到标签查询
	selector := client.MatchingLabels{
		constant.AppInstanceLabelKey:        clusterName,
		"apps.kubeblocks.io/component-name": componentName,
	}
	if err := c.List(ctx, &instanceSetList, selector, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list instanceset for cluster %s component %s: %w", clusterName, componentName, err)
	}

	switch len(instanceSetList.Items) {
	case 0:
		return nil, kbkit.ErrTargetNotFound
	case 1:
		return &instanceSetList.Items[0], nil
	default:
		return nil, kbkit.ErrMultipleFounded
	}
}

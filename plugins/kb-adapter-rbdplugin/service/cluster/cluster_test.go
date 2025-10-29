package cluster

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/index"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/model"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/testutil"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/service/kbkit"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestAssociateToKubeBlocksComponent(t *testing.T) {
	tests := []struct {
		name          string
		setup         func() (client.Client, *kbappsv1.Cluster)
		serviceID     string
		expectError   bool
		errorContains string
		verify        func(t *testing.T, client client.Client, clusterName, namespace string)
	}{
		{
			name: "successful_association_new_label",
			setup: func() (client.Client, *kbappsv1.Cluster) {
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithPhase(kbappsv1.RunningClusterPhase).
					Build()

				client := testutil.NewFakeClientWithIndexes(cluster)
				return client, cluster
			},
			serviceID:   "test-service-123",
			expectError: false,
			verify: func(t *testing.T, c client.Client, clusterName, namespace string) {
				cluster := &kbappsv1.Cluster{}
				err := c.Get(context.Background(), types.NamespacedName{
					Name: clusterName, Namespace: namespace,
				}, cluster)
				require.NoError(t, err)
				assert.Equal(t, "test-service-123", cluster.Labels[index.ServiceIDLabel])
			},
		},
		{
			name: "label_already_exists_correct_value",
			setup: func() (client.Client, *kbappsv1.Cluster) {
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID("test-service-123"). // 已经有正确的标签
					WithPhase(kbappsv1.RunningClusterPhase).
					Build()

				client := testutil.NewFakeClientWithIndexes(cluster)
				return client, cluster
			},
			serviceID:   "test-service-123",
			expectError: false,
			verify: func(t *testing.T, c client.Client, clusterName, namespace string) {
				cluster := &kbappsv1.Cluster{}
				err := c.Get(context.Background(), types.NamespacedName{
					Name: clusterName, Namespace: namespace,
				}, cluster)
				require.NoError(t, err)
				assert.Equal(t, "test-service-123", cluster.Labels[index.ServiceIDLabel])
			},
		},
		{
			name: "get_operation_fails",
			setup: func() (client.Client, *kbappsv1.Cluster) {
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithPhase(kbappsv1.RunningClusterPhase).
					Build()

				client := testutil.NewErrorClientBuilder(cluster).
					WithGetError(errors.New("network error")).
					Build()

				return client, cluster
			},
			serviceID:     "test-service-123",
			expectError:   true,
			errorContains: "failed to associate cluster",
		},
		{
			name: "patch_operation_fails",
			setup: func() (client.Client, *kbappsv1.Cluster) {
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithPhase(kbappsv1.RunningClusterPhase).
					Build()

				client := testutil.NewErrorClientBuilder(cluster).
					WithPatchError(errors.New("patch failed")).
					Build()

				return client, cluster
			},
			serviceID:     "test-service-123",
			expectError:   true,
			errorContains: "failed to associate cluster",
		},
		{
			name: "context_timeout",
			setup: func() (client.Client, *kbappsv1.Cluster) {
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithPhase(kbappsv1.RunningClusterPhase).
					Build()

				client := testutil.NewFakeClientWithIndexes()
				return client, cluster
			},
			serviceID:     "test-service-123",
			expectError:   true,
			errorContains: "failed to associate cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cluster := tt.setup()
			service := NewService(client)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := service.associateToKubeBlocksComponent(ctx, cluster, tt.serviceID)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				if tt.verify != nil {
					tt.verify(t, client, cluster.Name, cluster.Namespace)
				}
			}
		})
	}
}

func TestGetClusterPods(t *testing.T) {
	tests := []struct {
		name          string
		setup         func() (client.Client, *kbappsv1.Cluster)
		expectError   bool
		errorContains string
		expectPods    int
		verifyPods    func(t *testing.T, pods []model.Status)
	}{
		{
			name: "empty_component_specs",
			setup: func() (client.Client, *kbappsv1.Cluster) {
				// 创建没有 ComponentSpecs 的集群
				cluster := &kbappsv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "empty-cluster",
						Namespace: testutil.TestNamespace,
					},
					Spec: kbappsv1.ClusterSpec{
						ClusterDef:     "mysql",
						ComponentSpecs: []kbappsv1.ClusterComponentSpec{}, // 空的
					},
				}

				client := testutil.NewFakeClientWithIndexes(cluster)
				return client, cluster
			},
			expectError:   true,
			errorContains: "has no componentSpecs",
		},
		{
			name: "single_component_with_pods",
			setup: func() (client.Client, *kbappsv1.Cluster) {
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithPhase(kbappsv1.RunningClusterPhase).
					Build()

				// 创建 InstanceSet
				instanceSet := testutil.NewInstanceSetBuilder("test-cluster-mysql", testutil.TestNamespace).
					WithClusterInstance("test-cluster").
					WithComponentName("mysql").
					WithInstanceStatus("test-cluster-mysql-0", "test-cluster-mysql-1").
					WithReplicas(2).
					Build()

				// 创建 Pod
				pod1 := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster-mysql-0",
						Namespace: testutil.TestNamespace,
						Labels: map[string]string{
							"apps.kubeblocks.io/component-name": "mysql",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "mysql", Image: "mysql:8.0"},
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionTrue},
						},
					},
				}

				pod2 := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster-mysql-1",
						Namespace: testutil.TestNamespace,
						Labels: map[string]string{
							"apps.kubeblocks.io/component-name": "mysql",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "mysql", Image: "mysql:8.0"},
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionFalse},
						},
					},
				}

				client := testutil.NewFakeClientWithIndexes(cluster, instanceSet, pod1, pod2)
				return client, cluster
			},
			expectError: false,
			expectPods:  2,
			verifyPods: func(t *testing.T, pods []model.Status) {
				require.NotEmpty(t, pods)
				assert.Equal(t, []model.ReplicaContainer{{Name: "mysql"}}, pods[0].Containers)
			},
		},
		{
			name: "multiple_components",
			setup: func() (client.Client, *kbappsv1.Cluster) {
				cluster := testutil.NewClusterBuilder("multi-cluster", testutil.TestNamespace).
					WithClusterDef("redis").
					WithComponent("redis", "redis-7.0").
					WithComponent("redis-sentinel", "redis-sentinel-7.0").
					WithPhase(kbappsv1.RunningClusterPhase).
					Build()

				// Redis 组件的 InstanceSet 和 Pod
				redisInstanceSet := testutil.NewInstanceSetBuilder("multi-cluster-redis", testutil.TestNamespace).
					WithClusterInstance("multi-cluster").
					WithComponentName("redis").
					WithInstanceStatus("multi-cluster-redis-0").
					Build()

				redisPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "multi-cluster-redis-0",
						Namespace: testutil.TestNamespace,
						Labels: map[string]string{
							"apps.kubeblocks.io/component-name": "redis",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "redis", Image: "redis:7.0"},
						},
					},
					Status: corev1.PodStatus{Phase: corev1.PodRunning},
				}

				// Sentinel 组件的 InstanceSet 和 Pod
				sentinelInstanceSet := testutil.NewInstanceSetBuilder("multi-cluster-redis-sentinel", testutil.TestNamespace).
					WithClusterInstance("multi-cluster").
					WithComponentName("redis-sentinel").
					WithInstanceStatus("multi-cluster-redis-sentinel-0").
					Build()

				sentinelPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "multi-cluster-redis-sentinel-0",
						Namespace: testutil.TestNamespace,
						Labels: map[string]string{
							"apps.kubeblocks.io/component-name": "redis-sentinel",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "sentinel", Image: "sentinel:7.0"},
						},
					},
					Status: corev1.PodStatus{Phase: corev1.PodRunning},
				}

				client := testutil.NewFakeClientWithIndexes(
					cluster, redisInstanceSet, sentinelInstanceSet, redisPod, sentinelPod)
				return client, cluster
			},
			expectError: false,
			expectPods:  2,
			verifyPods: func(t *testing.T, pods []model.Status) {
				require.Len(t, pods, 2)
				assert.Equal(t, []model.ReplicaContainer{{Name: "redis"}}, pods[0].Containers)
				assert.Equal(t, []model.ReplicaContainer{{Name: "sentinel"}}, pods[1].Containers)
			},
		},
		{
			name: "instanceset_not_found",
			setup: func() (client.Client, *kbappsv1.Cluster) {
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithPhase(kbappsv1.RunningClusterPhase).
					Build()

				// 只创建集群，不创建 InstanceSet
				client := testutil.NewFakeClientWithIndexes(cluster)
				return client, cluster
			},
			expectError: false,
			expectPods:  0, // InstanceSet 不存在时返回空列表
		},
		{
			name: "pod_not_found",
			setup: func() (client.Client, *kbappsv1.Cluster) {
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithPhase(kbappsv1.RunningClusterPhase).
					Build()

				// 创建 InstanceSet 但不创建 Pod
				instanceSet := testutil.NewInstanceSetBuilder("test-cluster-mysql", testutil.TestNamespace).
					WithClusterInstance("test-cluster").
					WithComponentName("mysql").
					WithInstanceStatus("test-cluster-mysql-0", "test-cluster-mysql-1").
					Build()

				client := testutil.NewFakeClientWithIndexes(cluster, instanceSet)
				return client, cluster
			},
			expectError: false,
			expectPods:  0, // Pod 不存在时返回空列表
		},
		{
			name: "api_error_on_instanceset_query",
			setup: func() (client.Client, *kbappsv1.Cluster) {
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithPhase(kbappsv1.RunningClusterPhase).
					Build()

				client := testutil.NewErrorClientBuilder(cluster).
					WithListError(errors.New("api server error")).
					Build()
				return client, cluster
			},
			expectError:   true,
			errorContains: "get instanceset for component",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cluster := tt.setup()
			service := NewService(client)

			pods, err := service.getClusterPods(context.Background(), cluster)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.Len(t, pods, tt.expectPods)

				// 验证 Pod 状态结构
				for _, pod := range pods {
					// 由于我们不能导入 model 包，这里只做基本的断言
					assert.NotNil(t, pod)
				}

				if tt.verifyPods != nil {
					tt.verifyPods(t, pods)
				}
			}
		})
	}
}

func TestGetInstanceSetByCluster(t *testing.T) {
	tests := []struct {
		name          string
		setup         func() client.Client
		clusterName   string
		namespace     string
		componentName string
		expectError   bool
		errorType     error // 预期的错误类型
		verify        func(t *testing.T, instanceSet *workloadsv1.InstanceSet)
	}{
		{
			name: "index_query_success",
			setup: func() client.Client {
				instanceSet := testutil.NewInstanceSetBuilder("test-cluster-mysql", testutil.TestNamespace).
					WithClusterInstance("test-cluster").
					WithComponentName("mysql").
					Build()
				return testutil.NewFakeClientWithIndexes(instanceSet)
			},
			clusterName:   "test-cluster",
			namespace:     testutil.TestNamespace,
			componentName: "mysql",
			expectError:   false,
			verify: func(t *testing.T, instanceSet *workloadsv1.InstanceSet) {
				assert.Equal(t, "test-cluster-mysql", instanceSet.Name)
				assert.Equal(t, "test-cluster", instanceSet.Labels["app.kubernetes.io/instance"])
				assert.Equal(t, "mysql", instanceSet.Labels["apps.kubeblocks.io/component-name"])
			},
		},
		{
			name: "index_query_not_found",
			setup: func() client.Client {
				// 不创建任何 InstanceSet
				return testutil.NewFakeClientWithIndexes()
			},
			clusterName:   "non-existent-cluster",
			namespace:     testutil.TestNamespace,
			componentName: "mysql",
			expectError:   true,
			errorType:     kbkit.ErrTargetNotFound,
		},
		{
			name: "multiple_instancesets_found",
			setup: func() client.Client {
				// 创建两个相同标签的 InstanceSet（理论上不应该发生）
				instanceSet1 := testutil.NewInstanceSetBuilder("test-cluster-mysql-1", testutil.TestNamespace).
					WithClusterInstance("test-cluster").
					WithComponentName("mysql").
					Build()
				instanceSet2 := testutil.NewInstanceSetBuilder("test-cluster-mysql-2", testutil.TestNamespace).
					WithClusterInstance("test-cluster").
					WithComponentName("mysql").
					Build()

				return testutil.NewFakeClientWithIndexes(instanceSet1, instanceSet2)
			},
			clusterName:   "test-cluster",
			namespace:     testutil.TestNamespace,
			componentName: "mysql",
			expectError:   true,
			errorType:     kbkit.ErrMultipleFounded,
		},
		{
			name: "api_error_on_list",
			setup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithListError(errors.New("api server error")).
					Build()
			},
			clusterName:   "test-cluster",
			namespace:     testutil.TestNamespace,
			componentName: "mysql",
			expectError:   true,
			errorType:     nil, // API 错误是包装后的错误，不检查特定类型
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setup()

			instanceSet, err := getInstanceSetByCluster(
				context.Background(), client, tt.clusterName, tt.namespace, tt.componentName)

			if tt.expectError {
				require.Error(t, err)
				// 如果指定了错误类型，验证错误类型
				if tt.errorType != nil {
					assert.ErrorIs(t, err, tt.errorType)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, instanceSet)
				if tt.verify != nil {
					tt.verify(t, instanceSet)
				}
			}
		})
	}
}

func TestGetPodsByNames(t *testing.T) {
	tests := []struct {
		name          string
		setup         func() client.Client
		podNames      []string
		namespace     string
		expectPods    int
		expectError   bool
		errorContains string
	}{
		{
			name: "all_pods_exist",
			setup: func() client.Client {
				pod1 := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-1",
						Namespace: testutil.TestNamespace,
					},
					Status: corev1.PodStatus{Phase: corev1.PodRunning},
				}
				pod2 := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-2",
						Namespace: testutil.TestNamespace,
					},
					Status: corev1.PodStatus{Phase: corev1.PodRunning},
				}
				return testutil.NewFakeClient(pod1, pod2)
			},
			podNames:    []string{"pod-1", "pod-2"},
			namespace:   testutil.TestNamespace,
			expectPods:  2,
			expectError: false,
		},
		{
			name: "partial_pods_exist",
			setup: func() client.Client {
				pod1 := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-1",
						Namespace: testutil.TestNamespace,
					},
					Status: corev1.PodStatus{Phase: corev1.PodRunning},
				}
				return testutil.NewFakeClient(pod1)
			},
			podNames:    []string{"pod-1", "pod-2", "pod-3"},
			namespace:   testutil.TestNamespace,
			expectPods:  1, // 只有 pod-1 存在
			expectError: false,
		},
		{
			name: "no_pods_exist",
			setup: func() client.Client {
				return testutil.NewFakeClient()
			},
			podNames:    []string{"pod-1", "pod-2"},
			namespace:   testutil.TestNamespace,
			expectPods:  0,
			expectError: false,
		},
		{
			name: "empty_pod_names",
			setup: func() client.Client {
				return testutil.NewFakeClient()
			},
			podNames:    []string{},
			namespace:   testutil.TestNamespace,
			expectPods:  0,
			expectError: false,
		},
		{
			name: "some_pods_not_found_continues",
			setup: func() client.Client {
				pod1 := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-1",
						Namespace: testutil.TestNamespace,
					},
					Status: corev1.PodStatus{Phase: corev1.PodRunning},
				}
				// 只创建 pod-1，pod-2 不存在，测试容错能力
				return testutil.NewFakeClient(pod1)
			},
			podNames:    []string{"pod-1", "pod-2"},
			namespace:   testutil.TestNamespace,
			expectPods:  1, // pod-1 可以获取，pod-2 不存在但继续处理
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setup()

			pods, err := getPodsByNames(context.Background(), client, tt.podNames, tt.namespace)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.Len(t, pods, tt.expectPods)

				// 验证返回的 Pod 都是有效的
				for _, pod := range pods {
					assert.NotEmpty(t, pod.Name)
					assert.Equal(t, tt.namespace, pod.Namespace)
				}
			}
		})
	}
}

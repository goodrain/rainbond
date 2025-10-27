package cluster

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/furutachiKurea/block-mechanica/internal/model"
	"github.com/furutachiKurea/block-mechanica/internal/testutil"
	"github.com/furutachiKurea/block-mechanica/service/kbkit"

	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGetPodDetail(t *testing.T) {
	type expectation struct {
		wantErr  error
		errorMsg string
		check    func(t *testing.T, detail *model.PodDetail)
	}

	tests := []struct {
		name        string
		serviceID   string
		podName     string
		objects     func() []client.Object
		expectation expectation
	}{
		{
			name:      "success_with_instanceset_metadata",
			serviceID: "svc-success",
			podName:   "redis-sentinel-0",
			objects: func() []client.Object {
				componentName := "redis-sentinel"
				componentDef := "redis-sentinel-7-1.0.0"
				cluster := testutil.NewClusterBuilder("redis", testutil.TestNamespace).
					WithServiceID("svc-success").
					WithComponent(componentName, componentDef).
					WithComponentServiceVersion(componentName, "7.2.7").
					Build()

				instanceSet := &workloadsv1.InstanceSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redis-sentinel",
						Namespace: testutil.TestNamespace,
						Labels: map[string]string{
							"app.kubernetes.io/instance":        cluster.Name,
							"apps.kubeblocks.io/component-name": componentName,
						},
						Annotations: map[string]string{
							"app.kubernetes.io/component":        componentDef,
							"apps.kubeblocks.io/service-version": "7.2.7",
						},
					},
					Status: workloadsv1.InstanceSetStatus{
						InstanceStatus: []workloadsv1.InstanceStatus{{PodName: "redis-sentinel-0"}},
					},
				}

				start := metav1.NewTime(time.Unix(1700000000, 0))
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redis-sentinel-0",
						Namespace: testutil.TestNamespace,
						Labels: map[string]string{
							"apps.kubeblocks.io/component-name": componentName,
							"workloads.kubeblocks.io/instance":  instanceSet.Name,
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: componentName,
								Resources: corev1.ResourceRequirements{
									Limits: testutil.Resources("1", "1Gi"),
								},
							},
							{
								Name: "sidecar",
							},
						},
					},
					Status: corev1.PodStatus{
						Phase:     corev1.PodRunning,
						HostIP:    "10.0.0.1",
						PodIP:     "10.0.0.2",
						StartTime: &start,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Name:  componentName,
								State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: start}},
							},
							{
								Name:  "sidecar",
								State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: start}},
							},
						},
					},
				}

				var events []client.Object
				now := time.Now()
				for i := 0; i < 12; i++ {
					ts := metav1.NewTime(now.Add(time.Duration(-i) * time.Minute))
					events = append(events, &corev1.Event{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "event" + fmt.Sprint(i),
							Namespace: testutil.TestNamespace,
						},
						InvolvedObject: corev1.ObjectReference{
							Kind:      "Pod",
							Name:      pod.Name,
							Namespace: pod.Namespace,
						},
						FirstTimestamp: ts,
						Type:           "Normal",
						Reason:         fmt.Sprintf("reason-%d", i),
						Message:        fmt.Sprintf("message-%d", i),
					})
				}

				objects := []client.Object{cluster, instanceSet, pod}
				objects = append(objects, events...)
				return objects
			},
			expectation: expectation{
				check: func(t *testing.T, detail *model.PodDetail) {
					require.NotNil(t, detail)
					assert.Equal(t, "redis-sentinel-0", detail.Name)
					assert.Equal(t, "10.0.0.1", detail.NodeIP)
					assert.Equal(t, "10.0.0.2", detail.IP)
					assert.Equal(t, "7.2.7", detail.Version)
					require.Len(t, detail.Containers, 1)
					container := detail.Containers[0]
					assert.Equal(t, "redis-sentinel-7-1.0.0", container.ComponentDef)
					assert.Equal(t, "1Gi", container.LimitMemory)
					assert.Equal(t, "1", container.LimitCPU)
					assert.Equal(t, "Running", container.State)
					require.Len(t, detail.Events, 10)
					assert.Equal(t, "reason-0", detail.Events[0].Reason)
					assert.Equal(t, "message-0", detail.Events[0].Message)
					assert.Equal(t, "reason-9", detail.Events[9].Reason)
					assert.NotEmpty(t, detail.Events[0].Age)
				},
			},
		},
		{
			name:      "success_fallback_to_spec_service_version",
			serviceID: "svc-fallback-version",
			podName:   "pg-0",
			objects: func() []client.Object {
				componentName := "postgresql"
				cluster := testutil.NewClusterBuilder("pg", testutil.TestNamespace).
					WithServiceID("svc-fallback-version").
					WithComponent(componentName, "").
					WithComponentServiceVersion(componentName, "14.6").
					Build()

				instanceSet := &workloadsv1.InstanceSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pg",
						Namespace: testutil.TestNamespace,
						Labels: map[string]string{
							"app.kubernetes.io/instance":        cluster.Name,
							"apps.kubeblocks.io/component-name": componentName,
						},
						Annotations: map[string]string{
							"app.kubernetes.io/component": "postgresql-14-def",
						},
					},
					Status: workloadsv1.InstanceSetStatus{
						InstanceStatus: []workloadsv1.InstanceStatus{{PodName: "pg-0"}},
					},
				}

				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pg-0",
						Namespace: testutil.TestNamespace,
						Labels: map[string]string{
							"apps.kubeblocks.io/component-name": componentName,
							"workloads.kubeblocks.io/instance":  instanceSet.Name,
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: componentName}},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodPending,
						ContainerStatuses: []corev1.ContainerStatus{{
							Name: componentName,
							State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
								Reason:  "ImagePullBackOff",
								Message: "pulling image",
							}},
						}},
					},
				}

				event := &corev1.Event{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pg-event",
						Namespace: testutil.TestNamespace,
					},
					InvolvedObject: corev1.ObjectReference{
						Kind:      "Pod",
						Name:      pod.Name,
						Namespace: pod.Namespace,
					},
					Message: "pod waiting",
				}

				return []client.Object{cluster, instanceSet, pod, event}
			},
			expectation: expectation{
				check: func(t *testing.T, detail *model.PodDetail) {
					require.NotNil(t, detail)
					assert.Equal(t, "14.6", detail.Version)
					assert.Equal(t, "pending", detail.Status.TypeStr)
					assert.Equal(t, "ImagePullBackOff", detail.Status.Reason)
					assert.Equal(t, "pulling image", detail.Status.Message)
					assert.Equal(t, "ImagePullError", detail.Status.Advice)
					require.Len(t, detail.Events, 1)
					assert.Empty(t, detail.Events[0].Age)
				},
			},
		},
		{
			name:      "success_component_name_from_instanceset",
			serviceID: "svc-component-name",
			podName:   "mysql-0",
			objects: func() []client.Object {
				componentName := "mysql"
				cluster := testutil.NewClusterBuilder("mysql", testutil.TestNamespace).
					WithServiceID("svc-component-name").
					WithComponent(componentName, "mysql-8.0").
					Build()

				instanceSet := &workloadsv1.InstanceSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysql",
						Namespace: testutil.TestNamespace,
						Labels: map[string]string{
							"app.kubernetes.io/instance":        cluster.Name,
							"apps.kubeblocks.io/component-name": componentName,
						},
						Annotations: map[string]string{
							"app.kubernetes.io/component": componentName + "-def",
						},
					},
					Status: workloadsv1.InstanceSetStatus{
						InstanceStatus: []workloadsv1.InstanceStatus{{PodName: "mysql-0"}},
					},
				}

				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysql-0",
						Namespace: testutil.TestNamespace,
						Labels: map[string]string{
							"workloads.kubeblocks.io/instance": instanceSet.Name,
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: componentName}},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						ContainerStatuses: []corev1.ContainerStatus{{
							Name:  componentName,
							State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.Now()}},
						}},
					},
				}

				return []client.Object{cluster, instanceSet, pod}
			},
			expectation: expectation{
				check: func(t *testing.T, detail *model.PodDetail) {
					require.NotNil(t, detail)
					assert.Equal(t, "mysql-8.0", detail.Version)
					require.Len(t, detail.Containers, 1)
					assert.Equal(t, "mysql-def", detail.Containers[0].ComponentDef)
				},
			},
		},
		{
			name:      "target_pod_not_found",
			serviceID: "svc-missing-pod",
			podName:   "missing",
			objects: func() []client.Object {
				componentName := "redis"
				cluster := testutil.NewClusterBuilder("rediscluster", testutil.TestNamespace).
					WithServiceID("svc-missing-pod").
					WithComponent(componentName, "redis-def").
					Build()

				instanceSet := &workloadsv1.InstanceSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redis",
						Namespace: testutil.TestNamespace,
						Labels: map[string]string{
							"app.kubernetes.io/instance":        cluster.Name,
							"apps.kubeblocks.io/component-name": componentName,
						},
					},
					Status: workloadsv1.InstanceSetStatus{
						InstanceStatus: []workloadsv1.InstanceStatus{{PodName: "redis-0"}},
					},
				}

				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redis-0",
						Namespace: testutil.TestNamespace,
					},
				}

				return []client.Object{cluster, instanceSet, pod}
			},
			expectation: expectation{wantErr: kbkit.ErrTargetNotFound},
		},
		{
			name:      "missing_component_spec_error",
			serviceID: "svc-no-spec",
			podName:   "rogue-0",
			objects: func() []client.Object {
				cluster := testutil.NewClusterBuilder("rogue", testutil.TestNamespace).
					WithServiceID("svc-no-spec").
					WithComponent("valid", "valid-def").
					Build()

				instanceSet := &workloadsv1.InstanceSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "valid",
						Namespace: testutil.TestNamespace,
						Labels: map[string]string{
							"app.kubernetes.io/instance":        cluster.Name,
							"apps.kubeblocks.io/component-name": "valid",
						},
					},
					Status: workloadsv1.InstanceSetStatus{
						InstanceStatus: []workloadsv1.InstanceStatus{{PodName: "rogue-0"}},
					},
				}

				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rogue-0",
						Namespace: testutil.TestNamespace,
						Labels: map[string]string{
							"apps.kubeblocks.io/component-name": "rogue",
							"workloads.kubeblocks.io/instance":  instanceSet.Name,
						},
					},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "rogue"}}},
					Status: corev1.PodStatus{
						ContainerStatuses: []corev1.ContainerStatus{{Name: "rogue"}},
					},
				}

				return []client.Object{cluster, instanceSet, pod}
			},
			expectation: expectation{errorMsg: "component spec rogue not found"},
		},
		{
			name:      "missing_component_definition_error",
			serviceID: "svc-no-def",
			podName:   "node-0",
			objects: func() []client.Object {
				componentName := "node"
				cluster := testutil.NewClusterBuilder("node", testutil.TestNamespace).
					WithServiceID("svc-no-def").
					WithComponent(componentName, "").
					Build()

				// 清空 serviceVersion
				for i := range cluster.Spec.ComponentSpecs {
					cluster.Spec.ComponentSpecs[i].ServiceVersion = ""
				}

				instanceSet := &workloadsv1.InstanceSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node",
						Namespace: testutil.TestNamespace,
						Labels: map[string]string{
							"app.kubernetes.io/instance":        cluster.Name,
							"apps.kubeblocks.io/component-name": componentName,
						},
					},
					Status: workloadsv1.InstanceSetStatus{
						InstanceStatus: []workloadsv1.InstanceStatus{{PodName: "node-0"}},
					},
				}

				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node-0",
						Namespace: testutil.TestNamespace,
						Labels: map[string]string{
							"apps.kubeblocks.io/component-name": componentName,
							"workloads.kubeblocks.io/instance":  instanceSet.Name,
						},
					},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: componentName}}},
					Status: corev1.PodStatus{
						ContainerStatuses: []corev1.ContainerStatus{{Name: componentName}},
					},
				}

				return []client.Object{cluster, instanceSet, pod}
			},
			expectation: expectation{errorMsg: "component definition missing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := tt.objects()
			client := testutil.NewFakeClientWithIndexes(objects...)

			svc := &Service{client: client}
			detail, err := svc.GetPodDetail(context.Background(), tt.serviceID, tt.podName)

			if tt.expectation.wantErr != nil || tt.expectation.errorMsg != "" {
				require.Error(t, err)
				if tt.expectation.wantErr != nil {
					assert.ErrorIs(t, err, tt.expectation.wantErr)
				}
				if tt.expectation.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.expectation.errorMsg)
				}
				return
			}

			require.NoError(t, err)
			if tt.expectation.check != nil {
				tt.expectation.check(t, detail)
			}
		})
	}
}

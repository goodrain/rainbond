package handler

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// capability_id: rainbond.cluster.rbd-component-readiness
func TestPodReadyForOverview(t *testing.T) {
	tests := []struct {
		name  string
		pod   corev1.Pod
		ready bool
	}{
		{name: "ready", pod: podWithReadyCondition(corev1.ConditionTrue), ready: true},
		{name: "running but not ready", pod: podWithReadyCondition(corev1.ConditionFalse), ready: false},
		{name: "no ready condition", pod: corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning}}, ready: false},
		{name: "succeeded", pod: corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodSucceeded}}, ready: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.ready, podReadyForOverview(tt.pod))
		})
	}
}

func TestBuildRainbondComponentOverviewSortsByName(t *testing.T) {
	got := buildRainbondComponentOverview([]corev1.Pod{
		overviewPod("rbd-worker", true),
		overviewPod("rbd-api", true),
		overviewPod("rbd-worker", false),
	})

	require.Len(t, got, 2)
	assert.Equal(t, "rbd-api", got[0].Name)
	assert.Equal(t, 1, got[0].RunPods)
	assert.Equal(t, 1, got[0].AllPods)
	assert.Len(t, got[0].Pods, 1)
	assert.Equal(t, "rbd-worker", got[1].Name)
	assert.Equal(t, 1, got[1].RunPods)
	assert.Equal(t, 2, got[1].AllPods)
	assert.Len(t, got[1].Pods, 2)
}

func TestFinalizeRainbondComponentOverviewKeepsZeroPodOperatorSorted(t *testing.T) {
	components := buildRainbondComponentOverview([]corev1.Pod{
		overviewPod("rbd-worker", true),
		overviewPod("rbd-api", true),
	})

	got := finalizeRainbondComponentOverview(components)

	require.Len(t, got, 3)
	assert.Equal(t, "rainbond-operator", got[0].Name)
	assert.Zero(t, got[0].RunPods)
	assert.Zero(t, got[0].AllPods)
	assert.NotNil(t, got[0].Pods)
	assert.Empty(t, got[0].Pods)
	encoded, err := json.Marshal(got[0])
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"pods":[]`)
	assert.Equal(t, "rbd-api", got[1].Name)
	assert.Equal(t, "rbd-worker", got[2].Name)
}

func TestDeduplicateRainbondComponentPodsPreventsOverlappingCounts(t *testing.T) {
	tests := []struct {
		name       string
		configure  func(*corev1.Pod)
		duplicate  func(corev1.Pod) corev1.Pod
		wantAllPod int
	}{
		{
			name: "same UID",
			configure: func(pod *corev1.Pod) {
				pod.UID = "operator-uid"
			},
			duplicate:  func(pod corev1.Pod) corev1.Pod { return *pod.DeepCopy() },
			wantAllPod: 1,
		},
		{
			name:       "empty UID falls back to namespace and name",
			configure:  func(_ *corev1.Pod) {},
			duplicate:  func(pod corev1.Pod) corev1.Pod { return *pod.DeepCopy() },
			wantAllPod: 1,
		},
		{
			name:      "empty UID keeps distinct names",
			configure: func(_ *corev1.Pod) {},
			duplicate: func(pod corev1.Pod) corev1.Pod {
				duplicate := *pod.DeepCopy()
				duplicate.Name = "rainbond-operator-pod-2"
				return duplicate
			},
			wantAllPod: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operator := overviewPod("rainbond-operator", true)
			operator.Namespace = "rbd-system"
			tt.configure(&operator)
			pods := deduplicateRainbondComponentPods([]corev1.Pod{operator, tt.duplicate(operator)})

			components := buildRainbondComponentOverview(pods)

			require.Len(t, components, 1)
			assert.Equal(t, tt.wantAllPod, components[0].AllPods)
			assert.Equal(t, tt.wantAllPod, components[0].RunPods)
			assert.Len(t, components[0].Pods, tt.wantAllPod)
		})
	}
}

func podWithReadyCondition(status corev1.ConditionStatus) corev1.Pod {
	return corev1.Pod{
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: status},
			},
		},
	}
}

func overviewPod(componentName string, ready bool) corev1.Pod {
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}
	pod := podWithReadyCondition(status)
	pod.ObjectMeta = metav1.ObjectMeta{
		Name:   componentName + "-pod",
		Labels: map[string]string{"name": componentName},
	}
	return pod
}

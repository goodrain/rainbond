package exector

import (
	"testing"

	"github.com/goodrain/rainbond-operator/util/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSourceBuildModeFallsBackToBuildMode(t *testing.T) {
	envs := map[string]string{
		"BUILD_MODE": "DOCKERFILE",
	}

	if got := sourceBuildMode(envs); got != "DOCKERFILE" {
		t.Fatalf("expected sourceBuildMode to use BUILD_MODE fallback, got %q", got)
	}
}

func TestSourceBuildModePrefersExplicitMode(t *testing.T) {
	envs := map[string]string{
		"MODE":       "default",
		"BUILD_MODE": "DOCKERFILE",
	}

	if got := sourceBuildMode(envs); got != "DEFAULT" {
		t.Fatalf("expected MODE to win over BUILD_MODE, got %q", got)
	}
}

func TestSourceBuildNoCacheEnabledUsesBuildAlias(t *testing.T) {
	envs := map[string]string{
		"BUILD_NO_CACHE": "True",
	}

	if !sourceBuildNoCacheEnabled(envs) {
		t.Fatal("expected BUILD_NO_CACHE to enable no-cache mode")
	}
}

func TestSourceBuildNoCacheEnabledIgnoresBlankLegacyKey(t *testing.T) {
	envs := map[string]string{
		"NO_CACHE":       "",
		"BUILD_NO_CACHE": "true",
	}

	if !sourceBuildNoCacheEnabled(envs) {
		t.Fatal("expected BUILD_NO_CACHE to win when NO_CACHE is blank")
	}
}

func TestSourceCodeBuildItemGetHostAliasUsesNodeIPv4WhenHostIPUnavailable(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rbd-gateway-0",
				Namespace: constants.Namespace,
				Labels: map[string]string{
					"name": "rbd-gateway",
				},
			},
			Spec: corev1.PodSpec{
				NodeName: "gateway-node",
			},
			Status: corev1.PodStatus{
				HostIP: "",
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gateway-node",
			},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{Type: corev1.NodeInternalIP, Address: "2001:db8::10"},
					{Type: corev1.NodeInternalIP, Address: "10.10.10.10"},
				},
			},
		},
	)

	item := &SourceCodeBuildItem{KubeClient: client}
	got, err := item.getHostAlias()
	if err != nil {
		t.Fatalf("getHostAlias returned error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 host aliases, got %d", len(got))
	}
	for _, alias := range got {
		if alias.IP != "10.10.10.10" {
			t.Fatalf("expected IPv4 host alias IP, got %q", alias.IP)
		}
	}
}

func TestSourceCodeBuildItemGetHostAliasFallsBackToPodIPv4(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rbd-gateway-0",
				Namespace: constants.Namespace,
				Labels: map[string]string{
					"name": "rbd-gateway",
				},
			},
			Status: corev1.PodStatus{
				HostIP: "10.10.10.20",
			},
		},
	)

	item := &SourceCodeBuildItem{KubeClient: client}
	got, err := item.getHostAlias()
	if err != nil {
		t.Fatalf("getHostAlias returned error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 host aliases, got %d", len(got))
	}
	for _, alias := range got {
		if alias.IP != "10.10.10.20" {
			t.Fatalf("expected pod HostIP fallback, got %q", alias.IP)
		}
	}
}

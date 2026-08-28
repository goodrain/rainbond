// Copyright (C) 2014-2026 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

package sources

import (
	"testing"

	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPrepareBuildKitTomlCMPreservesExistingConfigMap(t *testing.T) {
	const (
		namespace = "rbd-system"
		name      = "goodrain-me"
		manual    = "debug = false\n[registry.\"docker.io\"]\n  mirrors = [\"mirror.example.com\"]"
	)
	client := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Data:       map[string]string{"buildkittoml": manual},
	})

	if err := PrepareBuildKitTomlCM(context.Background(), client, namespace, name, "goodrain.me"); err != nil {
		t.Fatalf("PrepareBuildKitTomlCM() error = %v", err)
	}
	for _, action := range client.Actions() {
		if action.GetVerb() != "get" {
			t.Fatalf("existing ConfigMap must be read-only, got Kubernetes action %q", action.GetVerb())
		}
	}

	cm, err := client.CoreV1().ConfigMaps(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get existing ConfigMap: %v", err)
	}
	if got := cm.Data["buildkittoml"]; got != manual {
		t.Fatalf("existing buildkittoml was changed:\n got: %q\nwant: %q", got, manual)
	}
}

func TestPrepareBuildKitTomlCMCreatesMissingConfigMap(t *testing.T) {
	const (
		namespace = "rbd-system"
		name      = "goodrain-me"
	)
	client := fake.NewSimpleClientset()

	if err := PrepareBuildKitTomlCM(context.Background(), client, namespace, name, "goodrain.me"); err != nil {
		t.Fatalf("PrepareBuildKitTomlCM() error = %v", err)
	}

	cm, err := client.CoreV1().ConfigMaps(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get created ConfigMap: %v", err)
	}
	if cm.Data["buildkittoml"] == "" {
		t.Fatal("created ConfigMap has empty buildkittoml")
	}
}

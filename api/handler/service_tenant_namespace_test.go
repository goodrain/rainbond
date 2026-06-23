package handler

import (
	"context"
	"testing"

	"github.com/goodrain/rainbond/util/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestMarkExistingNamespaceManagedPreservesExistingLabels(t *testing.T) {
	kubeClient := kubefake.NewSimpleClientset(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
			Labels: map[string]string{
				"kubernetes.io/metadata.name": "default",
			},
		},
	})
	action := &ServiceAction{kubeClient: kubeClient}

	exists, err := action.markExistingNamespaceManaged(context.Background(), "default")
	if err != nil {
		t.Fatalf("mark namespace managed: %v", err)
	}
	if !exists {
		t.Fatalf("expected default namespace to exist")
	}

	ns, err := kubeClient.CoreV1().Namespaces().Get(context.Background(), "default", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get namespace: %v", err)
	}
	if got := ns.Labels[constants.ResourceManagedByLabel]; got != constants.Rainbond {
		t.Fatalf("expected managed-by label %q, got %q", constants.Rainbond, got)
	}
	if got := ns.Labels["kubernetes.io/metadata.name"]; got != "default" {
		t.Fatalf("expected existing label to be preserved, got %q", got)
	}
}

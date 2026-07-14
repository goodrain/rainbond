package handler

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

func TestDesiredAgentClusterRoleGrantsClusterWideOperations(t *testing.T) {
	role := desiredAgentClusterRole("rainbond-agent-ops")

	want := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"*"},
			Resources: []string{"*"},
			Verbs:     []string{"*"},
		},
		{
			NonResourceURLs: []string{"*"},
			Verbs:           []string{"get"},
		},
	}

	if !reflect.DeepEqual(role.Rules, want) {
		t.Fatalf("unexpected agent cluster role rules: %#v", role.Rules)
	}
}

func TestEnsureAgentServiceAccountLabelsReconcilesMissingLabels(t *testing.T) {
	serviceAccount := &corev1.ServiceAccount{}

	if !ensureAgentServiceAccountLabels(serviceAccount) {
		t.Fatal("expected missing labels to be reconciled")
	}
	if serviceAccount.Labels["app.kubernetes.io/name"] != "rainbond-agent" {
		t.Fatalf("unexpected app name label: %q", serviceAccount.Labels["app.kubernetes.io/name"])
	}
	if serviceAccount.Labels["app.kubernetes.io/managed-by"] != "rainbond" {
		t.Fatalf("unexpected managed-by label: %q", serviceAccount.Labels["app.kubernetes.io/managed-by"])
	}
	if ensureAgentServiceAccountLabels(serviceAccount) {
		t.Fatal("expected second label reconciliation to be a no-op")
	}
}

func TestDesiredAgentClusterRoleBindingTargetsAgentServiceAccount(t *testing.T) {
	binding := desiredAgentClusterRoleBinding(
		"rainbond-agent-ops-rbd-system",
		"rainbond-agent-ops",
		"rbd-system",
		"rainbond-agent",
	)

	if binding.RoleRef.Kind != "ClusterRole" || binding.RoleRef.Name != "rainbond-agent-ops" {
		t.Fatalf("unexpected role ref: %#v", binding.RoleRef)
	}
	if len(binding.Subjects) != 1 {
		t.Fatalf("expected one subject, got %d", len(binding.Subjects))
	}
	subject := binding.Subjects[0]
	if subject.Kind != rbacv1.ServiceAccountKind || subject.Name != "rainbond-agent" || subject.Namespace != "rbd-system" {
		t.Fatalf("unexpected binding subject: %#v", subject)
	}
}

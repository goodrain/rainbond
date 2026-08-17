package handler

import (
	"context"
	"reflect"
	"testing"

	httputil "github.com/goodrain/rainbond/util/http"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestResolveAgentCredentialProfile(t *testing.T) {
	tests := []struct {
		name               string
		request            AgentKubeconfigBootstrapRequest
		wantProfile        string
		wantServiceAccount string
		wantBadRequest     bool
	}{
		{
			name:               "default ops keeps legacy identity",
			request:            AgentKubeconfigBootstrapRequest{},
			wantProfile:        agentCredentialProfileOps,
			wantServiceAccount: "rainbond-agent",
		},
		{
			name: "explicit ops keeps custom identity",
			request: AgentKubeconfigBootstrapRequest{
				CredentialProfile: agentCredentialProfileOps,
				ServiceAccount:    "custom-agent",
			},
			wantProfile:        agentCredentialProfileOps,
			wantServiceAccount: "custom-agent",
		},
		{
			name: "ops rejects fixed reader identity",
			request: AgentKubeconfigBootstrapRequest{
				CredentialProfile: agentCredentialProfileOps,
				ServiceAccount:    agentReaderServiceAccount,
			},
			wantBadRequest: true,
		},
		{
			name: "readonly uses reader identity",
			request: AgentKubeconfigBootstrapRequest{
				CredentialProfile: agentCredentialProfileReadonly,
			},
			wantProfile:        agentCredentialProfileReadonly,
			wantServiceAccount: "rainbond-agent-reader",
		},
		{
			name: "readonly accepts fixed reader identity",
			request: AgentKubeconfigBootstrapRequest{
				CredentialProfile: agentCredentialProfileReadonly,
				ServiceAccount:    "rainbond-agent-reader",
			},
			wantProfile:        agentCredentialProfileReadonly,
			wantServiceAccount: "rainbond-agent-reader",
		},
		{
			name: "readonly rejects legacy ops identity",
			request: AgentKubeconfigBootstrapRequest{
				CredentialProfile: agentCredentialProfileReadonly,
				ServiceAccount:    "rainbond-agent",
			},
			wantBadRequest: true,
		},
		{
			name: "readonly rejects custom identity",
			request: AgentKubeconfigBootstrapRequest{
				CredentialProfile: agentCredentialProfileReadonly,
				ServiceAccount:    "custom-agent",
			},
			wantBadRequest: true,
		},
		{
			name: "unknown profile is rejected",
			request: AgentKubeconfigBootstrapRequest{
				CredentialProfile: "administrator",
			},
			wantBadRequest: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, serviceAccount, err := resolveAgentCredentialProfile(&tt.request)
			if tt.wantBadRequest {
				if err == nil {
					t.Fatal("expected invalid credential profile to be rejected")
				}
				if _, ok := err.(httputil.ErrBadRequest); !ok {
					t.Fatalf("expected bad request error, got %T: %v", err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolve credential profile: %v", err)
			}
			if profile != tt.wantProfile {
				t.Fatalf("expected profile %q, got %q", tt.wantProfile, profile)
			}
			if serviceAccount != tt.wantServiceAccount {
				t.Fatalf("expected service account %q, got %q", tt.wantServiceAccount, serviceAccount)
			}
		})
	}
}

func TestDesiredAgentClusterRoleGrantsClusterWideOperations(t *testing.T) {
	role := desiredAgentClusterRole("rainbond-agent-ops", agentCredentialProfileOps)

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

func TestDesiredReadonlyAgentClusterRoleAllowsDiagnosticsOnly(t *testing.T) {
	role := desiredAgentClusterRole("rainbond-agent-reader", agentCredentialProfileReadonly)

	allowed := []struct {
		apiGroup string
		resource string
	}{
		{apiGroup: "", resource: "nodes"},
		{apiGroup: "", resource: "namespaces"},
		{apiGroup: "", resource: "pods"},
		{apiGroup: "", resource: "pods/log"},
		{apiGroup: "", resource: "services"},
		{apiGroup: "", resource: "endpoints"},
		{apiGroup: "", resource: "events"},
		{apiGroup: "", resource: "persistentvolumes"},
		{apiGroup: "", resource: "persistentvolumeclaims"},
		{apiGroup: "apps", resource: "deployments"},
		{apiGroup: "apps", resource: "statefulsets"},
		{apiGroup: "apps", resource: "daemonsets"},
		{apiGroup: "apps", resource: "replicasets"},
		{apiGroup: "batch", resource: "jobs"},
		{apiGroup: "batch", resource: "cronjobs"},
		{apiGroup: "networking.k8s.io", resource: "ingresses"},
		{apiGroup: "networking.k8s.io", resource: "networkpolicies"},
		{apiGroup: "metrics.k8s.io", resource: "nodes"},
		{apiGroup: "metrics.k8s.io", resource: "pods"},
	}
	for _, resource := range allowed {
		for _, verb := range []string{"get", "list", "watch"} {
			if !agentRoleAllows(role, resource.apiGroup, resource.resource, verb) {
				t.Errorf("expected readonly role to allow %s %s/%s", verb, resource.apiGroup, resource.resource)
			}
		}
	}

	for _, rule := range role.Rules {
		for _, verb := range rule.Verbs {
			if verb != "get" && verb != "list" && verb != "watch" {
				t.Errorf("readonly role grants forbidden verb %q in rule %#v", verb, rule)
			}
		}
	}
}

func TestDesiredReadonlyAgentClusterRoleRejectsSensitiveOperations(t *testing.T) {
	role := desiredAgentClusterRole("rainbond-agent-reader", agentCredentialProfileReadonly)

	forbiddenResources := []struct {
		apiGroup string
		resource string
	}{
		{apiGroup: "", resource: "secrets"},
		{apiGroup: "", resource: "serviceaccounts"},
		{apiGroup: "", resource: "serviceaccounts/token"},
		{apiGroup: "", resource: "pods/exec"},
		{apiGroup: "", resource: "pods/attach"},
		{apiGroup: "", resource: "pods/portforward"},
	}
	for _, resource := range forbiddenResources {
		for _, verb := range []string{"get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"} {
			if agentRoleAllows(role, resource.apiGroup, resource.resource, verb) {
				t.Errorf("readonly role unexpectedly allows %s %s/%s", verb, resource.apiGroup, resource.resource)
			}
		}
	}

	for _, resource := range []struct {
		apiGroup string
		resource string
	}{
		{apiGroup: "", resource: "pods"},
		{apiGroup: "apps", resource: "deployments"},
		{apiGroup: "batch", resource: "jobs"},
	} {
		for _, verb := range []string{"create", "update", "patch", "delete", "deletecollection"} {
			if agentRoleAllows(role, resource.apiGroup, resource.resource, verb) {
				t.Errorf("readonly role unexpectedly allows %s %s/%s", verb, resource.apiGroup, resource.resource)
			}
		}
	}
}

func TestReconcileAgentRBACRepairsDrift(t *testing.T) {
	desiredRole := desiredAgentClusterRole("rainbond-agent-reader", agentCredentialProfileReadonly)
	existingRole := &rbacv1.ClusterRole{Rules: []rbacv1.PolicyRule{{
		APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"},
	}}}
	if !reconcileAgentClusterRole(existingRole, desiredRole) {
		t.Fatal("expected drifted reader role to be reconciled")
	}
	if !reflect.DeepEqual(existingRole.Rules, desiredRole.Rules) {
		t.Fatalf("reader role rules were not reconciled: %#v", existingRole.Rules)
	}
	if reconcileAgentClusterRole(existingRole, desiredRole) {
		t.Fatal("expected matching reader role to require no update")
	}

	desiredBinding := desiredAgentClusterRoleBinding(
		"rainbond-agent-reader-rbd-system",
		"rainbond-agent-reader",
		"rbd-system",
		"rainbond-agent-reader",
	)
	existingBinding := &rbacv1.ClusterRoleBinding{
		Subjects: []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: "rainbond-agent", Namespace: "rbd-system"}},
		RoleRef:  rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "rainbond-agent-ops"},
	}
	if !reconcileAgentClusterRoleBinding(existingBinding, desiredBinding) {
		t.Fatal("expected drifted reader binding to be reconciled")
	}
	if !reflect.DeepEqual(existingBinding.Subjects, desiredBinding.Subjects) || !reflect.DeepEqual(existingBinding.RoleRef, desiredBinding.RoleRef) {
		t.Fatalf("reader binding was not reconciled: %#v", existingBinding)
	}
	if reconcileAgentClusterRoleBinding(existingBinding, desiredBinding) {
		t.Fatal("expected matching reader binding to require no update")
	}
}

func TestAgentRBACProfilesUseIndependentBindings(t *testing.T) {
	opsRoleName, opsBindingName := agentRBACResourceNames(agentCredentialProfileOps, "rbd-system")
	readerRoleName, readerBindingName := agentRBACResourceNames(agentCredentialProfileReadonly, "rbd-system")
	if opsRoleName != "rainbond-agent-ops" || opsBindingName != "rainbond-agent-ops-rbd-system" {
		t.Fatalf("unexpected ops RBAC names: %q/%q", opsRoleName, opsBindingName)
	}
	if readerRoleName != "rainbond-agent-reader" || readerBindingName != "rainbond-agent-reader-rbd-system" {
		t.Fatalf("unexpected reader RBAC names: %q/%q", readerRoleName, readerBindingName)
	}
	if opsRoleName == readerRoleName || opsBindingName == readerBindingName {
		t.Fatalf("expected profile RBAC names to be independent, ops=%q/%q reader=%q/%q", opsRoleName, opsBindingName, readerRoleName, readerBindingName)
	}

	opsBinding := desiredAgentClusterRoleBinding(opsBindingName, opsRoleName, "rbd-system", "rainbond-agent")
	readerBinding := desiredAgentClusterRoleBinding(readerBindingName, readerRoleName, "rbd-system", "rainbond-agent-reader")
	originalOpsBinding := opsBinding.DeepCopy()
	driftedReaderBinding := &rbacv1.ClusterRoleBinding{}
	if !reconcileAgentClusterRoleBinding(driftedReaderBinding, readerBinding) {
		t.Fatal("expected reader binding drift to be reconciled")
	}
	if !reflect.DeepEqual(opsBinding, originalOpsBinding) {
		t.Fatalf("reader reconciliation unexpectedly modified ops binding: %#v", opsBinding)
	}
	if driftedReaderBinding.Name == opsBinding.Name || driftedReaderBinding.RoleRef.Name == opsBinding.RoleRef.Name {
		t.Fatalf("reader binding overlaps ops binding: %#v", driftedReaderBinding)
	}
}

func TestEnsureAgentServiceAccountKeepsProfileBindingsIndependent(t *testing.T) {
	ctx := context.Background()
	clientset := k8sfake.NewSimpleClientset()

	if err := ensureAgentServiceAccountWithClient(ctx, clientset, agentCredentialProfileOps, "rbd-system", agentOpsServiceAccount); err != nil {
		t.Fatalf("ensure ops service account: %v", err)
	}
	opsBinding, err := clientset.RbacV1().ClusterRoleBindings().Get(ctx, "rainbond-agent-ops-rbd-system", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get ops binding: %v", err)
	}
	opsBindingBefore := opsBinding.DeepCopy()

	if err := ensureAgentServiceAccountWithClient(ctx, clientset, agentCredentialProfileReadonly, "rbd-system", agentReaderServiceAccount); err != nil {
		t.Fatalf("ensure reader service account: %v", err)
	}
	for _, serviceAccount := range []string{agentOpsServiceAccount, agentReaderServiceAccount} {
		if _, err := clientset.CoreV1().ServiceAccounts("rbd-system").Get(ctx, serviceAccount, metav1.GetOptions{}); err != nil {
			t.Fatalf("get service account %q: %v", serviceAccount, err)
		}
	}
	readerBinding, err := clientset.RbacV1().ClusterRoleBindings().Get(ctx, "rainbond-agent-reader-rbd-system", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get reader binding: %v", err)
	}
	if len(readerBinding.Subjects) != 1 || readerBinding.Subjects[0].Name != agentReaderServiceAccount {
		t.Fatalf("reader binding targets unexpected subjects: %#v", readerBinding.Subjects)
	}

	readerRole, err := clientset.RbacV1().ClusterRoles().Get(ctx, agentReaderClusterRole, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get reader role: %v", err)
	}
	readerRole.Rules = []rbacv1.PolicyRule{{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}}}
	if _, err = clientset.RbacV1().ClusterRoles().Update(ctx, readerRole, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("drift reader role: %v", err)
	}
	readerBinding.Subjects = []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: agentOpsServiceAccount, Namespace: "rbd-system"}}
	readerBinding.RoleRef.Name = agentOpsClusterRole
	if _, err = clientset.RbacV1().ClusterRoleBindings().Update(ctx, readerBinding, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("drift reader binding: %v", err)
	}

	if err := ensureAgentServiceAccountWithClient(ctx, clientset, agentCredentialProfileReadonly, "rbd-system", agentReaderServiceAccount); err != nil {
		t.Fatalf("reconcile reader service account: %v", err)
	}
	reconciledReaderRole, err := clientset.RbacV1().ClusterRoles().Get(ctx, agentReaderClusterRole, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get reconciled reader role: %v", err)
	}
	if !reflect.DeepEqual(reconciledReaderRole.Rules, desiredAgentClusterRole(agentReaderClusterRole, agentCredentialProfileReadonly).Rules) {
		t.Fatalf("reader role drift was not repaired: %#v", reconciledReaderRole.Rules)
	}
	reconciledReaderBinding, err := clientset.RbacV1().ClusterRoleBindings().Get(ctx, "rainbond-agent-reader-rbd-system", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get reconciled reader binding: %v", err)
	}
	if len(reconciledReaderBinding.Subjects) != 1 || reconciledReaderBinding.Subjects[0].Name != agentReaderServiceAccount || reconciledReaderBinding.RoleRef.Name != agentReaderClusterRole {
		t.Fatalf("reader binding drift was not repaired: %#v", reconciledReaderBinding)
	}
	opsBindingAfter, err := clientset.RbacV1().ClusterRoleBindings().Get(ctx, "rainbond-agent-ops-rbd-system", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get ops binding after reader reconciliation: %v", err)
	}
	if !reflect.DeepEqual(opsBindingAfter, opsBindingBefore) {
		t.Fatalf("reader reconciliation modified ops binding: before=%#v after=%#v", opsBindingBefore, opsBindingAfter)
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

func agentRoleAllows(role *rbacv1.ClusterRole, apiGroup, resource, verb string) bool {
	for _, rule := range role.Rules {
		if containsAgentRBACValue(rule.APIGroups, apiGroup) &&
			containsAgentRBACValue(rule.Resources, resource) &&
			containsAgentRBACValue(rule.Verbs, verb) {
			return true
		}
	}
	return false
}

func containsAgentRBACValue(values []string, want string) bool {
	for _, value := range values {
		if value == want || value == "*" {
			return true
		}
	}
	return false
}

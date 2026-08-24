package handler

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"reflect"
	"time"

	httputil "github.com/goodrain/rainbond/util/http"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	agentCredentialProfileOps      = "ops"
	agentCredentialProfileReadonly = "readonly"
	agentOpsServiceAccount         = "rainbond-agent"
	agentReaderServiceAccount      = "rainbond-agent-reader"
	agentOpsClusterRole            = "rainbond-agent-ops"
	agentReaderClusterRole         = "rainbond-agent-reader"
)

// AgentKubeconfigBootstrapRequest creates an agent-owned Kubernetes identity.
type AgentKubeconfigBootstrapRequest struct {
	RegionName        string `json:"region_name"`
	ContextID         string `json:"context_id"`
	CredentialProfile string `json:"credential_profile"`
	ServiceAccount    string `json:"service_account"`
	Namespace         string `json:"namespace"`
	ExpirationSeconds int64  `json:"expiration_seconds"`
}

// AgentKubeconfigBootstrapResponse returns a kubeconfig that can be stored by rainbond-agent.
type AgentKubeconfigBootstrapResponse struct {
	RegionName        string `json:"region_name"`
	ContextID         string `json:"context_id"`
	CredentialProfile string `json:"credential_profile"`
	Namespace         string `json:"namespace"`
	ServiceAccount    string `json:"service_account"`
	Kubeconfig        string `json:"kubeconfig"`
	ExpiresAt         string `json:"expires_at,omitempty"`
}

func (c *clusterAction) BootstrapAgentKubeconfig(ctx context.Context, req *AgentKubeconfigBootstrapRequest) (*AgentKubeconfigBootstrapResponse, error) {
	if req == nil {
		req = &AgentKubeconfigBootstrapRequest{}
	}
	credentialProfile, serviceAccount, err := resolveAgentCredentialProfile(req)
	if err != nil {
		return nil, err
	}
	namespace := firstNonEmptyAgentValue(req.Namespace, c.namespace, "rbd-system")
	contextID := firstNonEmptyAgentValue(req.ContextID, req.RegionName, "rainbond")
	expirationSeconds := req.ExpirationSeconds
	if expirationSeconds <= 0 {
		expirationSeconds = int64((24 * time.Hour * 365).Seconds())
	}

	if err := c.ensureAgentServiceAccount(ctx, credentialProfile, namespace, serviceAccount); err != nil {
		return nil, err
	}
	token, expiresAt, err := c.createAgentServiceAccountToken(ctx, namespace, serviceAccount, expirationSeconds)
	if err != nil {
		return nil, err
	}
	caData, err := c.clusterCAData()
	if err != nil {
		return nil, err
	}
	kubeconfig := buildAgentKubeconfig(contextID, contextID, serviceAccount, namespace, c.config.Host, caData, token)
	return &AgentKubeconfigBootstrapResponse{
		RegionName:        req.RegionName,
		ContextID:         contextID,
		CredentialProfile: credentialProfile,
		Namespace:         namespace,
		ServiceAccount:    serviceAccount,
		Kubeconfig:        kubeconfig,
		ExpiresAt:         expiresAt,
	}, nil
}

func resolveAgentCredentialProfile(req *AgentKubeconfigBootstrapRequest) (string, string, error) {
	profile := firstNonEmptyAgentValue(req.CredentialProfile, agentCredentialProfileOps)
	switch profile {
	case agentCredentialProfileOps:
		serviceAccount := firstNonEmptyAgentValue(req.ServiceAccount, agentOpsServiceAccount)
		if serviceAccount == agentReaderServiceAccount {
			return "", "", httputil.NewErrBadRequest(fmt.Errorf("service_account %q is reserved for credential_profile %q", serviceAccount, agentCredentialProfileReadonly))
		}
		return profile, serviceAccount, nil
	case agentCredentialProfileReadonly:
		if req.ServiceAccount != "" && req.ServiceAccount != agentReaderServiceAccount {
			return "", "", httputil.NewErrBadRequest(fmt.Errorf("credential_profile %q requires service_account %q", profile, agentReaderServiceAccount))
		}
		return profile, agentReaderServiceAccount, nil
	default:
		return "", "", httputil.NewErrBadRequest(fmt.Errorf("unsupported credential_profile %q", profile))
	}
}

func (c *clusterAction) ensureAgentServiceAccount(ctx context.Context, credentialProfile, namespace, serviceAccount string) error {
	return ensureAgentServiceAccountWithClient(ctx, c.clientset, credentialProfile, namespace, serviceAccount)
}

func ensureAgentServiceAccountWithClient(ctx context.Context, clientset kubernetes.Interface, credentialProfile, namespace, serviceAccount string) error {
	existingServiceAccount, err := clientset.CoreV1().ServiceAccounts(namespace).Get(ctx, serviceAccount, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = clientset.CoreV1().ServiceAccounts(namespace).Create(ctx, desiredAgentServiceAccount(namespace, serviceAccount), metav1.CreateOptions{})
		existingServiceAccount = nil
	}
	if err != nil {
		return fmt.Errorf("ensure service account: %w", err)
	}
	if existingServiceAccount != nil && ensureAgentServiceAccountLabels(existingServiceAccount) {
		_, err = clientset.CoreV1().ServiceAccounts(namespace).Update(ctx, existingServiceAccount, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("update service account labels: %w", err)
		}
	}

	roleName, bindingName := agentRBACResourceNames(credentialProfile, namespace)
	desiredRole := desiredAgentClusterRole(roleName, credentialProfile)
	existingRole, err := clientset.RbacV1().ClusterRoles().Get(ctx, roleName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = clientset.RbacV1().ClusterRoles().Create(ctx, desiredRole, metav1.CreateOptions{})
		existingRole = nil
	}
	if err != nil {
		return fmt.Errorf("ensure cluster role: %w", err)
	}
	if existingRole != nil && reconcileAgentClusterRole(existingRole, desiredRole) {
		_, err = clientset.RbacV1().ClusterRoles().Update(ctx, existingRole, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("update cluster role: %w", err)
		}
	}

	desiredBinding := desiredAgentClusterRoleBinding(bindingName, roleName, namespace, serviceAccount)
	existingBinding, err := clientset.RbacV1().ClusterRoleBindings().Get(ctx, bindingName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = clientset.RbacV1().ClusterRoleBindings().Create(ctx, desiredBinding, metav1.CreateOptions{})
		existingBinding = nil
	}
	if err != nil {
		return fmt.Errorf("ensure cluster role binding: %w", err)
	}
	if existingBinding != nil && reconcileAgentClusterRoleBinding(existingBinding, desiredBinding) {
		_, err = clientset.RbacV1().ClusterRoleBindings().Update(ctx, existingBinding, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("update cluster role binding: %w", err)
		}
	}
	return nil
}

func desiredAgentServiceAccount(namespace, serviceAccount string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccount,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "rainbond-agent",
				"app.kubernetes.io/managed-by": "rainbond",
			},
		},
	}
}

func ensureAgentServiceAccountLabels(serviceAccount *corev1.ServiceAccount) bool {
	changed := false
	if serviceAccount.Labels == nil {
		serviceAccount.Labels = map[string]string{}
		changed = true
	}
	expected := desiredAgentServiceAccount("", "").Labels
	for key, value := range expected {
		if serviceAccount.Labels[key] != value {
			serviceAccount.Labels[key] = value
			changed = true
		}
	}
	return changed
}

func agentRBACResourceNames(credentialProfile, namespace string) (string, string) {
	roleName := agentOpsClusterRole
	if credentialProfile == agentCredentialProfileReadonly {
		roleName = agentReaderClusterRole
	}
	return roleName, fmt.Sprintf("%s-%s", roleName, namespace)
}

func desiredAgentClusterRole(roleName, credentialProfile string) *rbacv1.ClusterRole {
	rules := []rbacv1.PolicyRule{
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
	if credentialProfile == agentCredentialProfileReadonly {
		readVerbs := []string{"get", "list", "watch"}
		rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{
					"nodes", "namespaces", "pods", "pods/log", "services", "endpoints", "events",
					"persistentvolumes", "persistentvolumeclaims",
				},
				Verbs: readVerbs,
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "statefulsets", "daemonsets", "replicasets"},
				Verbs:     readVerbs,
			},
			{
				APIGroups: []string{"batch"},
				Resources: []string{"jobs", "cronjobs"},
				Verbs:     readVerbs,
			},
			{
				APIGroups: []string{"networking.k8s.io"},
				Resources: []string{"ingresses", "networkpolicies"},
				Verbs:     readVerbs,
			},
			{
				APIGroups: []string{"metrics.k8s.io"},
				Resources: []string{"nodes", "pods"},
				Verbs:     readVerbs,
			},
		}
	}
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
		},
		Rules: rules,
	}
}

func reconcileAgentClusterRole(existingRole, desiredRole *rbacv1.ClusterRole) bool {
	if reflect.DeepEqual(existingRole.Rules, desiredRole.Rules) {
		return false
	}
	existingRole.Rules = desiredRole.Rules
	return true
}

func reconcileAgentClusterRoleBinding(existingBinding, desiredBinding *rbacv1.ClusterRoleBinding) bool {
	if reflect.DeepEqual(existingBinding.Subjects, desiredBinding.Subjects) && reflect.DeepEqual(existingBinding.RoleRef, desiredBinding.RoleRef) {
		return false
	}
	existingBinding.Subjects = desiredBinding.Subjects
	existingBinding.RoleRef = desiredBinding.RoleRef
	return true
}

func desiredAgentClusterRoleBinding(bindingName, roleName, namespace, serviceAccount string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: bindingName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     roleName,
		},
	}
}

func (c *clusterAction) createAgentServiceAccountToken(ctx context.Context, namespace, serviceAccount string, expirationSeconds int64) (string, string, error) {
	tokenRequest, err := c.clientset.CoreV1().ServiceAccounts(namespace).CreateToken(ctx, serviceAccount, &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			ExpirationSeconds: &expirationSeconds,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return "", "", fmt.Errorf("create service account token: %w", err)
	}
	expiresAt := ""
	if !tokenRequest.Status.ExpirationTimestamp.IsZero() {
		expiresAt = tokenRequest.Status.ExpirationTimestamp.Time.Format(time.RFC3339)
	}
	return tokenRequest.Status.Token, expiresAt, nil
}

func (c *clusterAction) clusterCAData() (string, error) {
	if len(c.config.CAData) > 0 {
		return base64.StdEncoding.EncodeToString(c.config.CAData), nil
	}
	if c.config.CAFile != "" {
		data, err := os.ReadFile(c.config.CAFile)
		if err != nil {
			return "", fmt.Errorf("read cluster ca file: %w", err)
		}
		return base64.StdEncoding.EncodeToString(data), nil
	}
	return "", nil
}

func buildAgentKubeconfig(contextID, clusterName, userName, namespace, server, caData, token string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- name: %q
  cluster:
    server: %q
    certificate-authority-data: %q
users:
- name: %q
  user:
    token: %q
contexts:
- name: %q
  context:
    cluster: %q
    user: %q
    namespace: %q
current-context: %q
`, clusterName, server, caData, userName, token, contextID, clusterName, userName, namespace, contextID)
}

func firstNonEmptyAgentValue(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

package handler

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"reflect"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AgentKubeconfigBootstrapRequest creates an agent-owned Kubernetes identity.
type AgentKubeconfigBootstrapRequest struct {
	RegionName        string `json:"region_name"`
	ContextID         string `json:"context_id"`
	ServiceAccount    string `json:"service_account"`
	Namespace         string `json:"namespace"`
	ExpirationSeconds int64  `json:"expiration_seconds"`
}

// AgentKubeconfigBootstrapResponse returns a kubeconfig that can be stored by rainbond-agent.
type AgentKubeconfigBootstrapResponse struct {
	RegionName     string `json:"region_name"`
	ContextID      string `json:"context_id"`
	Namespace      string `json:"namespace"`
	ServiceAccount string `json:"service_account"`
	Kubeconfig     string `json:"kubeconfig"`
	ExpiresAt      string `json:"expires_at,omitempty"`
}

func (c *clusterAction) BootstrapAgentKubeconfig(ctx context.Context, req *AgentKubeconfigBootstrapRequest) (*AgentKubeconfigBootstrapResponse, error) {
	if req == nil {
		req = &AgentKubeconfigBootstrapRequest{}
	}
	namespace := firstNonEmptyAgentValue(req.Namespace, c.namespace, "rbd-system")
	serviceAccount := firstNonEmptyAgentValue(req.ServiceAccount, "rainbond-agent")
	contextID := firstNonEmptyAgentValue(req.ContextID, req.RegionName, "rainbond")
	expirationSeconds := req.ExpirationSeconds
	if expirationSeconds <= 0 {
		expirationSeconds = int64((24 * time.Hour * 365).Seconds())
	}

	if err := c.ensureAgentServiceAccount(ctx, namespace, serviceAccount); err != nil {
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
		RegionName:     req.RegionName,
		ContextID:      contextID,
		Namespace:      namespace,
		ServiceAccount: serviceAccount,
		Kubeconfig:     kubeconfig,
		ExpiresAt:      expiresAt,
	}, nil
}

func (c *clusterAction) ensureAgentServiceAccount(ctx context.Context, namespace, serviceAccount string) error {
	existingServiceAccount, err := c.clientset.CoreV1().ServiceAccounts(namespace).Get(ctx, serviceAccount, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = c.clientset.CoreV1().ServiceAccounts(namespace).Create(ctx, desiredAgentServiceAccount(namespace, serviceAccount), metav1.CreateOptions{})
	}
	if err != nil {
		return fmt.Errorf("ensure service account: %w", err)
	}
	if existingServiceAccount != nil && ensureAgentServiceAccountLabels(existingServiceAccount) {
		_, err = c.clientset.CoreV1().ServiceAccounts(namespace).Update(ctx, existingServiceAccount, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("update service account labels: %w", err)
		}
	}

	roleName := "rainbond-agent-ops"
	desiredRole := desiredAgentClusterRole(roleName)
	existingRole, err := c.clientset.RbacV1().ClusterRoles().Get(ctx, roleName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = c.clientset.RbacV1().ClusterRoles().Create(ctx, desiredRole, metav1.CreateOptions{})
	}
	if err != nil {
		return fmt.Errorf("ensure cluster role: %w", err)
	}
	if existingRole != nil && !reflect.DeepEqual(existingRole.Rules, desiredRole.Rules) {
		existingRole.Rules = desiredRole.Rules
		_, err = c.clientset.RbacV1().ClusterRoles().Update(ctx, existingRole, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("update cluster role: %w", err)
		}
	}

	bindingName := fmt.Sprintf("%s-%s", roleName, namespace)
	desiredBinding := desiredAgentClusterRoleBinding(bindingName, roleName, namespace, serviceAccount)
	existingBinding, err := c.clientset.RbacV1().ClusterRoleBindings().Get(ctx, bindingName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = c.clientset.RbacV1().ClusterRoleBindings().Create(ctx, desiredBinding, metav1.CreateOptions{})
	}
	if err != nil {
		return fmt.Errorf("ensure cluster role binding: %w", err)
	}
	if existingBinding != nil && (!reflect.DeepEqual(existingBinding.Subjects, desiredBinding.Subjects) || !reflect.DeepEqual(existingBinding.RoleRef, desiredBinding.RoleRef)) {
		existingBinding.Subjects = desiredBinding.Subjects
		existingBinding.RoleRef = desiredBinding.RoleRef
		_, err = c.clientset.RbacV1().ClusterRoleBindings().Update(ctx, existingBinding, metav1.UpdateOptions{})
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

func desiredAgentClusterRole(roleName string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
			{
				NonResourceURLs: []string{"*"},
				Verbs:           []string{"get"},
			},
		},
	}
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

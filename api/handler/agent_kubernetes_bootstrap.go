package handler

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
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
	_, err := c.clientset.CoreV1().ServiceAccounts(namespace).Get(ctx, serviceAccount, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = c.clientset.CoreV1().ServiceAccounts(namespace).Create(ctx, &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccount,
				Namespace: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/name":       "rainbond-agent",
					"app.kubernetes.io/managed-by": "rainbond",
				},
			},
		}, metav1.CreateOptions{})
	}
	if err != nil {
		return fmt.Errorf("ensure service account: %w", err)
	}

	roleName := "rainbond-agent-ops"
	_, err = c.clientset.RbacV1().ClusterRoles().Get(ctx, roleName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = c.clientset.RbacV1().ClusterRoles().Create(ctx, &rbacv1.ClusterRole{
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
		}, metav1.CreateOptions{})
	}
	if err != nil {
		return fmt.Errorf("ensure cluster role: %w", err)
	}

	bindingName := fmt.Sprintf("%s-%s", roleName, namespace)
	_, err = c.clientset.RbacV1().ClusterRoleBindings().Get(ctx, bindingName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = c.clientset.RbacV1().ClusterRoleBindings().Create(ctx, &rbacv1.ClusterRoleBinding{
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
		}, metav1.CreateOptions{})
	}
	if err != nil {
		return fmt.Errorf("ensure cluster role binding: %w", err)
	}
	return nil
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

package sources

import (
	"context"
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goodrain/rainbond/config/configs"
	componentK8s "github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/util/constants"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	tenantSSHPrivateKeyDataKey = "id_rsa"
	tenantSSHPublicKeyDataKey  = "id_rsa.pub"
)

type tenantSSHKeyPair struct {
	Private string
	Public  string
}

// GetPublicKey returns the tenant public key and persists the key pair in a
// shared Kubernetes Secret so every rbd-chaos pod uses the same private key.
func GetPublicKey(tenantID string) (string, error) {
	home, _ := Home()
	if home == "" {
		home = "/root"
	}

	key, err := getOrCreateTenantSSHKey(context.Background(), tenantID, home, tenantSSHKeyNamespace(), defaultKubernetesClient())
	if err != nil {
		logrus.Errorf("get tenant ssh key failure: %v", err)
		return "", err
	}
	return key.Public, nil
}

func getPrivateFile(ctx context.Context, tenantID, home, namespace string, client kubernetes.Interface) string {
	if home == "" {
		home = "/root"
	}
	privatePath := filepath.Join(home, ".ssh", tenantID)
	if tenantID != "" && tenantID != "builder_rsa" {
		if err := syncTenantSSHKeyFromSecret(ctx, tenantID, home, namespace, client); err != nil {
			logrus.Warnf("sync tenant ssh key from secret failure: %v", err)
		}
		if ok, _ := util.FileExists(privatePath); ok {
			return privatePath
		}
	}

	builderKeyPath := filepath.Join(home, ".ssh", "builder_rsa")
	if ok, _ := util.FileExists(builderKeyPath); ok {
		return builderKeyPath
	}
	return filepath.Join(home, ".ssh", "id_rsa")
}

func getOrCreateTenantSSHKey(ctx context.Context, tenantID, home, namespace string, client kubernetes.Interface) (tenantSSHKeyPair, error) {
	if tenantID == "" {
		return tenantSSHKeyPair{}, fmt.Errorf("tenant id is empty")
	}
	if home == "" {
		home = "/root"
	}
	if namespace == "" {
		namespace = constants.Namespace
	}

	if client != nil {
		key, ok, err := getTenantSSHKeyFromSecret(ctx, client, namespace, tenantID)
		if err != nil {
			return tenantSSHKeyPair{}, err
		}
		if ok {
			return key, writeTenantSSHKeyToHome(home, tenantID, key)
		}
	}

	key, ok, err := readTenantSSHKeyFromHome(home, tenantID)
	if err != nil {
		return tenantSSHKeyPair{}, err
	}
	if !ok {
		private, public, err := MakeSSHKeyPair()
		if err != nil {
			return tenantSSHKeyPair{}, err
		}
		key = tenantSSHKeyPair{Private: private, Public: public}
	}

	if client != nil {
		key, err = createTenantSSHKeySecret(ctx, client, namespace, tenantID, key)
		if err != nil {
			return tenantSSHKeyPair{}, err
		}
	}
	return key, writeTenantSSHKeyToHome(home, tenantID, key)
}

func syncTenantSSHKeyFromSecret(ctx context.Context, tenantID, home, namespace string, client kubernetes.Interface) error {
	if client == nil {
		return nil
	}
	key, ok, err := getTenantSSHKeyFromSecret(ctx, client, namespace, tenantID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return writeTenantSSHKeyToHome(home, tenantID, key)
}

func getTenantSSHKeyFromSecret(ctx context.Context, client kubernetes.Interface, namespace, tenantID string) (tenantSSHKeyPair, bool, error) {
	secret, err := client.CoreV1().Secrets(namespace).Get(ctx, tenantSSHKeySecretName(tenantID), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return tenantSSHKeyPair{}, false, nil
	}
	if err != nil {
		return tenantSSHKeyPair{}, false, err
	}

	private := string(secret.Data[tenantSSHPrivateKeyDataKey])
	public := string(secret.Data[tenantSSHPublicKeyDataKey])
	if private == "" || public == "" {
		return tenantSSHKeyPair{}, false, fmt.Errorf("tenant ssh key secret %s/%s is incomplete", namespace, secret.Name)
	}
	return tenantSSHKeyPair{Private: private, Public: public}, true, nil
}

func createTenantSSHKeySecret(ctx context.Context, client kubernetes.Interface, namespace, tenantID string, key tenantSSHKeyPair) (tenantSSHKeyPair, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tenantSSHKeySecretName(tenantID),
			Namespace: namespace,
			Labels: map[string]string{
				constants.ResourceManagedByLabel: "rainbond",
				constants.ResourceAppLabel:       "rbd-chaos",
			},
			Annotations: map[string]string{
				"rainbond.io/tenant-id": tenantID,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			tenantSSHPrivateKeyDataKey: []byte(key.Private),
			tenantSSHPublicKeyDataKey:  []byte(key.Public),
		},
	}

	if _, err := client.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return tenantSSHKeyPair{}, err
		}
		existing, ok, getErr := getTenantSSHKeyFromSecret(ctx, client, namespace, tenantID)
		if getErr != nil {
			return tenantSSHKeyPair{}, getErr
		}
		if !ok {
			return tenantSSHKeyPair{}, fmt.Errorf("tenant ssh key secret %s/%s already exists but cannot be read", namespace, secret.Name)
		}
		return existing, nil
	}
	return key, nil
}

func readTenantSSHKeyFromHome(home, tenantID string) (tenantSSHKeyPair, bool, error) {
	private, err := os.ReadFile(filepath.Join(home, ".ssh", tenantID))
	if os.IsNotExist(err) {
		return tenantSSHKeyPair{}, false, nil
	}
	if err != nil {
		return tenantSSHKeyPair{}, false, err
	}

	public, err := os.ReadFile(filepath.Join(home, ".ssh", tenantID+".pub"))
	if os.IsNotExist(err) {
		return tenantSSHKeyPair{}, false, nil
	}
	if err != nil {
		return tenantSSHKeyPair{}, false, err
	}
	if len(private) == 0 || len(public) == 0 {
		return tenantSSHKeyPair{}, false, nil
	}
	return tenantSSHKeyPair{Private: string(private), Public: string(public)}, true, nil
}

func writeTenantSSHKeyToHome(home, tenantID string, key tenantSSHKeyPair) error {
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return err
	}
	privatePath := filepath.Join(sshDir, tenantID)
	if err := os.WriteFile(privatePath, []byte(key.Private), 0600); err != nil {
		return err
	}
	if err := os.Chmod(privatePath, 0600); err != nil {
		return err
	}
	publicPath := filepath.Join(sshDir, tenantID+".pub")
	if err := os.WriteFile(publicPath, []byte(key.Public), 0644); err != nil {
		return err
	}
	return os.Chmod(publicPath, 0644)
}

func tenantSSHKeyNamespace() string {
	if configs.Default() != nil && configs.Default().PublicConfig != nil && configs.Default().PublicConfig.RbdNamespace != "" {
		return configs.Default().PublicConfig.RbdNamespace
	}
	return util.GetenvDefault("RBD_NAMESPACE", constants.Namespace)
}

func defaultKubernetesClient() kubernetes.Interface {
	if componentK8s.Default() == nil {
		return nil
	}
	return componentK8s.Default().Clientset
}

func tenantSSHKeySecretName(tenantID string) string {
	sum := sha1.Sum([]byte(tenantID))
	return fmt.Sprintf("rbd-builder-ssh-key-%x", sum)
}

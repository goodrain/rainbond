package sources

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestTenantSSHKeyPersistsInSecretAndSyncsAcrossHomes(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	namespace := "rbd-system"
	tenantID := "tenant-a"
	firstHome := t.TempDir()
	secondHome := t.TempDir()

	firstKey, err := getOrCreateTenantSSHKey(ctx, tenantID, firstHome, namespace, client)
	if err != nil {
		t.Fatalf("get or create first key: %v", err)
	}
	if firstKey.Private == "" || firstKey.Public == "" {
		t.Fatalf("expected generated key pair, got private=%q public=%q", firstKey.Private, firstKey.Public)
	}

	secret, err := client.CoreV1().Secrets(namespace).Get(ctx, tenantSSHKeySecretName(tenantID), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get key secret: %v", err)
	}
	assertTenantSSHKeySecret(t, secret, firstKey)

	secondKey, err := getOrCreateTenantSSHKey(ctx, tenantID, secondHome, namespace, client)
	if err != nil {
		t.Fatalf("get or create second key: %v", err)
	}
	if secondKey != firstKey {
		t.Fatalf("expected second home to reuse secret key, got %#v want %#v", secondKey, firstKey)
	}
	assertTenantSSHKeyFiles(t, secondHome, tenantID, firstKey)
}

func TestTenantSSHKeyStoresExistingLocalKeyInSecret(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	namespace := "rbd-system"
	tenantID := "tenant-existing"
	home := t.TempDir()

	private, public, err := MakeSSHKeyPair()
	if err != nil {
		t.Fatalf("make key pair: %v", err)
	}
	existingKey := tenantSSHKeyPair{Private: private, Public: public}
	writeLocalTenantSSHKeyForTest(t, home, tenantID, existingKey)

	got, err := getOrCreateTenantSSHKey(ctx, tenantID, home, namespace, client)
	if err != nil {
		t.Fatalf("get or create key: %v", err)
	}
	if got != existingKey {
		t.Fatalf("expected existing local key to be preserved, got %#v want %#v", got, existingKey)
	}

	secret, err := client.CoreV1().Secrets(namespace).Get(ctx, tenantSSHKeySecretName(tenantID), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get key secret: %v", err)
	}
	assertTenantSSHKeySecret(t, secret, existingKey)
}

func TestGetPrivateFileSyncsTenantKeyFromSecret(t *testing.T) {
	ctx := context.Background()
	namespace := "rbd-system"
	tenantID := "tenant-build"
	home := t.TempDir()
	private, public, err := MakeSSHKeyPair()
	if err != nil {
		t.Fatalf("make key pair: %v", err)
	}
	expected := tenantSSHKeyPair{Private: private, Public: public}
	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tenantSSHKeySecretName(tenantID),
			Namespace: namespace,
		},
		Data: map[string][]byte{
			tenantSSHPrivateKeyDataKey: []byte(expected.Private),
			tenantSSHPublicKeyDataKey:  []byte(expected.Public),
		},
	})

	got := getPrivateFile(ctx, tenantID, home, namespace, client)
	want := filepath.Join(home, ".ssh", tenantID)
	if got != want {
		t.Fatalf("expected tenant private key path, got %q want %q", got, want)
	}
	assertTenantSSHKeyFiles(t, home, tenantID, expected)
}

func TestGetPrivateFilePrefersSecretOverStaleLocalKey(t *testing.T) {
	ctx := context.Background()
	namespace := "rbd-system"
	tenantID := "tenant-stale"
	home := t.TempDir()

	stalePrivate, stalePublic, err := MakeSSHKeyPair()
	if err != nil {
		t.Fatalf("make stale key pair: %v", err)
	}
	writeLocalTenantSSHKeyForTest(t, home, tenantID, tenantSSHKeyPair{Private: stalePrivate, Public: stalePublic})

	private, public, err := MakeSSHKeyPair()
	if err != nil {
		t.Fatalf("make secret key pair: %v", err)
	}
	expected := tenantSSHKeyPair{Private: private, Public: public}
	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tenantSSHKeySecretName(tenantID),
			Namespace: namespace,
		},
		Data: map[string][]byte{
			tenantSSHPrivateKeyDataKey: []byte(expected.Private),
			tenantSSHPublicKeyDataKey:  []byte(expected.Public),
		},
	})

	got := getPrivateFile(ctx, tenantID, home, namespace, client)
	want := filepath.Join(home, ".ssh", tenantID)
	if got != want {
		t.Fatalf("expected tenant private key path, got %q want %q", got, want)
	}
	assertTenantSSHKeyFiles(t, home, tenantID, expected)
}

func assertTenantSSHKeySecret(t *testing.T, secret *corev1.Secret, expected tenantSSHKeyPair) {
	t.Helper()
	if string(secret.Data[tenantSSHPrivateKeyDataKey]) != expected.Private {
		t.Fatalf("secret private key mismatch")
	}
	if string(secret.Data[tenantSSHPublicKeyDataKey]) != expected.Public {
		t.Fatalf("secret public key mismatch")
	}
}

func assertTenantSSHKeyFiles(t *testing.T, home, tenantID string, expected tenantSSHKeyPair) {
	t.Helper()
	privatePath := filepath.Join(home, ".ssh", tenantID)
	publicPath := filepath.Join(home, ".ssh", tenantID+".pub")

	private, err := os.ReadFile(privatePath)
	if err != nil {
		t.Fatalf("read private key: %v", err)
	}
	if string(private) != expected.Private {
		t.Fatalf("local private key mismatch")
	}

	public, err := os.ReadFile(publicPath)
	if err != nil {
		t.Fatalf("read public key: %v", err)
	}
	if string(public) != expected.Public {
		t.Fatalf("local public key mismatch")
	}
}

func writeLocalTenantSSHKeyForTest(t *testing.T, home, tenantID string, key tenantSSHKeyPair) {
	t.Helper()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatalf("create ssh dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, tenantID), []byte(key.Private), 0600); err != nil {
		t.Fatalf("write private key: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, tenantID+".pub"), []byte(key.Public), 0644); err != nil {
		t.Fatalf("write public key: %v", err)
	}
}

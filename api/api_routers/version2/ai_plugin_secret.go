package version2

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/goodrain/rainbond/api/middleware"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	utils "github.com/goodrain/rainbond/util"
	httputil "github.com/goodrain/rainbond/util/http"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const aiPlatformSecretName = "rainbond-ai-engine-platform"

type aiPlatformSecrets interface {
	Get(context.Context, string, metav1.GetOptions) (*corev1.Secret, error)
	Create(context.Context, *corev1.Secret, metav1.CreateOptions) (*corev1.Secret, error)
}

func aiPlatformNamespace() string {
	return utils.GetenvDefault("RBD_NAMESPACE", "rbd-system")
}

func systemAIPlatformSecrets() aiPlatformSecrets {
	client := k8s.Default()
	if client == nil || client.Clientset == nil {
		return nil
	}
	return client.Clientset.CoreV1().Secrets(aiPlatformNamespace())
}

func aiPlatformSecretToken(secret *corev1.Secret) (string, error) {
	if secret == nil || secret.Name != aiPlatformSecretName || secret.Namespace != aiPlatformNamespace() || secret.Type != corev1.SecretTypeOpaque {
		return "", errors.New("invalid AI platform Secret")
	}
	token := string(secret.Data["token"])
	decoded, err := hex.DecodeString(token)
	if err != nil || len(token) != 64 || len(decoded) != 32 {
		return "", errors.New("invalid AI platform credential")
	}
	return token, nil
}

func readAIPlatformToken(ctx context.Context, secrets aiPlatformSecrets) (string, error) {
	if secrets == nil {
		return "", errors.New("AI platform Secret client unavailable")
	}
	secret, err := secrets.Get(ctx, aiPlatformSecretName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return aiPlatformSecretToken(secret)
}

func ensureAIPlatformToken(ctx context.Context, secrets aiPlatformSecrets) (string, error) {
	if secrets == nil {
		return "", errors.New("AI platform Secret client unavailable")
	}
	secret, err := secrets.Get(ctx, aiPlatformSecretName, metav1.GetOptions{})
	if err == nil {
		return aiPlatformSecretToken(secret)
	}
	if !apierrors.IsNotFound(err) {
		return "", err
	}
	var entropy [32]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", err
	}
	secret, err = secrets.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: aiPlatformSecretName, Namespace: aiPlatformNamespace()},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{"token": []byte(hex.EncodeToString(entropy[:]))},
	}, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		// Concurrent installers converge on the first persisted credential.
		return readAIPlatformToken(ctx, secrets)
	}
	if err != nil {
		return "", err
	}
	return aiPlatformSecretToken(secret)
}

func aiPlatformTokenBootstrap(getSecrets func() aiPlatformSecrets) http.Handler {
	return middleware.CleanupIdentity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		token, err := ensureAIPlatformToken(r.Context(), getSecrets())
		if err != nil {
			// API/client error text can contain sensitive data; never echo it.
			httputil.ReturnError(r, w, http.StatusServiceUnavailable, "AI_PLATFORM_CREDENTIAL_UNAVAILABLE")
			return
		}
		httputil.ReturnSuccess(r, w, struct {
			Token string `json:"token"`
		}{token})
	}))
}

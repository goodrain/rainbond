package version2

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

// capability_id: rainbond.plugin.ai-platform-secret-bootstrap
func TestAIPlatformSecretBootstrapIsFixedIdempotentAndConcurrent(t *testing.T) {
	t.Setenv("RBD_NAMESPACE", "fixture-platform")
	client := fake.NewSimpleClientset()
	secrets := client.CoreV1().Secrets("fixture-platform")
	const workers = 12
	values := make(chan string, workers)
	failures := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := ensureAIPlatformToken(context.Background(), secrets)
			values <- value
			failures <- err
		}()
	}
	wg.Wait()
	close(values)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	first := ""
	for value := range values {
		if first == "" {
			first = value
		}
		if value != first || len(value) != 64 {
			t.Fatal("concurrent bootstrap did not converge on one opaque token")
		}
	}
	stored, err := secrets.Get(context.Background(), "rainbond-ai-engine-platform", metav1.GetOptions{})
	if err != nil || stored.Type != corev1.SecretTypeOpaque || string(stored.Data["token"]) != first {
		t.Fatal("bootstrap did not store the token in the fixed AI Secret")
	}
	if again, err := ensureAIPlatformToken(context.Background(), secrets); err != nil || again != first {
		t.Fatal("bootstrap changed an existing credential")
	}
	list, err := secrets.List(context.Background(), metav1.ListOptions{})
	if err != nil || len(list.Items) != 1 {
		t.Fatal("bootstrap created additional resources")
	}
}

func TestAIPlatformSecretFailuresDoNotCreateOrExposeCredentials(t *testing.T) {
	t.Setenv("TOKEN", "region-fixture-token")
	t.Setenv("RBD_NAMESPACE", "fixture-platform")
	for _, tc := range []struct {
		name   string
		secret *corev1.Secret
	}{
		{name: "missing secret"},
		{name: "malformed token", secret: &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "rainbond-ai-engine-platform", Namespace: "fixture-platform"}, Type: corev1.SecretTypeOpaque, Data: map[string][]byte{"token": []byte("bad\nfixture")}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			if tc.secret != nil {
				_, _ = client.CoreV1().Secrets("fixture-platform").Create(context.Background(), tc.secret, metav1.CreateOptions{})
			}
			client.ClearActions()
			if _, err := readAIPlatformToken(context.Background(), client.CoreV1().Secrets("fixture-platform")); err == nil {
				t.Fatal("invalid read-only credential was accepted")
			}
			for _, action := range client.Actions() {
				if action.GetVerb() != "get" {
					t.Fatal("proxy credential read mutated resources")
				}
			}
			if tc.secret != nil {
				if _, err := ensureAIPlatformToken(context.Background(), client.CoreV1().Secrets("fixture-platform")); err == nil {
					t.Fatal("bootstrap silently replaced malformed credential")
				}
			}
		})
	}
	client := fake.NewSimpleClientset()
	client.PrependReactor("get", "secrets", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("fixture credential-bearing upstream error")
	})
	if _, err := ensureAIPlatformToken(context.Background(), client.CoreV1().Secrets("fixture-platform")); err == nil {
		t.Fatal("bootstrap accepted unreadable Secret")
	}
	for _, action := range client.Actions() {
		if action.GetVerb() != "get" {
			t.Fatal("read error triggered resource creation")
		}
	}
	client = fake.NewSimpleClientset()
	client.PrependReactor("create", "secrets", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("fixture-sensitive-create-error")
	})
	request := httptest.NewRequest(http.MethodPost, "/v2/platform/plugins/rainbond-ai-engine/platform-token", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Token region-fixture-token")
	response := httptest.NewRecorder()
	aiPlatformTokenBootstrap(func() aiPlatformSecrets { return client.CoreV1().Secrets("fixture-platform") }).ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), "fixture-sensitive") {
		t.Fatal("bootstrap did not fail closed with a sanitized error")
	}
}

func TestAIManagementProxyNeverBootstrapsMissingSecret(t *testing.T) {
	t.Setenv("TOKEN", "region-fixture-token")
	t.Setenv("RBD_NAMESPACE", "fixture-platform")
	client := fake.NewSimpleClientset()
	router := aiPlatformSecretTestRouter(t, client.CoreV1().Secrets("fixture-platform"), "http://127.0.0.1:1")
	request := httptest.NewRequest(http.MethodPost, "/v2/platform/backend/plugins/rainbond-ai-engine/api/v1/ai-engine/model-apis", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Token region-fixture-token")
	request.Header.Set("X-AI-Platform-Token", aiPlatformFixtureToken)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), aiPlatformFixtureToken) {
		t.Fatal("proxy did not fail closed on a missing credential")
	}
	for _, action := range client.Actions() {
		if action.GetVerb() != "get" {
			t.Fatal("proxy bootstrapped or changed a Secret")
		}
	}
}

func TestAIPlatformBootstrapAndProxyRequireRealPlatformTLSOrToken(t *testing.T) {
	t.Setenv("TOKEN", "")
	t.Setenv("RBD_NAMESPACE", "fixture-platform")
	client := fake.NewSimpleClientset()
	secrets := client.CoreV1().Secrets("fixture-platform")
	var received atomic.Value
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-AI-Platform-Token")
		received.Store(token)
		if token == "" || r.Header.Get("Authorization") != "" {
			t.Error("proxy did not use the isolated platform credential")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	router := aiPlatformSecretTestRouter(t, secrets, upstream.URL)
	plain := httptest.NewServer(router)
	defer plain.Close()
	for _, path := range []string{"/v2/platform/plugins/rainbond-ai-engine/platform-token", "/v2/platform/backend/plugins/rainbond-ai-engine/api/v1/ai-engine/model-apis"} {
		req, _ := http.NewRequest(http.MethodPost, plain.URL+path, strings.NewReader(`{}`))
		req.Header.Set("X-Forwarded-Client-Cert", "forged-fixture")
		req.Header.Set("X-AI-Platform-Token", aiPlatformFixtureToken)
		req.Header.Set("X-AI-Team-Name", "forged-team")
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("anonymous bootstrap/proxy status = %d", response.StatusCode)
		}
	}
	if len(client.Actions()) != 0 {
		t.Fatal("anonymous request accessed the Secret")
	}

	ca, certificate := aiPlatformClientCertificate(t)
	server := httptest.NewUnstartedServer(router)
	server.TLS = &tls.Config{ClientCAs: ca, ClientAuth: tls.RequireAndVerifyClientCert, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	defer server.Close()
	mtls := server.Client()
	mtls.Transport.(*http.Transport).TLSClientConfig.Certificates = []tls.Certificate{certificate}
	response, err := mtls.Post(server.URL+"/v2/platform/plugins/rainbond-ai-engine/platform-token", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Bean struct {
			Token string `json:"token"`
		} `json:"bean"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || len(envelope.Bean.Token) != 64 || response.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("real mTLS bootstrap failed")
	}
	client.ClearActions()
	response, err = mtls.Post(server.URL+"/v2/platform/backend/plugins/rainbond-ai-engine/api/v1/ai-engine/model-apis", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent || received.Load() != envelope.Bean.Token {
		t.Fatal("tokenless verified mTLS proxy did not reuse the bootstrap Secret")
	}
	for _, action := range client.Actions() {
		if action.GetVerb() != "get" {
			t.Fatal("request proxy created or changed a resource")
		}
	}

	// A configured Region TOKEN keeps its existing precedence over TLS identity.
	t.Setenv("TOKEN", "region-fixture-token")
	for _, path := range []string{"/v2/platform/plugins/rainbond-ai-engine/platform-token", "/v2/platform/backend/plugins/rainbond-ai-engine/api/v1/ai-engine/model-apis"} {
		response, err = mtls.Post(server.URL+path, "application/json", strings.NewReader(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatal("verified TLS bypassed the configured Region token")
		}
	}
	req, _ := http.NewRequest(http.MethodPost, plain.URL+"/v2/platform/plugins/rainbond-ai-engine/platform-token", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Token region-fixture-token")
	response, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), envelope.Bean.Token) {
		t.Fatal("configured Region token bootstrap did not reuse credential")
	}
}

func aiPlatformSecretTestRouter(t *testing.T, secrets aiPlatformSecrets, backend string) http.Handler {
	t.Helper()
	router := chi.NewRouter()
	router.Method(http.MethodPost, "/v2/platform/plugins/rainbond-ai-engine/platform-token", aiPlatformTokenBootstrap(func() aiPlatformSecrets { return secrets }))
	router.Post("/v2/platform/backend/plugins/{plugin_name}/*", func(w http.ResponseWriter, r *http.Request) {
		servePluginBackendWithToken(w, r, func(name string) (*v1alpha1.RBDPlugin, error) {
			plugin := testRBDPlugin("")
			plugin.Name = name
			plugin.Spec.BackendService = backend
			return plugin, nil
		}, func(ctx context.Context) (string, error) { return readAIPlatformToken(ctx, secrets) })
	})
	return router
}

func aiPlatformClientCertificate(t *testing.T) (*x509.CertPool, tls.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "AI fixture CA"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "AI fixture Console"}, NotBefore: ca.NotBefore, NotAfter: ca.NotAfter, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, KeyUsage: x509.KeyUsageDigitalSignature}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}))
	return pool, certificate
}

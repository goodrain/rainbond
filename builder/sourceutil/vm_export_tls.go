package sourceutil

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	vmExportGVR = schema.GroupVersionResource{
		Group:    "export.kubevirt.io",
		Version:  "v1beta1",
		Resource: "virtualmachineexports",
	}
	vmExportTokenHeader           = "x-kubevirt-export-token"
	vmExportDynamicClientProvider = func() dynamic.Interface {
		component := k8s.Default()
		if component == nil || component.DynamicClient == nil {
			return nil
		}
		return component.DynamicClient
	}
	vmExportSecretGetter = func(namespace, name string) ([]byte, error) {
		component := k8s.Default()
		if component == nil || component.Clientset == nil {
			return nil, nil
		}
		if strings.TrimSpace(namespace) == "" || strings.TrimSpace(name) == "" {
			return nil, nil
		}
		secret, err := component.Clientset.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}
		if token, ok := secret.Data["token"]; ok && len(token) > 0 {
			return token, nil
		}
		for _, token := range secret.Data {
			if len(token) > 0 {
				return token, nil
			}
		}
		return nil, nil
	}
)

type vmExportAuthConfig struct {
	CertPEM    []byte
	Token      []byte
	ExportName string
	Namespace  string
	Source     string
}

type VMExportAuthConfig struct {
	CertPEM    []byte
	Token      []byte
	ExportName string
	Namespace  string
	Source     string
}

// SetVMExportDynamicClientProviderForTest temporarily overrides the VM export dynamic client provider.
func SetVMExportDynamicClientProviderForTest(provider func() dynamic.Interface) func() {
	original := vmExportDynamicClientProvider
	vmExportDynamicClientProvider = provider
	return func() {
		vmExportDynamicClientProvider = original
	}
}

// SetVMExportSecretGetterForTest temporarily overrides the VM export token secret lookup.
func SetVMExportSecretGetterForTest(getter func(namespace, name string) ([]byte, error)) func() {
	original := vmExportSecretGetter
	vmExportSecretGetter = getter
	return func() {
		vmExportSecretGetter = original
	}
}

// NewRemotePackageHTTPClient returns a client that trusts the matching VM export CA when available.
func NewRemotePackageHTTPClient(rawURL string) *http.Client {
	client := cloneDefaultHTTPClient()
	authConfig, err := lookupVMExportAuthByURL(rawURL)
	if err != nil {
		logrus.Warningf("lookup vm export auth for %s failed: %v", rawURL, err)
		return client
	}
	if len(authConfig.CertPEM) == 0 && len(authConfig.Token) == 0 {
		return client
	}
	logrus.Infof(
		"vm export auth matched: url=%s export=%s namespace=%s source=%s cert=%t token=%t",
		rawURL,
		authConfig.ExportName,
		authConfig.Namespace,
		authConfig.Source,
		len(authConfig.CertPEM) > 0,
		len(authConfig.Token) > 0,
	)

	transport, ok := cloneHTTPTransport(client.Transport)
	if !ok {
		logrus.Warningf("skip vm export auth injection for %s: unsupported transport %T", rawURL, client.Transport)
		return client
	}

	if len(authConfig.CertPEM) > 0 {
		pool, err := currentCertPool(transport.TLSClientConfig)
		if err != nil {
			logrus.Warningf("prepare tls root CAs for %s failed: %v", rawURL, err)
			return client
		}
		if ok := pool.AppendCertsFromPEM(authConfig.CertPEM); !ok {
			logrus.Warningf("append vm export cert for %s failed", rawURL)
			return client
		}

		tlsConfig := cloneTLSConfig(transport.TLSClientConfig)
		tlsConfig.RootCAs = pool
		transport.TLSClientConfig = tlsConfig
	}
	client.Transport = transport
	if len(authConfig.Token) > 0 {
		client.Transport = &vmExportTokenRoundTripper{
			base:  transport,
			token: strings.TrimSpace(string(authConfig.Token)),
		}
	}
	logrus.Infof(
		"vm export auth injected: url=%s export=%s namespace=%s source=%s cert=%t token=%t",
		rawURL,
		authConfig.ExportName,
		authConfig.Namespace,
		authConfig.Source,
		len(authConfig.CertPEM) > 0,
		len(authConfig.Token) > 0,
	)
	return client
}

func LookupVMExportAuthConfigByURL(rawURL string) (VMExportAuthConfig, error) {
	authConfig, err := lookupVMExportAuthByURL(rawURL)
	if err != nil {
		return VMExportAuthConfig{}, err
	}
	return VMExportAuthConfig{
		CertPEM:    authConfig.CertPEM,
		Token:      authConfig.Token,
		ExportName: authConfig.ExportName,
		Namespace:  authConfig.Namespace,
		Source:     authConfig.Source,
	}, nil
}

func lookupVMExportAuthByURL(rawURL string) (vmExportAuthConfig, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return vmExportAuthConfig{}, nil
	}

	dynamicClient := vmExportDynamicClientProvider()
	if dynamicClient == nil {
		return vmExportAuthConfig{}, nil
	}

	list, err := dynamicClient.Resource(vmExportGVR).Namespace(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return vmExportAuthConfig{}, err
	}
	for _, item := range list.Items {
		authConfig, matched, err := extractVMExportAuthForURL(item.Object, rawURL)
		if err != nil {
			return vmExportAuthConfig{}, err
		}
		if matched {
			return authConfig, nil
		}
	}
	return vmExportAuthConfig{}, nil
}

func extractVMExportAuthForURL(obj map[string]interface{}, rawURL string) (vmExportAuthConfig, bool, error) {
	for _, fields := range [][]string{
		{"status", "links", "internal"},
		{"status", "links", "external"},
	} {
		link, found, err := unstructured.NestedMap(obj, fields...)
		if err != nil || !found {
			continue
		}
		volumes, found, err := unstructured.NestedSlice(link, "volumes")
		if err != nil || !found || !vmExportVolumesContainURL(volumes, rawURL) {
			continue
		}
		certValue, _, _ := unstructured.NestedString(link, "cert")
		certPEM, err := decodeVMExportCert(certValue)
		if err != nil {
			return vmExportAuthConfig{}, true, err
		}
		tokenSecretRef := extractVMExportTokenSecretRef(obj)
		namespace, _, _ := unstructured.NestedString(obj, "metadata", "namespace")
		token, err := vmExportSecretGetter(namespace, tokenSecretRef)
		if err != nil {
			return vmExportAuthConfig{}, true, err
		}
		return vmExportAuthConfig{
			CertPEM:    certPEM,
			Token:      token,
			ExportName: getNestedString(obj, "metadata", "name"),
			Namespace:  namespace,
			Source:     fields[len(fields)-1],
		}, true, nil
	}
	return vmExportAuthConfig{}, false, nil
}

func vmExportVolumesContainURL(volumes []interface{}, rawURL string) bool {
	for _, volumeValue := range volumes {
		volume, ok := volumeValue.(map[string]interface{})
		if !ok {
			continue
		}
		formats, found, err := unstructured.NestedSlice(volume, "formats")
		if err != nil || !found {
			continue
		}
		for _, formatValue := range formats {
			formatItem, ok := formatValue.(map[string]interface{})
			if !ok {
				continue
			}
			url, _, _ := unstructured.NestedString(formatItem, "url")
			if strings.TrimSpace(url) == rawURL {
				return true
			}
		}
	}
	return false
}

func decodeVMExportCert(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	if strings.Contains(trimmed, "BEGIN CERTIFICATE") {
		return []byte(trimmed), nil
	}
	decoded, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("decode vm export cert failed: %w", err)
	}
	return decoded, nil
}

func extractVMExportTokenSecretRef(obj map[string]interface{}) string {
	if tokenSecretRef, _, _ := unstructured.NestedString(obj, "status", "tokenSecretRef"); strings.TrimSpace(tokenSecretRef) != "" {
		return tokenSecretRef
	}
	tokenSecretRef, _, _ := unstructured.NestedString(obj, "spec", "tokenSecretRef")
	return tokenSecretRef
}

func getNestedString(obj map[string]interface{}, fields ...string) string {
	value, _, _ := unstructured.NestedString(obj, fields...)
	return value
}

type vmExportTokenRoundTripper struct {
	base  http.RoundTripper
	token string
}

func (r *vmExportTokenRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}
	cloned := req.Clone(req.Context())
	if strings.TrimSpace(r.token) != "" && cloned.Header.Get(vmExportTokenHeader) == "" {
		cloned.Header.Set(vmExportTokenHeader, r.token)
	}
	base := r.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(cloned)
}

func cloneDefaultHTTPClient() *http.Client {
	if http.DefaultClient == nil {
		return &http.Client{}
	}
	cloned := *http.DefaultClient
	return &cloned
}

func cloneHTTPTransport(transport http.RoundTripper) (*http.Transport, bool) {
	switch typed := transport.(type) {
	case nil:
		defaultTransport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return nil, false
		}
		return defaultTransport.Clone(), true
	case *http.Transport:
		return typed.Clone(), true
	default:
		return nil, false
	}
}

func cloneTLSConfig(cfg *tls.Config) *tls.Config {
	if cfg == nil {
		return &tls.Config{}
	}
	return cfg.Clone()
}

func currentCertPool(cfg *tls.Config) (*x509.CertPool, error) {
	if cfg != nil && cfg.RootCAs != nil {
		return cfg.RootCAs.Clone(), nil
	}
	systemPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}
	if systemPool != nil {
		return systemPool, nil
	}
	return x509.NewCertPool(), nil
}

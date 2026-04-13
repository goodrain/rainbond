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
	vmExportDynamicClientProvider = func() dynamic.Interface {
		component := k8s.Default()
		if component == nil || component.DynamicClient == nil {
			return nil
		}
		return component.DynamicClient
	}
)

// SetVMExportDynamicClientProviderForTest temporarily overrides the VM export dynamic client provider.
func SetVMExportDynamicClientProviderForTest(provider func() dynamic.Interface) func() {
	original := vmExportDynamicClientProvider
	vmExportDynamicClientProvider = provider
	return func() {
		vmExportDynamicClientProvider = original
	}
}

// NewRemotePackageHTTPClient returns a client that trusts the matching VM export CA when available.
func NewRemotePackageHTTPClient(rawURL string) *http.Client {
	client := cloneDefaultHTTPClient()
	certPEM, err := lookupVMExportCertByURL(rawURL)
	if err != nil {
		logrus.Warningf("lookup vm export cert for %s failed: %v", rawURL, err)
		return client
	}
	if len(certPEM) == 0 {
		return client
	}

	transport, ok := cloneHTTPTransport(client.Transport)
	if !ok {
		logrus.Warningf("skip vm export cert injection for %s: unsupported transport %T", rawURL, client.Transport)
		return client
	}

	pool, err := currentCertPool(transport.TLSClientConfig)
	if err != nil {
		logrus.Warningf("prepare tls root CAs for %s failed: %v", rawURL, err)
		return client
	}
	if ok := pool.AppendCertsFromPEM(certPEM); !ok {
		logrus.Warningf("append vm export cert for %s failed", rawURL)
		return client
	}

	tlsConfig := cloneTLSConfig(transport.TLSClientConfig)
	tlsConfig.RootCAs = pool
	transport.TLSClientConfig = tlsConfig
	client.Transport = transport
	return client
}

func lookupVMExportCertByURL(rawURL string) ([]byte, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, nil
	}

	dynamicClient := vmExportDynamicClientProvider()
	if dynamicClient == nil {
		return nil, nil
	}

	list, err := dynamicClient.Resource(vmExportGVR).Namespace(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, item := range list.Items {
		certPEM, err := extractVMExportCertForURL(item.Object, rawURL)
		if err != nil {
			return nil, err
		}
		if len(certPEM) > 0 {
			return certPEM, nil
		}
	}
	return nil, nil
}

func extractVMExportCertForURL(obj map[string]interface{}, rawURL string) ([]byte, error) {
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
		return decodeVMExportCert(certValue)
	}
	return nil, nil
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

package sourceutil

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"strings"
)

// NewRemotePackageHTTPClient returns a clone of the default client for remote package access.
func NewRemotePackageHTTPClient(rawURL string) *http.Client {
	client := cloneDefaultHTTPClient()
	client.Transport = cloneHTTPTransport(client.Transport)
	if shouldSkipTLSVerifyForVMExport(rawURL) {
		transport, ok := client.Transport.(*http.Transport)
		if ok {
			tlsConfig := &tls.Config{}
			if transport.TLSClientConfig != nil {
				tlsConfig = transport.TLSClientConfig.Clone()
			}
			tlsConfig.InsecureSkipVerify = true
			transport.TLSClientConfig = tlsConfig
		}
	}
	return client
}

func cloneDefaultHTTPClient() *http.Client {
	if http.DefaultClient == nil {
		return &http.Client{}
	}
	cloned := *http.DefaultClient
	return &cloned
}

func cloneHTTPTransport(roundTripper http.RoundTripper) http.RoundTripper {
	if roundTripper == nil {
		roundTripper = http.DefaultTransport
	}
	transport, ok := roundTripper.(*http.Transport)
	if !ok {
		return roundTripper
	}
	return transport.Clone()
}

func shouldSkipTLSVerifyForVMExport(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if !strings.HasPrefix(host, "virt-export-") {
		return false
	}
	return strings.HasSuffix(host, ".svc") || strings.HasSuffix(host, ".svc.cluster.local")
}

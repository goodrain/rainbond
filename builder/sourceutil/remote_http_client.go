package sourceutil

import "net/http"

// NewRemotePackageHTTPClient returns a clone of the default client for remote package access.
func NewRemotePackageHTTPClient(rawURL string) *http.Client {
	_ = rawURL
	return cloneDefaultHTTPClient()
}

func cloneDefaultHTTPClient() *http.Client {
	if http.DefaultClient == nil {
		return &http.Client{}
	}
	cloned := *http.DefaultClient
	return &cloned
}

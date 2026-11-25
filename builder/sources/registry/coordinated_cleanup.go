package registry

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// ErrCoordinatedCleanup reports a rejected or unverified cleanup operation.
var ErrCoordinatedCleanup = errors.New("coordinated registry cleanup rejected")

// ErrCleanupManifestAbsent reports that the requested manifest does not exist.
var ErrCleanupManifestAbsent = errors.New("registry cleanup manifest absent")
var cleanupRepository = regexp.MustCompile(`^[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*(?:/[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*)*$`)
var cleanupDigest = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var cleanupTag = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)

// CoordinatedCleanupClient is limited to advisory reads and one exact-target,
// permitted DELETE. Its transport never follows redirects or retries auth.
// The caller must derive this endpoint from the verified system Registry.
type CoordinatedCleanupClient struct {
	endpoint, user, pass string
	client               *http.Client
}

// NewCoordinatedCleanupClient binds cleanup requests to a verified Registry origin.
func NewCoordinatedCleanupClient(endpoint, user, pass string) (*CoordinatedCleanupClient, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, ErrCoordinatedCleanup
	}
	return &CoordinatedCleanupClient{endpoint: strings.TrimSuffix(endpoint, "/"), user: user, pass: pass, client: &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{ResponseHeaderTimeout: 15 * time.Second, IdleConnTimeout: 30 * time.Second}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *CoordinatedCleanupClient) request(ctx context.Context, method, path, permit string) (*http.Response, error) {
	if c == nil || c.client == nil {
		return nil, ErrCoordinatedCleanup
	}
	request, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, nil)
	if err != nil {
		return nil, ErrCoordinatedCleanup
	}
	request.Header.Set("Accept", acceptedManifestMediaTypes())
	if c.user != "" || c.pass != "" {
		request.SetBasicAuth(c.user, c.pass)
	}
	if permit != "" {
		request.Header.Set("X-Rainbond-Cleanup-Permit", permit)
	}
	response, err := c.client.Do(request)
	if err != nil {
		return nil, ErrCoordinatedCleanup
	}
	return response, nil
}
func cleanupClose(response *http.Response) {
	io.Copy(io.Discard, io.LimitReader(response.Body, 1024))
	response.Body.Close()
}
func validCleanupRepository(repository string) bool {
	return len(repository) <= 255 && cleanupRepository.MatchString(repository)
}

// Resolve reads the immutable digest of a manifest without changing its tags.
func (c *CoordinatedCleanupClient) Resolve(ctx context.Context, repository, reference string) (string, error) {
	if !validCleanupRepository(repository) || (!cleanupTag.MatchString(reference) && !cleanupDigest.MatchString(reference)) {
		return "", ErrCoordinatedCleanup
	}
	response, err := c.request(ctx, http.MethodHead, "/v2/"+repository+"/manifests/"+reference, "")
	if err != nil {
		return "", err
	}
	defer cleanupClose(response)
	if response.StatusCode == http.StatusNotFound {
		return "", ErrCleanupManifestAbsent
	}
	value := response.Header.Get("Docker-Content-Digest")
	if response.StatusCode != http.StatusOK || !cleanupDigest.MatchString(value) || (cleanupDigest.MatchString(reference) && reference != value) {
		return "", ErrCoordinatedCleanup
	}
	return value, nil
}

// Tags refuses incomplete pagination; guessing a missing alias could delete a
// retained manifest shared by that alias. Oversized repositories are skipped.
func (c *CoordinatedCleanupClient) Tags(ctx context.Context, repository string) ([]string, error) {
	if !validCleanupRepository(repository) {
		return nil, ErrCoordinatedCleanup
	}
	response, err := c.request(ctx, http.MethodGet, "/v2/"+repository+"/tags/list?n=1001", "")
	if err != nil {
		return nil, err
	}
	defer cleanupClose(response)
	if response.StatusCode != http.StatusOK || response.Header.Get("Link") != "" {
		return nil, ErrCoordinatedCleanup
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20+1))
	if err != nil || len(raw) > 1<<20 {
		return nil, ErrCoordinatedCleanup
	}
	var body struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}
	if json.Unmarshal(raw, &body) != nil || body.Name != repository || len(body.Tags) > 1000 {
		return nil, ErrCoordinatedCleanup
	}
	seen := map[string]bool{}
	for _, tag := range body.Tags {
		if !cleanupTag.MatchString(tag) || seen[tag] {
			return nil, ErrCoordinatedCleanup
		}
		seen[tag] = true
	}
	return body.Tags, nil
}

// Delete sends one exact-digest deletion with its coordination permit.
func (c *CoordinatedCleanupClient) Delete(ctx context.Context, repository, digest, permit string) (int, error) {
	if !validCleanupRepository(repository) || !cleanupDigest.MatchString(digest) || permit == "" || len(permit) > 16384 || strings.ContainsAny(permit, "\r\n\x00") {
		return 0, ErrCoordinatedCleanup
	}
	response, err := c.request(ctx, http.MethodDelete, "/v2/"+repository+"/manifests/"+digest, permit)
	if err != nil {
		return 0, err
	}
	defer cleanupClose(response)
	return response.StatusCode, nil
}

// Exists verifies whether an exact manifest digest remains in the Registry.
func (c *CoordinatedCleanupClient) Exists(ctx context.Context, repository, digest string) (bool, error) {
	if !cleanupDigest.MatchString(digest) {
		return false, ErrCoordinatedCleanup
	}
	_, err := c.Resolve(ctx, repository, digest)
	if errors.Is(err, ErrCleanupManifestAbsent) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

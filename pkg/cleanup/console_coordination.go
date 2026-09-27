package cleanup

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ConsoleCoordinationScope uses the installation-owned key, never an administrator token.
type ConsoleCoordinationScope struct {
	Enterprise, Region string
	Key                []byte `json:"-"`
}

// Configured distinguishes signed Console access from direct Core credentials.
func (c ConsoleCoordinationScope) Configured() bool {
	return c.Enterprise != "" || c.Region != "" || len(c.Key) > 0
}

type consoleCoordinationTransport struct {
	endpoint *url.URL
	scope    ConsoleCoordinationScope
	next     http.RoundTripper
}

var consoleCoordinationScope = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// NewConsole wraps only cleanup POSTs into signed, bounded Console requests.
func newConsoleCoordinationTransport(endpoint string, scope ConsoleCoordinationScope, next http.RoundTripper) (http.RoundTripper, error) {
	origin, err := url.Parse(endpoint)
	if err != nil || origin.Host == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" || (origin.Path != "" && origin.Path != "/") || (origin.Scheme != "https" && origin.Scheme != "http") || !scope.ValidIdentity() || len(scope.Key) < 32 || len(scope.Key) > 4096 || next == nil {
		return nil, ErrCoordinationChanged
	}
	scope.Key = append([]byte(nil), scope.Key...)
	return &consoleCoordinationTransport{endpoint: origin, scope: scope, next: next}, nil
}

func (c *consoleCoordinationTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.Method != "POST" || request.URL.Scheme != c.endpoint.Scheme || request.URL.Host != c.endpoint.Host || !strings.HasPrefix(request.URL.Path, "/v2/cleanup/") || request.URL.RawQuery != "" || request.URL.RawPath != "" || request.URL.Fragment != "" {
		return nil, ErrCoordinationChanged
	}
	for _, part := range strings.Split(request.URL.Path[1:], "/") {
		if part == "" || part == "." || part == ".." {
			return nil, ErrCoordinationChanged
		}
	}
	var raw []byte
	if request.Body != nil {
		defer request.Body.Close()
		var err error
		raw, err = io.ReadAll(io.LimitReader(request.Body, 16385))
		if err != nil {
			return nil, ErrCoordinationChanged
		}
	}
	if len(raw) > 16384 || !json.Valid(raw) {
		return nil, ErrCoordinationChanged
	}
	payload, err := json.Marshal(struct {
		Path string `json:"path"`
		Body string `json:"body"`
	}{request.URL.Path, string(raw)})
	if err != nil {
		return nil, ErrCoordinationChanged
	}
	target := *c.endpoint
	target.Path = "/console/cleanup/internal/coordination/" + c.scope.Enterprise + "/" + c.scope.Region
	at := strconv.FormatInt(time.Now().Unix(), 10)
	sum := sha256.Sum256(payload)
	message := strings.Join([]string{"cleanup-coordination-v1", "POST", target.RequestURI(), c.scope.Enterprise, c.scope.Region, at, hex.EncodeToString(sum[:])}, "\n")
	mac := hmac.New(sha256.New, c.scope.Key)
	mac.Write([]byte(message))
	forwarded, err := http.NewRequestWithContext(request.Context(), "POST", target.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, ErrCoordinationChanged
	}
	forwarded.GetBody = nil
	forwarded.Header.Set("Content-Type", "application/json")
	forwarded.Header.Set("X-Cleanup-Coordination-Time", at)
	forwarded.Header.Set("X-Cleanup-Coordination-Signature", hex.EncodeToString(mac.Sum(nil)))
	return c.next.RoundTrip(forwarded)
}

// NewConsoleCoordinationClient uses the installation-scoped signing key. It
// never needs a Region administrator token or copies caller credentials.
func NewConsoleCoordinationClient(endpoint string, scope ConsoleCoordinationScope, allowHTTP bool, transport http.RoundTripper) (*CoordinationClient, error) {
	origin, err := url.Parse(endpoint)
	if err != nil || (origin.Scheme == "http" && !allowHTTP) {
		return nil, ErrCoordinationChanged
	}
	signed, err := newConsoleCoordinationTransport(endpoint, scope, transport)
	if err != nil {
		return nil, err
	}
	return &CoordinationClient{base: origin.Scheme + "://" + origin.Host, http: &http.Client{Transport: signed, Timeout: 4 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

// ValidIdentity checks scope identifiers without loading a signing credential.
func (c ConsoleCoordinationScope) ValidIdentity() bool {
	return consoleCoordinationScope.MatchString(c.Enterprise) && consoleCoordinationScope.MatchString(c.Region)
}

package sentry

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

const flushTimeout = 2 * time.Second

var sensitiveKeyRE = regexp.MustCompile(`(?i)(token|password|secret|authorization|cookie|key|dsn)`)
var sensitiveValueRE = regexp.MustCompile(`(?i)\b((?:token|password|secret|authorization|cookie|dsn|api[_-]?key|access[_-]?key|secret[_-]?key)\s*[:=]\s*)(?:bearer\s+)?[^&\s"'<>]+`)
var bearerValueRE = regexp.MustCompile(`(?i)\b(bearer\s+)[a-z0-9._~+/=-]+`)

var (
	activeClient *client
	mu           sync.RWMutex
	wg           sync.WaitGroup
)

type client struct {
	envelopeURL string
	authHeader  string
	publicDSN   string
	component   string
	config      Config
	httpClient  *http.Client
}

type exceptionValue struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type exceptionPayload struct {
	Values []exceptionValue `json:"values"`
}

type eventPayload struct {
	EventID     string                 `json:"event_id"`
	Timestamp   string                 `json:"timestamp"`
	Platform    string                 `json:"platform"`
	Level       string                 `json:"level"`
	Logger      string                 `json:"logger"`
	Environment string                 `json:"environment,omitempty"`
	Release     string                 `json:"release,omitempty"`
	Tags        map[string]string      `json:"tags,omitempty"`
	Exception   exceptionPayload       `json:"exception"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
}

// Init starts official error reporting when an ingest DSN is configured.
func Init(component string) bool {
	cfg := currentConfig()
	if !cfg.Enabled {
		return false
	}
	transport, err := newClient(cfg, component)
	if err != nil {
		return false
	}

	mu.Lock()
	activeClient = transport
	mu.Unlock()
	return true
}

// Flush sends pending events before process exit.
func Flush() {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(flushTimeout):
	}
}

// CaptureException reports a non-panic error with optional tags.
func CaptureException(err error, tags map[string]string) {
	if err == nil {
		return
	}
	c := currentClient()
	if c == nil {
		return
	}
	c.capture(sanitizeString(err.Error()), "error", tags, nil)
}

// CapturePanic reports recovered panic details with a stack trace.
func CapturePanic(rvr interface{}, stack []byte, tags map[string]string) {
	if rvr == nil {
		return
	}
	c := currentClient()
	if c == nil {
		return
	}
	if stack == nil {
		stack = debug.Stack()
	}
	c.capture(sanitizeString(fmt.Sprintf("panic: %v", rvr)), "panic", tags, map[string]interface{}{
		"stack": sanitizeString(string(stack)),
	})
}

// CaptureHTTPPanic reports an HTTP panic with sanitized request context.
func CaptureHTTPPanic(rvr interface{}, r *http.Request, stack []byte, tags map[string]string) {
	if rvr == nil {
		return
	}
	if tags == nil {
		tags = map[string]string{}
	}
	if r != nil {
		tags["http.method"] = r.Method
	}
	CapturePanic(rvr, stack, tags)
}

func newClient(cfg Config, component string) (*client, error) {
	parsed, err := url.Parse(cfg.DSN)
	if err != nil {
		return nil, err
	}
	if parsed.User == nil || parsed.User.Username() == "" {
		return nil, fmt.Errorf("sentry dsn missing public key")
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) == 0 || segments[len(segments)-1] == "" {
		return nil, fmt.Errorf("sentry dsn missing project id")
	}
	projectID := segments[len(segments)-1]
	basePath := strings.Join(segments[:len(segments)-1], "/")
	envelopePath := path.Join("/", basePath, "api", projectID, "envelope") + "/"

	envelopeURL := *parsed
	envelopeURL.User = nil
	envelopeURL.RawQuery = ""
	envelopeURL.Fragment = ""
	envelopeURL.Path = envelopePath
	publicDSN := *parsed
	publicDSN.User = url.User(parsed.User.Username())
	publicDSN.RawQuery = ""
	publicDSN.Fragment = ""

	authHeader := fmt.Sprintf(
		"Sentry sentry_version=7, sentry_client=rainbond-go/1.0, sentry_key=%s",
		parsed.User.Username(),
	)

	return &client{
		envelopeURL: envelopeURL.String(),
		authHeader:  authHeader,
		publicDSN:   publicDSN.String(),
		component:   component,
		config:      cfg,
		httpClient:  &http.Client{Timeout: 3 * time.Second},
	}, nil
}

func currentClient() *client {
	mu.RLock()
	defer mu.RUnlock()
	return activeClient
}

func (c *client) capture(message, exceptionType string, tags map[string]string, extra map[string]interface{}) {
	if tags == nil {
		tags = map[string]string{}
	}
	tags["component"] = valueOrDefault(tags["component"], c.component)
	event := eventPayload{
		EventID:     newEventID(),
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		Platform:    "go",
		Level:       "error",
		Logger:      "rainbond",
		Environment: c.config.Environment,
		Release:     c.config.Release,
		Tags:        tags,
		Exception: exceptionPayload{
			Values: []exceptionValue{{
				Type:  exceptionType,
				Value: message,
			}},
		},
		Extra: sanitizeExtra(extra),
	}
	c.send(event)
}

func (c *client) send(event eventPayload) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	envelopeHeader, err := json.Marshal(map[string]string{
		"event_id": event.EventID,
		"sent_at":  time.Now().UTC().Format(time.RFC3339Nano),
		"dsn":      c.publicDSN,
	})
	if err != nil {
		return
	}
	itemHeader := []byte(`{"type":"event","content_type":"application/json"}`)
	envelope := bytes.Join([][]byte{envelopeHeader, itemHeader, payload}, []byte("\n"))
	wg.Add(1)
	go func() {
		defer wg.Done()
		req, err := http.NewRequest("POST", c.envelopeURL, bytes.NewReader(envelope))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Sentry-Auth", c.authHeader)
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return
		}
		_ = resp.Body.Close()
	}()
}

func newEventID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strings.ReplaceAll(fmt.Sprintf("%d", time.Now().UnixNano()), "-", "")
	}
	return hex.EncodeToString(b[:])
}

func sanitizeExtra(extra map[string]interface{}) map[string]interface{} {
	if extra == nil {
		return nil
	}
	result := make(map[string]interface{}, len(extra))
	for key, value := range extra {
		if sensitiveKeyRE.MatchString(key) {
			result[key] = "[Filtered]"
			continue
		}
		if text, ok := value.(string); ok {
			result[key] = sanitizeString(text)
			continue
		}
		result[key] = value
	}
	return result
}

func sanitizeString(value string) string {
	value = sensitiveValueRE.ReplaceAllString(value, "${1}[Filtered]")
	return bearerValueRE.ReplaceAllString(value, "${1}[Filtered]")
}

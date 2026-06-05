package prober

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type probeResult string

const (
	runtimeProbeResultSuccess probeResult = "success"
	runtimeProbeResultWarning probeResult = "warning"
	runtimeProbeResultFailure probeResult = "failure"
	runtimeProbeResultUnknown probeResult = "unknown"
	maxProbeBodyLength                   = 10 * 1 << 10
)

type httpRuntimeProber interface {
	Probe(url *url.URL, headers http.Header, timeout time.Duration) (probeResult, string, error)
}

type tcpRuntimeProber interface {
	Probe(host string, port int, timeout time.Duration) (probeResult, string, error)
}

func newHTTPRuntimeProber(followNonLocalRedirects bool) httpRuntimeProber {
	transport := &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		DisableKeepAlives: true,
		Proxy:             nil,
	}
	return &defaultHTTPRuntimeProber{
		transport:               transport,
		followNonLocalRedirects: followNonLocalRedirects,
	}
}

func newTCPRuntimeProber() tcpRuntimeProber {
	return &defaultTCPRuntimeProber{}
}

type defaultHTTPRuntimeProber struct {
	transport               *http.Transport
	followNonLocalRedirects bool
}

func (p *defaultHTTPRuntimeProber) Probe(target *url.URL, headers http.Header, timeout time.Duration) (probeResult, string, error) {
	client := &http.Client{
		Timeout:       timeout,
		Transport:     p.transport,
		CheckRedirect: redirectChecker(p.followNonLocalRedirects),
	}
	req, err := http.NewRequest("GET", target.String(), nil)
	if err != nil {
		return runtimeProbeResultFailure, err.Error(), nil
	}
	if headers == nil {
		headers = http.Header{}
	}
	if _, ok := headers["Accept"]; !ok {
		headers.Set("Accept", "*/*")
	} else if headers.Get("Accept") == "" {
		headers.Del("Accept")
	}
	req.Header = headers
	req.Host = headers.Get("Host")

	res, err := client.Do(req)
	if err != nil {
		return runtimeProbeResultFailure, err.Error(), nil
	}
	defer res.Body.Close()

	bodyBuffer := make([]byte, maxProbeBodyLength)
	n, readErr := res.Body.Read(bodyBuffer)
	if readErr != nil && !errors.Is(readErr, net.ErrClosed) && readErr.Error() != "EOF" {
		return runtimeProbeResultFailure, "", readErr
	}
	body := string(bodyBuffer[:n])

	if res.StatusCode >= http.StatusOK && res.StatusCode < http.StatusBadRequest {
		if res.StatusCode >= http.StatusMultipleChoices {
			return runtimeProbeResultWarning, fmt.Sprintf("Probe terminated redirects, Response body: %v", body), nil
		}
		return runtimeProbeResultSuccess, body, nil
	}
	return runtimeProbeResultFailure, fmt.Sprintf("HTTP probe failed with statuscode: %d", res.StatusCode), nil
}

type defaultTCPRuntimeProber struct{}

func (p *defaultTCPRuntimeProber) Probe(host string, port int, timeout time.Duration) (probeResult, string, error) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), timeout)
	if err != nil {
		return runtimeProbeResultFailure, err.Error(), nil
	}
	if err := conn.Close(); err != nil {
		return runtimeProbeResultFailure, err.Error(), nil
	}
	return runtimeProbeResultSuccess, "", nil
}

func redirectChecker(followNonLocalRedirects bool) func(*http.Request, []*http.Request) error {
	if followNonLocalRedirects {
		return nil
	}
	return func(req *http.Request, via []*http.Request) error {
		if req.URL.Hostname() != via[0].URL.Hostname() {
			return http.ErrUseLastResponse
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
}

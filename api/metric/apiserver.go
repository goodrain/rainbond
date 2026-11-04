package metric

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	promclient "github.com/goodrain/rainbond/api/client/prometheus"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const nativeAPIServerMetricsQuery = `count(apiserver_request_total{rainbond_source!="rbd-api"})`

var apiServerRequestDimensions = []string{"code", "component", "dry_run", "group", "resource", "scope", "subresource", "verb", "version"}

var apiServerRequestsDesc = prometheus.NewDesc("apiserver_request_total", "Kubernetes API server requests.",
	append(append([]string{"apiserver"}, apiServerRequestDimensions...), "rainbond_source", "rainbond_api_instance"), nil)
var apiServerCollectionDesc = prometheus.NewDesc("rbd_api_exporter_apiserver_collection_success",
	"Whether API server request counters are available from an existing source or compatibility collection.", nil, nil)

type apiServerContextMonitor interface {
	GetMetricWithContext(context.Context, string, time.Time) promclient.Metric
}

// APIServerCollector fills missing request counters through the existing rbdapi scrape job.
// It does not change monitoring configuration or require an Operator upgrade.
type APIServerCollector struct {
	mu      sync.Mutex
	fetch   func(context.Context) ([]prometheus.Metric, error)
	now     func() time.Time
	timeout time.Duration
	expires time.Time
	metrics []prometheus.Metric
	success float64
}

// NewAPIServerCollector reuses the Kubernetes identity already used by rbd-api.
func NewAPIServerCollector(kube kubernetes.Interface, config *rest.Config, monitor promclient.Interface) (*APIServerCollector, error) {
	if kube == nil || config == nil || monitor == nil {
		return nil, errors.New("API server metric dependencies unavailable")
	}
	reader, ok := monitor.(apiServerContextMonitor)
	if !ok {
		return nil, errors.New("context-aware monitoring client required")
	}
	host, err := url.Parse(config.Host)
	if err != nil || host.Scheme != "https" || host.Hostname() == "" || host.User != nil || config.Insecure {
		return nil, errors.New("verified Kubernetes HTTPS configuration required")
	}
	copyConfig := rest.CopyConfig(config)
	if copyConfig.ServerName == "" {
		copyConfig.ServerName = host.Hostname()
	}
	transport, err := rest.TransportFor(copyConfig)
	if err != nil {
		return nil, errors.New("Kubernetes metrics transport unavailable")
	}
	instance, err := os.Hostname()
	if err != nil || instance == "" {
		return nil, errors.New("rbd-api replica identity unavailable")
	}
	source := &apiServerMetricSource{
		kube: kube, monitor: reader, maxBytes: 32 << 20, instance: instance,
		client: &http.Client{Transport: transport, Timeout: 3 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
	}
	return &APIServerCollector{fetch: source.collect, now: time.Now, timeout: 3 * time.Second}, nil
}

// Describe only declares metrics; registering the collector never calls Kubernetes.
func (c *APIServerCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- apiServerRequestsDesc
	ch <- apiServerCollectionDesc
}

// Collect publishes real upstream counters, or only an unavailable status on failure.
func (c *APIServerCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	if !c.now().Before(c.expires) {
		ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
		metrics, err := c.fetch(ctx)
		cancel()
		c.metrics, c.success = metrics, 1
		if err != nil {
			c.metrics, c.success = nil, 0
			// Upstream errors and response bodies may contain sensitive connection details.
			logrus.Debug("API server request-counter collection unavailable")
		}
		c.expires = c.now().Add(15 * time.Second)
	}
	metrics, success := c.metrics, c.success
	c.mu.Unlock()
	ch <- prometheus.MustNewConstMetric(apiServerCollectionDesc, prometheus.GaugeValue, success)
	for _, metric := range metrics {
		ch <- metric
	}
}

type apiServerMetricSource struct {
	kube     kubernetes.Interface
	monitor  apiServerContextMonitor
	client   *http.Client
	maxBytes int64
	instance string
}

func (s *apiServerMetricSource) collect(ctx context.Context) ([]prometheus.Metric, error) {
	native := s.monitor.GetMetricWithContext(ctx, nativeAPIServerMetricsQuery, time.Now())
	if native.Error != "" || native.MetricType != promclient.MetricTypeVector || len(native.MetricValues) > 1 {
		return nil, errors.New("cannot determine existing API server metric availability")
	}
	if len(native.MetricValues) == 1 {
		point := native.MetricValues[0].Sample
		if point == nil || math.IsNaN(point.Value()) || math.IsInf(point.Value(), 0) || point.Value() < 0 {
			return nil, errors.New("invalid existing API server metric availability")
		}
		if point.Value() > 0 {
			return nil, nil
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	endpoints, err := s.kube.CoreV1().Endpoints("default").Get(ctx, "kubernetes", metav1.GetOptions{})
	if err != nil {
		return nil, errors.New("API server endpoint discovery unavailable")
	}
	unique := make(map[string]struct{})
	for _, subset := range endpoints.Subsets {
		for _, port := range subset.Ports {
			if port.Name != "https" || (port.Protocol != "" && port.Protocol != "TCP") {
				continue
			}
			if port.Port <= 0 || port.Port > 65535 {
				return nil, errors.New("invalid API server HTTPS port")
			}
			for _, address := range subset.Addresses {
				if net.ParseIP(address.IP) == nil {
					return nil, errors.New("invalid API server endpoint address")
				}
				unique[net.JoinHostPort(address.IP, strconv.Itoa(int(port.Port)))] = struct{}{}
			}
		}
	}
	if len(unique) == 0 || len(unique) > 16 {
		return nil, errors.New("API server endpoint set unavailable")
	}
	targets := make([]string, 0, len(unique))
	for target := range unique {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	type result struct {
		metrics []prometheus.Metric
		err     error
	}
	results := make(chan result, len(targets))
	workers := make(chan struct{}, 4)
	for _, target := range targets {
		go func(target string) {
			select {
			case workers <- struct{}{}:
				defer func() { <-workers }()
			case <-ctx.Done():
				results <- result{err: ctx.Err()}
				return
			}
			metrics, err := s.read(ctx, target)
			results <- result{metrics, err}
		}(target)
	}
	var metrics []prometheus.Metric
	for range targets {
		result := <-results
		if result.err != nil {
			return nil, result.err
		}
		metrics = append(metrics, result.metrics...)
	}
	return metrics, nil
}

func (s *apiServerMetricSource) read(ctx context.Context, target string) ([]prometheus.Metric, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+target+"/metrics", nil)
	if err != nil {
		return nil, errors.New("invalid API server metrics request")
	}
	req.Header.Set("Accept", "text/plain; version=0.0.4")
	response, err := s.client.Do(req)
	if err != nil {
		return nil, errors.New("API server metrics request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API server metrics returned HTTP %d", response.StatusCode)
	}
	reader := &io.LimitedReader{R: response.Body, N: s.maxBytes + 1}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	var counters bytes.Buffer
	for scanner.Scan() {
		line := scanner.Bytes()
		if bytes.HasPrefix(line, []byte("# TYPE apiserver_request_total ")) ||
			bytes.HasPrefix(line, []byte("apiserver_request_total{")) ||
			bytes.HasPrefix(line, []byte("apiserver_request_total ")) {
			if counters.Len()+len(line) > 4<<20 {
				return nil, errors.New("API server request-counter response too large")
			}
			counters.Write(line)
			counters.WriteByte('\n')
		}
	}
	if scanner.Err() != nil || reader.N == 0 {
		return nil, errors.New("API server metrics response incomplete")
	}
	var parser expfmt.TextParser
	families, err := parser.TextToMetricFamilies(&counters)
	if err != nil {
		return nil, errors.New("invalid API server request counters")
	}
	family := families["apiserver_request_total"]
	if family == nil || family.GetType() != dto.MetricType_COUNTER || len(family.Metric) == 0 {
		return nil, errors.New("API server request counters unavailable")
	}
	totals := make(map[[9]string]float64)
	for _, metric := range family.Metric {
		if metric.Counter == nil {
			return nil, errors.New("invalid API server counter type")
		}
		value := metric.Counter.GetValue()
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, errors.New("invalid API server counter value")
		}
		labels := make(map[string]string)
		for _, label := range metric.Label {
			labels[label.GetName()] = label.GetValue()
		}
		if labels["code"] == "" || labels["verb"] == "" {
			return nil, errors.New("API server request dimensions unavailable")
		}
		var key [9]string
		for i, name := range apiServerRequestDimensions {
			key[i] = labels[name]
		}
		totals[key] += value
		if math.IsInf(totals[key], 0) {
			return nil, errors.New("API server request counter overflow")
		}
	}
	metrics := make([]prometheus.Metric, 0, len(totals))
	for key, value := range totals {
		labels := append(append([]string{target}, key[:]...), "rbd-api", s.instance)
		metrics = append(metrics, prometheus.MustNewConstMetric(apiServerRequestsDesc, prometheus.CounterValue, value, labels...))
	}
	return metrics, nil
}

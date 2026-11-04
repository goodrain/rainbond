// capability_id: rainbond.api.apiserver-request-metrics
package metric

import (
	"context"
	"encoding/pem"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	promclient "github.com/goodrain/rainbond/api/client/prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

type apiServerMonitorStub struct {
	promclient.Interface
	metric promclient.Metric
	query  string
	calls  int
}

func (s *apiServerMonitorStub) GetMetric(query string, _ time.Time) promclient.Metric {
	s.query = query
	s.calls++
	return s.metric
}

func (s *apiServerMonitorStub) GetMetricWithContext(_ context.Context, query string, ts time.Time) promclient.Metric {
	return s.GetMetric(query, ts)
}

func missingNativeAPIServerMetrics() *apiServerMonitorStub {
	return &apiServerMonitorStub{metric: promclient.Metric{MetricData: promclient.MetricData{MetricType: promclient.MetricTypeVector}}}
}

func apiServerCounterBody(success, failure float64) string {
	return fmt.Sprintf("# TYPE apiserver_request_total counter\napiserver_request_total{code=\"200\",verb=\"GET\",resource=\"pods\"} %g\napiserver_request_total{code=\"500\",verb=\"GET\",resource=\"pods\"} %g\n# TYPE unrelated_counter counter\nunrelated_counter 42\n", success, failure)
}

func newAPIServerFixture(t *testing.T, handlers ...http.HandlerFunc) (*APIServerCollector, *fake.Clientset, *apiServerMonitorStub) {
	t.Helper()
	endpoints := &corev1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: "kubernetes", Namespace: "default"}}
	var config *rest.Config
	for _, handler := range handlers {
		server := httptest.NewTLSServer(handler)
		t.Cleanup(server.Close)
		parsed, err := url.Parse(server.URL)
		if err != nil {
			t.Fatal(err)
		}
		if config == nil {
			config = &rest.Config{Host: server.URL, TLSClientConfig: rest.TLSClientConfig{
				CAData: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}),
			}}
		}
		port, err := strconv.Atoi(parsed.Port())
		if err != nil {
			t.Fatal(err)
		}
		endpoints.Subsets = append(endpoints.Subsets, corev1.EndpointSubset{
			Addresses: []corev1.EndpointAddress{{IP: parsed.Hostname()}},
			Ports:     []corev1.EndpointPort{{Name: "https", Port: int32(port), Protocol: corev1.ProtocolTCP}},
		})
	}
	if config == nil {
		config = &rest.Config{Host: "https://kubernetes.default.svc"}
	}
	kube := fake.NewSimpleClientset(endpoints)
	monitor := missingNativeAPIServerMetrics()
	collector, err := NewAPIServerCollector(kube, config, monitor)
	if err != nil {
		t.Fatal(err)
	}
	return collector, kube, monitor
}

func gatherAPIServerMetrics(t *testing.T, collector *APIServerCollector) map[string]*dto.MetricFamily {
	t.Helper()
	registry := prometheus.NewRegistry()
	if err := registry.Register(collector); err != nil {
		t.Fatal(err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]*dto.MetricFamily)
	for _, family := range families {
		result[family.GetName()] = family
	}
	return result
}

func TestAPIServerCountersUseExistingScrapeWithoutChangingConfiguration(t *testing.T) {
	collector, kube, monitor := newAPIServerFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metrics" || r.Method != http.MethodGet {
			t.Errorf("unexpected upstream request: %s %s", r.Method, r.URL.Path)
		}
		io.WriteString(w, apiServerCounterBody(18, 2))
	})
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)
	if len(kube.Actions()) != 0 || monitor.calls != 0 {
		t.Fatal("metric registration must not query live dependencies")
	}
	original := prometheus.NewGauge(prometheus.GaugeOpts{Name: "existing_rbd_api_metric", Help: "Existing metric."})
	original.Set(7)
	registry.MustRegister(original)
	server := httptest.NewServer(promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	defer server.Close()
	response, err := http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var parser expfmt.TextParser
	families, err := parser.TextToMetricFamilies(response.Body)
	if err != nil || response.StatusCode != 200 {
		t.Fatalf("existing metrics scrape failed: status=%d error=%v", response.StatusCode, err)
	}
	if families["existing_rbd_api_metric"].Metric[0].Gauge.GetValue() != 7 {
		t.Fatal("existing API metrics must remain available")
	}
	requests := families["apiserver_request_total"]
	if requests == nil || requests.GetType() != dto.MetricType_COUNTER || len(requests.Metric) != 2 {
		t.Fatalf("missing real upstream request counters: %+v", requests)
	}
	var successful, total float64
	for _, metric := range requests.Metric {
		labels := map[string]string{}
		for _, label := range metric.Label {
			labels[label.GetName()] = label.GetValue()
		}
		if labels["rainbond_source"] != "rbd-api" || labels["apiserver"] == "" || labels["resource"] != "pods" || labels["verb"] != "GET" {
			t.Fatalf("upstream identity or dimensions lost: %+v", labels)
		}
		total += metric.Counter.GetValue()
		if labels["code"] == "200" {
			successful += metric.Counter.GetValue()
		}
	}
	if successful != 18 || total != 20 || families["unrelated_counter"] != nil {
		t.Fatal("the old dashboard must receive only the original request counters, not a fabricated percentage")
	}
	if monitor.query != nativeAPIServerMetricsQuery || !strings.Contains(monitor.query, `rainbond_source!="rbd-api"`) {
		t.Fatal("native-source detection must exclude the compatibility collector itself")
	}
	for _, action := range kube.Actions() {
		if action.GetVerb() != "get" || action.GetResource().Resource != "endpoints" || action.GetNamespace() != "default" {
			t.Fatalf("upgrading rbd-api must not edit Operator or user monitoring configuration: %+v", action)
		}
	}
}

func TestAPIServerNativeCollectionIsPreserved(t *testing.T) {
	collector, kube, monitor := newAPIServerFixture(t)
	point := promclient.Point{1, 12}
	monitor.metric.MetricValues = []promclient.MetricValue{{Sample: &point}}
	families := gatherAPIServerMetrics(t, collector)
	if families["apiserver_request_total"] != nil || len(kube.Actions()) != 0 {
		t.Fatal("an existing native collector must be retained without duplicate counters or new API calls")
	}
	if families["rbd_api_exporter_apiserver_collection_success"].Metric[0].Gauge.GetValue() != 1 {
		t.Fatal("native metric availability must be reported")
	}
}

func TestAPIServerFailureLeavesOriginalMetricsScrapable(t *testing.T) {
	collector, _, _ := newAPIServerFixture(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream response must not be exported", http.StatusForbidden)
	})
	original := prometheus.NewGauge(prometheus.GaugeOpts{Name: "existing_rbd_api_metric", Help: "Existing metric."})
	original.Set(7)
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector, original)
	server := httptest.NewServer(promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	defer server.Close()
	response, err := http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if response.StatusCode != 200 || !strings.Contains(text, "existing_rbd_api_metric 7") ||
		!strings.Contains(text, "rbd_api_exporter_apiserver_collection_success 0") ||
		strings.Contains(text, "apiserver_request_total") || strings.Contains(text, "upstream response") {
		t.Fatal("a failed compatibility source must not break existing scrapes, expose response bodies or fabricate request counters")
	}
}

func TestAPIServerReplicasKeepIndependentCountersAndIgnoreNonReadyTargets(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, apiServerCounterBody(9, 1)) }
	collector, kube, _ := newAPIServerFixture(t, handler, handler)
	endpoint, _ := kube.CoreV1().Endpoints("default").Get(context.Background(), "kubernetes", metav1.GetOptions{})
	endpoint.Subsets = append(endpoint.Subsets, corev1.EndpointSubset{
		NotReadyAddresses: []corev1.EndpointAddress{{IP: "192.0.2.1"}},
		Ports:             []corev1.EndpointPort{{Name: "https", Port: 443}},
	}, corev1.EndpointSubset{Addresses: []corev1.EndpointAddress{{IP: "192.0.2.2"}}, Ports: []corev1.EndpointPort{{Name: "http", Port: 80}}})
	endpoint.Subsets = append(endpoint.Subsets, endpoint.Subsets[0])
	if _, err := kube.CoreV1().Endpoints("default").Update(context.Background(), endpoint, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	families := gatherAPIServerMetrics(t, collector)
	requests := families["apiserver_request_total"]
	if requests == nil || len(requests.Metric) != 4 {
		t.Fatalf("each unique ready HTTPS endpoint must retain its counters: %+v", requests)
	}
	targets := make(map[string]bool)
	for _, metric := range requests.Metric {
		for _, label := range metric.Label {
			if label.GetName() == "apiserver" {
				targets[label.GetValue()] = true
			}
		}
	}
	if len(targets) != 2 {
		t.Fatal("API server replicas must not be collapsed behind a rotating load balancer")
	}
}

func TestAPIServerFailuresDoNotPublishZeroOrPartialRequestCounters(t *testing.T) {
	cases := []struct {
		name string
		body string
		code int
	}{
		{"forbidden", "private upstream response", 403},
		{"unavailable", "unavailable", 503},
		{"missing", "other_metric 1\n", 200},
		{"malformed", "# TYPE apiserver_request_total counter\napiserver_request_total{broken\n", 200},
		{"wrong_type", "# TYPE apiserver_request_total gauge\napiserver_request_total{code=\"200\",verb=\"GET\"} 1\n", 200},
		{"negative", apiServerCounterBody(-1, 0), 200},
		{"nan", apiServerCounterBody(math.NaN(), 0), 200},
		{"infinite", apiServerCounterBody(math.Inf(1), 0), 200},
		{"dimensions_missing", "# TYPE apiserver_request_total counter\napiserver_request_total 1\n", 200},
		{"overflow", "# TYPE apiserver_request_total counter\napiserver_request_total{code=\"200\",verb=\"GET\",extra=\"a\"} 1e308\napiserver_request_total{code=\"200\",verb=\"GET\",extra=\"b\"} 1e308\n", 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			collector, _, _ := newAPIServerFixture(t,
				func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, apiServerCounterBody(9, 1)) },
				func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.code); io.WriteString(w, tc.body) })
			families := gatherAPIServerMetrics(t, collector)
			if families["apiserver_request_total"] != nil || families["rbd_api_exporter_apiserver_collection_success"].Metric[0].Gauge.GetValue() != 0 {
				t.Fatal("failed or incomplete collection must remain unavailable, never a healthy zero or partial success rate")
			}
		})
	}
}

func TestAPIServerCacheCoalescesConcurrentScrapesAndPreservesCounterReset(t *testing.T) {
	var requests atomic.Int64
	value := atomic.Int64{}
	value.Store(100)
	collector, _, monitor := newAPIServerFixture(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		io.WriteString(w, apiServerCounterBody(float64(value.Load()), 0))
	})
	now := time.Now()
	collector.now = func() time.Time { return now }
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := registry.Gather(); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if requests.Load() != 1 || monitor.calls != 1 {
		t.Fatal("concurrent scrapes must reuse a bounded cache")
	}
	value.Store(2)
	now = now.Add(16 * time.Second)
	families := gatherAPIServerMetrics(t, collector)
	for _, metric := range families["apiserver_request_total"].Metric {
		for _, label := range metric.Label {
			if label.GetName() == "code" && label.GetValue() == "200" && metric.Counter.GetValue() != 2 {
				t.Fatal("forward the real restarted Counter rather than accumulating proxy increments")
			}
		}
	}
}

func TestAPIServerDeadlineAndRedirectsAreBounded(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		collector, _, _ := newAPIServerFixture(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
		collector.timeout = 75 * time.Millisecond
		start := time.Now()
		families := gatherAPIServerMetrics(t, collector)
		if time.Since(start) > time.Second || families["apiserver_request_total"] != nil {
			t.Fatal("API server collection must time out without blocking original metric availability")
		}
	})
	t.Run("redirect", func(t *testing.T) {
		var followed atomic.Int64
		other := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			followed.Add(1)
			io.WriteString(w, apiServerCounterBody(9, 1))
		}))
		defer other.Close()
		collector, _, _ := newAPIServerFixture(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, other.URL, 302) })
		families := gatherAPIServerMetrics(t, collector)
		if followed.Load() != 0 || families["apiserver_request_total"] != nil {
			t.Fatal("Kubernetes credentials must not follow a metrics redirect")
		}
	})
}

func TestAPIServerMonitorFailureDoesNotGuessNativeConfiguration(t *testing.T) {
	for _, metric := range []promclient.Metric{{Error: "unavailable"}, {}, {MetricData: promclient.MetricData{MetricType: promclient.MetricTypeVector, MetricValues: []promclient.MetricValue{{}}}}} {
		collector, kube, monitor := newAPIServerFixture(t)
		monitor.metric = metric
		families := gatherAPIServerMetrics(t, collector)
		if len(kube.Actions()) != 0 || families["apiserver_request_total"] != nil {
			t.Fatal("unknown native-source state must not enable duplicate collection")
		}
	}
}

func TestAPIServerTransportRequiresVerifiedHTTPS(t *testing.T) {
	kube := fake.NewSimpleClientset()
	monitor := missingNativeAPIServerMetrics()
	for _, cfg := range []*rest.Config{nil, {Host: "http://kubernetes.default.svc"}, {Host: "https://kubernetes.default.svc", TLSClientConfig: rest.TLSClientConfig{Insecure: true}}, {Host: "https://$K8S_USERNAME:$K8S_PASSWORD@kubernetes.default.svc"}, {Host: "https://kubernetes.default.svc", TLSClientConfig: rest.TLSClientConfig{CAData: []byte("invalid CA")}}} {
		if _, err := NewAPIServerCollector(kube, cfg, monitor); err == nil {
			t.Fatal("do not introduce insecure transport or credential-bearing target URLs")
		}
	}
}

func TestAPIServerResponseSizeIsBounded(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, apiServerCounterBody(9, 1))
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL)
	source := &apiServerMetricSource{client: server.Client(), maxBytes: 32}
	if metrics, err := source.read(context.Background(), target.Host); err == nil || len(metrics) != 0 {
		t.Fatal("an incomplete oversized response must not be accepted as valid metrics")
	}
}

func TestAPIServerProxyReplicasHaveDistinctCounterSeries(t *testing.T) {
	var value atomic.Int64
	value.Store(100)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, apiServerCounterBody(float64(value.Load()), 0))
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL)
	first := &apiServerMetricSource{client: server.Client(), maxBytes: 1 << 20, instance: "api-a"}
	second := &apiServerMetricSource{client: server.Client(), maxBytes: 1 << 20, instance: "api-b"}
	metricsA, err := first.read(context.Background(), target.Host)
	if err != nil {
		t.Fatal(err)
	}
	value.Store(90)
	metricsB, err := second.read(context.Background(), target.Host)
	if err != nil {
		t.Fatal(err)
	}
	collector := &APIServerCollector{now: time.Now, timeout: time.Second, fetch: func(context.Context) ([]prometheus.Metric, error) {
		return append(metricsA, metricsB...), nil
	}}
	families := gatherAPIServerMetrics(t, collector)
	if len(families["apiserver_request_total"].Metric) != 4 {
		t.Fatal("different cached samples from API replicas must not become a single resetting Counter")
	}
}

func TestAPIServerMissingDiscoveryRemainsUnavailable(t *testing.T) {
	collector, _, _ := newAPIServerFixture(t)
	families := gatherAPIServerMetrics(t, collector)
	if families["apiserver_request_total"] != nil || families["rbd_api_exporter_apiserver_collection_success"].Metric[0].Gauge.GetValue() != 0 {
		t.Fatal("missing ready endpoints must not be reported as zero request traffic")
	}
}

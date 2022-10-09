package monitor

import (
	"github.com/prometheus/client_golang/prometheus"
)

var healthStatus float64

// Metric name parts.
const (
	// Namespace for all metrics.
	namespace = "node"
	// Subsystem(s).
	exporter = "exporter"
)

//Exporter collects builder metrics. It implements prometheus.Collector.
type Exporter struct {
	test prometheus.Counter
}

var healthDesc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, exporter, "health_status"),
	"node service health status.",
	[]string{"service_name"}, nil,
)

//NewExporter new a exporter
func NewExporter() *Exporter {
	return &Exporter{
		test: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "test",
			Subsystem: "test",
			Name:      "test",
			Help:      "get and counter",
		}),
	}
}

//Describe implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	e.test.Describe(ch)
	metricCh := make(chan prometheus.Metric)
	doneCh := make(chan struct{})
	go func() {
		for m := range metricCh {
			ch <- m.Desc()
		}
		close(doneCh)
	}()

	e.Collect(metricCh)
	close(metricCh)
	<-doneCh
}

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.scrape(ch)
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	e.test.Add(1)
	e.test.Collect(ch)
	ch <- prometheus.MustNewConstMetric(healthDesc, prometheus.GaugeValue, healthStatus, "node")
}

func ChangeHealthStatus(isStatus float64) {
	healthStatus = isStatus
}

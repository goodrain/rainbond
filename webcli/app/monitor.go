package app

import (
	"github.com/prometheus/client_golang/prometheus"

)

// Metric name parts.
const (
	// Namespace for all metrics.
	namespace = "webcli"
	// Subsystem(s).
	exporter = "exporter"
)

//Exporter collects entrance metrics. It implements prometheus.Collector.
type Exporter struct {
	ExecuteCommandTotal prometheus.Counter
	ExecuteCommandFailed prometheus.Counter
}
var healthDesc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, exporter, "health_status"),
	"health status.",
	[]string{"service_name"}, nil,
)

//NewExporter new a exporter
func NewExporter() *Exporter {
	return &Exporter{
		ExecuteCommandTotal:prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "execute_command_total",
			Help:      "Total number of execution commands",
		}),
		ExecuteCommandFailed:prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "execute_command_failed",
			Help:      "failed number of execution commands",
		}),
	}
}

//Describe implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
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

	ch <- prometheus.MustNewConstMetric(healthDesc, prometheus.GaugeValue, 1, "webcli")
	ch <- prometheus.MustNewConstMetric(e.ExecuteCommandTotal.Desc(), prometheus.CounterValue, ExecuteCommandTotal)
	ch <- prometheus.MustNewConstMetric(e.ExecuteCommandFailed.Desc(), prometheus.CounterValue, ExecuteCommandFailed)
}

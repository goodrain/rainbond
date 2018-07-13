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
	healthStatus prometheus.Gauge
	ExecuteCommandTotal prometheus.Gauge
	ExecuteCommandFailed prometheus.Gauge
}

//NewExporter new a exporter
func NewExporter() *Exporter {
	return &Exporter{
		healthStatus: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "webcli_health_status",
			Help:      "webcli component health status.",
		}),
		ExecuteCommandTotal:prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "execute_command_total",
			Help:      "Total number of execution commands",
		}),
		ExecuteCommandFailed:prometheus.NewGauge(prometheus.GaugeOpts{
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

	ch <- prometheus.MustNewConstMetric(e.healthStatus.Desc(), prometheus.GaugeValue, 1)
	ch <- prometheus.MustNewConstMetric(e.ExecuteCommandTotal.Desc(), prometheus.GaugeValue, ExecuteCommandTotal)
	ch <- prometheus.MustNewConstMetric(e.ExecuteCommandFailed.Desc(), prometheus.GaugeValue, ExecuteCommandFailed)
}

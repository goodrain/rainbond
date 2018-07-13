package monitor

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/goodrain/rainbond/builder/discover"
	"github.com/goodrain/rainbond/builder/exector"
)

// Metric name parts.
const (
	// Namespace for all metrics.
	namespace = "builder"
	// Subsystem(s).
	exporter = "exporter"
)

//Exporter collects entrance metrics. It implements prometheus.Collector.
type Exporter struct {
	healthStatus prometheus.Gauge
	taskNum prometheus.Gauge
	taskError prometheus.Gauge
}

//NewExporter new a exporter
func NewExporter() *Exporter {
	return &Exporter{
		healthStatus: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "builder_health_status",
			Help:      "builder component health status.",
		}),
		taskNum:prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "builder_task_number",
			Help:      "builder number of tasks",
		}),
		taskError:prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "builder_task_error",
			Help:      "builder number of task errors",
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

	healthInfo := discover.HealthCheck()
	healthStatus := healthInfo["status"]
	var val float64
	if healthStatus == "health" {
		val = 1
	} else {
		val = 0
	}

	ch <- prometheus.MustNewConstMetric(e.healthStatus.Desc(), prometheus.GaugeValue, val)
	ch <- prometheus.MustNewConstMetric(e.taskNum.Desc(), prometheus.GaugeValue, exector.TaskNum)
	ch <- prometheus.MustNewConstMetric(e.taskError.Desc(), prometheus.GaugeValue, exector.ErrorNum)
}

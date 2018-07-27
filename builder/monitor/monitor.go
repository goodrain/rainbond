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
	taskNum prometheus.Counter
	taskError prometheus.Counter
}

var healthDesc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, exporter, "health_status"),
	"builder service health status.",
	[]string{"service_name"}, nil,
)

//NewExporter new a exporter
func NewExporter() *Exporter {
	return &Exporter{
		taskNum:prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "builder_task_number",
			Help:      "builder number of tasks",
		}),
		taskError:prometheus.NewCounter(prometheus.CounterOpts{
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

	ch <- prometheus.MustNewConstMetric(healthDesc, prometheus.GaugeValue, val, "builder")
	ch <- prometheus.MustNewConstMetric(e.taskNum.Desc(), prometheus.CounterValue, exector.TaskNum)
	ch <- prometheus.MustNewConstMetric(e.taskError.Desc(), prometheus.CounterValue, exector.ErrorNum)
}

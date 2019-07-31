package monitor

import (
	"github.com/goodrain/rainbond/builder/discover"
	"github.com/goodrain/rainbond/builder/exector"
	"github.com/prometheus/client_golang/prometheus"
)

// Metric name parts.
const (
	// Namespace for all metrics.
	namespace = "builder"
	// Subsystem(s).
	exporter = "exporter"
)

//Exporter collects builder metrics. It implements prometheus.Collector.
type Exporter struct {
	taskNum                     prometheus.Counter
	taskError                   prometheus.Counter
	taskBackMetric              prometheus.Counter
	maxConcurrentTaskMetric     prometheus.Counter
	currentConcurrentTaskMetric prometheus.Counter
	exec                        exector.Manager
}

var healthDesc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, exporter, "health_status"),
	"builder service health status.",
	[]string{"service_name"}, nil,
)

//NewExporter new a exporter
func NewExporter(exec exector.Manager) *Exporter {
	return &Exporter{
		exec: exec,
		taskNum: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "builder_task_number",
			Help:      "builder number of tasks",
		}),
		taskError: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "builder_task_error",
			Help:      "builder number of task errors",
		}),
		taskBackMetric: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "builder_back_task_num",
			Help:      "builder number of task by callback",
		}),
		maxConcurrentTaskMetric: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "builder_max_concurrent_task",
			Help:      "Maximum number of concurrent execution tasks supported",
		}),
		currentConcurrentTaskMetric: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "builder_current_concurrent_task",
			Help:      "Number of tasks currently being performed",
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
	ch <- prometheus.MustNewConstMetric(e.taskNum.Desc(), prometheus.CounterValue, exector.MetricTaskNum)
	ch <- prometheus.MustNewConstMetric(e.taskError.Desc(), prometheus.CounterValue, exector.MetricErrorTaskNum)
	ch <- prometheus.MustNewConstMetric(e.taskBackMetric.Desc(), prometheus.CounterValue, exector.MetricBackTaskNum)
	ch <- prometheus.MustNewConstMetric(e.maxConcurrentTaskMetric.Desc(), prometheus.GaugeValue, e.exec.GetMaxConcurrentTask())
	ch <- prometheus.MustNewConstMetric(e.currentConcurrentTaskMetric.Desc(), prometheus.GaugeValue, e.exec.GetCurrentConcurrentTask())
}

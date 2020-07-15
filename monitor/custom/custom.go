package custom

import (
	"time"

	"github.com/goodrain/rainbond/monitor/prometheus"
	"github.com/prometheus/common/model"
)

// Metrics metrics struct
type Metrics struct {
	Name     string
	Metrics  []string
	Interval time.Duration
	Timeout  time.Duration
	Scheme   string
	Path     string
}

// AddMetrics add mysql metrics into prometheus
func AddMetrics(p *prometheus.Manager, metrics Metrics) {
	p.UpdateScrape(&prometheus.ScrapeConfig{
		JobName:        metrics.Name,
		ScrapeInterval: model.Duration(metrics.Interval),
		ScrapeTimeout:  model.Duration(metrics.Timeout),
		MetricsPath:    metrics.Path,
		Scheme:         metrics.Scheme,
		ServiceDiscoveryConfig: prometheus.ServiceDiscoveryConfig{
			StaticConfigs: []*prometheus.Group{
				{
					Targets: metrics.Metrics,
					Labels: map[model.LabelName]model.LabelValue{
						"component":    model.LabelValue(metrics.Name),
						"service_name": model.LabelValue(metrics.Name),
					},
				},
			},
		},
	})
}

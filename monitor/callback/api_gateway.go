package callback

import (
	"github.com/goodrain/rainbond/discover"
	"github.com/goodrain/rainbond/discover/config"
	"github.com/goodrain/rainbond/monitor/prometheus"
	"github.com/goodrain/rainbond/monitor/utils"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"time"
)

type APIGateway struct {
	discover.Callback
	Prometheus      *prometheus.Manager
	sortedEndpoints []string

	endpoints []*config.Endpoint
}

func (in *APIGateway) Error(err error) {
	logrus.Error(err)
}

func (in *APIGateway) Name() string {
	return "api_gateway"
}

func (in *APIGateway) UpdateEndpoints(endpoints ...*config.Endpoint) {
	newArr := utils.TrimAndSort(endpoints)

	if utils.ArrCompare(in.sortedEndpoints, newArr) {
		logrus.Debugf("The endpoints is not modify: %s", in.Name())
		return
	}

	in.sortedEndpoints = newArr

	scrape := in.toScrape()
	in.Prometheus.UpdateScrape(scrape)
}

func (e *APIGateway) toScrape() *prometheus.ScrapeConfig {
	ts := make([]string, 0, len(e.sortedEndpoints))
	for _, end := range e.sortedEndpoints {
		ts = append(ts, end)
	}

	return &prometheus.ScrapeConfig{
		JobName:        e.Name(),
		ScrapeInterval: model.Duration(15 * time.Second),
		ScrapeTimeout:  model.Duration(15 * time.Second),
		MetricsPath:    "/apisix/prometheus/metrics",
		HonorLabels:    true,
		ServiceDiscoveryConfig: prometheus.ServiceDiscoveryConfig{
			KubernetesSDConfigs: []*prometheus.SDConfig{
				{
					Role: prometheus.RoleEndpoint,
					NamespaceDiscovery: prometheus.NamespaceDiscovery{
						Names: []string{"ingress-apisix"},
					},
					Selectors: []prometheus.SelectorConfig{
						{
							Role:  prometheus.RoleEndpoint,
							Field: "metadata.name=rbd-api-api-inner",
						},
					},
				},
			},
		},
	}
}

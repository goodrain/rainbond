package prom

import (
	"context"
	"github.com/goodrain/rainbond/api/client/prometheus"
	"github.com/goodrain/rainbond/config/configs"
)

var defaultPromComponent *Component

// Component -
type Component struct {
	PrometheusCli prometheus.Interface
}

// Prometheus -
func Prometheus() *Component {
	defaultPromComponent = &Component{}
	return defaultPromComponent
}

// Start -
func (c *Component) Start(ctx context.Context, cfg *configs.Config) error {
	prometheusCli, err := prometheus.NewPrometheus(&prometheus.Options{
		Endpoint: cfg.APIConfig.PrometheusEndpoint,
	})
	c.PrometheusCli = prometheusCli
	return err
}

// CloseHandle -
func (c *Component) CloseHandle() {
}

// Default -
func Default() *Component {
	return defaultPromComponent
}

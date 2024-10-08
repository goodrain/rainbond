package configs

import "github.com/spf13/pflag"

type PrometheusConfig struct {
	PrometheusMetricPath string
}

func AddPrometheusFlags(fs *pflag.FlagSet, pc *PrometheusConfig) {
	fs.StringVar(&pc.PrometheusMetricPath, "metric", "/metrics", "prometheus metrics path")
}

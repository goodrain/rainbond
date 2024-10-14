package configs

import "github.com/spf13/pflag"

type ServerConfig struct {
	RbdWorker          string
	EventLogServers    []string
	PrometheusEndpoint string
	RbdHub             string
	MQAPI              string
}

func AddServerFlags(fs *pflag.FlagSet, sc *ServerConfig) {
	fs.StringSliceVar(&sc.EventLogServers, "event-servers", []string{"rbd-api-api-inner:6366"}, "event log server address")
	fs.StringVar(&sc.RbdWorker, "worker-api", "rbd-worker:6535", "the rbd-worker server api")
	fs.StringVar(&sc.PrometheusEndpoint, "prom-api", "rbd-monitor:9999", "The service DNS name of Prometheus api. Default to rbd-monitor:9999")
	fs.StringVar(&sc.RbdHub, "hub-api", "http://rbd-hub:5000", "the rbd-hub server api")
	fs.StringVar(&sc.MQAPI, "mq-api", "rbd-mq:6300", "the rbd-mq server api")
}

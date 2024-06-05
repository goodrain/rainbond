package metrics

import (
	"context"
	"github.com/goodrain/rainbond/mq/monitor"
	"github.com/goodrain/rainbond/mq/mqcomponent/mqclient"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/pkg/gogo"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/version"
	"github.com/sirupsen/logrus"
	"net/http"
)

// Server 要做一些监控或者指标收集
type Server struct {
}

// Start -
func (s *Server) Start(ctx context.Context, cfg *configs.Config) error {
	return s.StartCancel(ctx, nil, cfg)
}

// CloseHandle -
func (s *Server) CloseHandle() {
}

// New -
func New() *Server {
	return &Server{}
}

// StartCancel -
func (s *Server) StartCancel(ctx context.Context, cancel context.CancelFunc, cfg *configs.Config) error {
	prometheus.MustRegister(version.NewCollector("acp_mq"))
	prometheus.MustRegister(monitor.NewExporter(mqclient.Default().ActionMQ()))
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		httputil.ReturnSuccess(r, w, map[string]string{"status": "health", "info": "mq service health"})
	})
	return gogo.Go(func(ctx context.Context) error {
		logrus.Infof("start metrics server")
		defer cancel()
		if err := http.ListenAndServe(":6301", nil); err != nil {
			logrus.Error("mq pprof listen error.", err.Error())
			return err
		}
		return nil
	})
}

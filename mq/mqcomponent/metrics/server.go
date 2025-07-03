package metrics

import (
	"context"
	"fmt"
	"time"
	"github.com/goodrain/rainbond/mq/monitor"
	"github.com/goodrain/rainbond/mq/mqcomponent/mqclient"
	"github.com/prometheus/client_golang/prometheus/promhttp"

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
func (s *Server) Start(ctx context.Context) error {
	return s.StartCancel(ctx, nil)
}

// CloseHandle -
func (s *Server) CloseHandle() {
}

// New -
func New() *Server {
	return &Server{}
}

// StartCancel -
func (s *Server) StartCancel(ctx context.Context, cancel context.CancelFunc) error {
	prometheus.MustRegister(version.NewCollector("acp_mq"))
	prometheus.MustRegister(monitor.NewExporter(mqclient.Default().ActionMQ()))
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		
		// Check if MQ service is available
		mq := mqclient.Default().ActionMQ()
		if mq == nil {
			httputil.ReturnError(r, w, 500, "MQ service unavailable")
			return
		}
		
		// Test MQ functionality with a health check message
		testTopic := "health-check"
		testMessage := fmt.Sprintf("health-test-%d", time.Now().Unix())
		
		// Test enqueue operation
		if err := mq.Enqueue(ctx, testTopic, testMessage); err != nil {
			logrus.Errorf("MQ health check enqueue failed: %v", err)
			httputil.ReturnError(r, w, 500, fmt.Sprintf("MQ enqueue failed: %v", err))
			return
		}
		
		// Test dequeue operation  
		if _, err := mq.Dequeue(ctx, testTopic); err != nil {
			logrus.Errorf("MQ health check dequeue failed: %v", err)
			httputil.ReturnError(r, w, 500, fmt.Sprintf("MQ dequeue failed: %v", err))
			return
		}
		
		// Check queue status for main topics
		builderQueueSize := mq.MessageQueueSize("builder")
		if builderQueueSize < 0 {
			httputil.ReturnError(r, w, 500, "MQ queue status abnormal")
			return
		}
		
		// Return detailed health status
		httputil.ReturnSuccess(r, w, map[string]interface{}{
			"status": "healthy",
			"timestamp": time.Now().Unix(),
			"topics": mq.GetAllTopics(),
			"builder_queue_size": builderQueueSize,
		})
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

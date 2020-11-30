// RAINBOND, Application Management Platform
// Copyright (C) 2014-2019 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

//collectors
//Special instructions: this package Learn from the nginx-ingress-controller

package collectors

import (
	"io"
	"io/ioutil"
	"net"
	"os"

	jsoniter "github.com/json-iterator/go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

type upstream struct {
	Latency        float64 `json:"upstreamLatency"`
	ResponseLength float64 `json:"upstreamResponseLength"`
	ResponseTime   float64 `json:"upstreamResponseTime"`
	//Status         string  `json:"upstreamStatus"`
}

type socketData struct {
	upstream
	Host           string  `json:"host"`
	Status         string  `json:"status"`
	ResponseLength float64 `json:"responseLength"`
	Method         string  `json:"method"`
	RequestLength  float64 `json:"requestLength"`
	RequestTime    float64 `json:"requestTime"`
	Namespace      string  `json:"namespace"`
	ServiceID      string  `json:"service_id"`
	Path           string  `json:"path"`
}

// SocketCollector stores prometheus metrics and ingress meta-data
type SocketCollector struct {
	prometheus.Collector
	requestTime     *prometheus.HistogramVec
	requestLength   *prometheus.HistogramVec
	responseTime    *prometheus.HistogramVec
	responseLength  *prometheus.HistogramVec
	upstreamLatency *prometheus.SummaryVec
	bytesSent       *prometheus.HistogramVec
	requests        *prometheus.CounterVec
	listener        net.Listener
	metricMapping   map[string]interface{}
	hosts           sets.String
	metricsPerHost  bool
}

var (
	requestTags = []string{
		"status",
		"method",
		"path",
		"namespace",
		"service",
		"service_id",
	}
)

// NewSocketCollector creates a new SocketCollector instance using
// the ingress watch namespace and class used by the controller
func NewSocketCollector(gatewayHost string, metricsPerHost bool) (*SocketCollector, error) {
	socket := "/tmp/prometheus-nginx.socket"
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return nil, err
	}

	err = os.Chmod(socket, 0777)
	if err != nil {
		return nil, err
	}

	constLabels := prometheus.Labels{
		"gateway": gatewayHost,
	}

	requestTags := requestTags
	if metricsPerHost {
		requestTags = append(requestTags, "host")
	}

	sc := &SocketCollector{
		listener:       listener,
		metricsPerHost: metricsPerHost,
		responseTime: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "response_duration_seconds",
				Help:        "The time spent on receiving the response from the upstream server",
				Namespace:   PrometheusNamespace,
				ConstLabels: constLabels,
			},
			requestTags,
		),
		responseLength: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "response_size",
				Help:        "The response length (including request line, header, and request body)",
				Namespace:   PrometheusNamespace,
				ConstLabels: constLabels,
			},
			requestTags,
		),

		requestTime: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "request_duration_seconds",
				Help:        "The request processing time in milliseconds",
				Namespace:   PrometheusNamespace,
				ConstLabels: constLabels,
			},
			requestTags,
		),
		requestLength: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "request_size",
				Help:        "The request length (including request line, header, and request body)",
				Namespace:   PrometheusNamespace,
				Buckets:     prometheus.LinearBuckets(10, 10, 10), // 10 buckets, each 10 bytes wide.
				ConstLabels: constLabels,
			},
			requestTags,
		),

		requests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "requests",
				Help:        "The total number of client requests.",
				Namespace:   PrometheusNamespace,
				ConstLabels: constLabels,
			},
			[]string{"host", "namespace", "service", "status", "service_id"},
		),

		bytesSent: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "bytes_sent",
				Help:        "The number of bytes sent to a client",
				Namespace:   PrometheusNamespace,
				Buckets:     prometheus.ExponentialBuckets(10, 10, 7), // 7 buckets, exponential factor of 10.
				ConstLabels: constLabels,
			},
			requestTags,
		),

		upstreamLatency: prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:        "upstream_latency_seconds",
				Help:        "Upstream service latency per Ingress",
				Namespace:   PrometheusNamespace,
				ConstLabels: constLabels,
			},
			[]string{"namespace", "service", "service_id"},
		),
	}

	sc.metricMapping = map[string]interface{}{
		prometheus.BuildFQName(PrometheusNamespace, "", "request_duration_seconds"): sc.requestTime,
		prometheus.BuildFQName(PrometheusNamespace, "", "request_size"):             sc.requestLength,

		prometheus.BuildFQName(PrometheusNamespace, "", "response_duration_seconds"): sc.responseTime,
		prometheus.BuildFQName(PrometheusNamespace, "", "response_size"):             sc.responseLength,

		prometheus.BuildFQName(PrometheusNamespace, "", "bytes_sent"): sc.bytesSent,

		prometheus.BuildFQName(PrometheusNamespace, "", "upstream_latency_seconds"): sc.upstreamLatency,
	}

	return sc, nil
}

// SetHosts sets the hostnames that are being served by the ingress controller
// This set of hostnames is used to filter the metrics to be exposed
func (sc *SocketCollector) SetHosts(hosts sets.String) {
	sc.hosts = hosts
}

func (sc *SocketCollector) handleMessage(msg []byte) {
	// Unmarshal bytes
	var statsBatch []socketData
	err := jsoniter.ConfigCompatibleWithStandardLibrary.Unmarshal(msg, &statsBatch)
	if err != nil {
		logrus.Errorf("Unexpected error deserializing JSON payload: %v. Payload:\n%v", err, string(msg))
		return
	}
	for _, stats := range statsBatch {
		if !sc.hosts.HasAny(stats.Host, "tls"+stats.Host) {
			logrus.Debugf("skiping metric for host %v that is not being served", stats.Host)
			continue
		}
		// Note these must match the order in requestTags at the top
		requestLabels := prometheus.Labels{
			"status":     stats.Status,
			"method":     stats.Method,
			"path":       stats.Path,
			"namespace":  stats.Namespace,
			"service":    stats.ServiceID,
			"service_id": stats.ServiceID,
		}
		if sc.metricsPerHost {
			requestLabels["host"] = stats.Host
		}
		collectorLabels := prometheus.Labels{
			"namespace":  stats.Namespace,
			"service":    stats.ServiceID,
			"service_id": stats.ServiceID,
			"status":     stats.Status,
			"host":       stats.Host,
		}
		latencyLabels := prometheus.Labels{
			"namespace":  stats.Namespace,
			"service":    stats.ServiceID,
			"service_id": stats.ServiceID,
		}
		requestsMetric, err := sc.requests.GetMetricWith(collectorLabels)
		if err != nil {
			logrus.Errorf("Error fetching requests metric: %v", err)
		} else {
			requestsMetric.Inc()
		}
		if stats.Latency != -1 {
			latencyMetric, err := sc.upstreamLatency.GetMetricWith(latencyLabels)
			if err != nil {
				logrus.Errorf("Error fetching latency metric: %v", err)
			} else {
				latencyMetric.Observe(stats.Latency)
			}
		}
		if stats.RequestTime != -1 {
			requestTimeMetric, err := sc.requestTime.GetMetricWith(requestLabels)
			if err != nil {
				logrus.Errorf("Error fetching request duration metric: %v", err)
			} else {
				requestTimeMetric.Observe(stats.RequestTime)
			}
		}
		if stats.RequestLength != -1 {
			requestLengthMetric, err := sc.requestLength.GetMetricWith(requestLabels)
			if err != nil {
				logrus.Errorf("Error fetching request length metric: %v", err)
			} else {
				requestLengthMetric.Observe(stats.RequestLength)
			}
		}
		if stats.ResponseTime != -1 {
			responseTimeMetric, err := sc.responseTime.GetMetricWith(requestLabels)
			if err != nil {
				logrus.Errorf("Error fetching upstream response time metric: %v", err)
			} else {
				responseTimeMetric.Observe(stats.ResponseTime)
			}
		}
		if stats.ResponseLength != -1 {
			bytesSentMetric, err := sc.bytesSent.GetMetricWith(requestLabels)
			if err != nil {
				logrus.Errorf("Error fetching bytes sent metric: %v", err)
			} else {
				bytesSentMetric.Observe(stats.ResponseLength)
			}
			responseSizeMetric, err := sc.responseLength.GetMetricWith(requestLabels)
			if err != nil {
				logrus.Errorf("Error fetching bytes sent metric: %v", err)
			} else {
				responseSizeMetric.Observe(stats.ResponseLength)
			}
		}
	}
}

// Start listen for connections in the unix socket and spawns a goroutine to process the content
func (sc *SocketCollector) Start() {
	for {
		conn, err := sc.listener.Accept()
		if err != nil {
			continue
		}
		go handleMessages(conn, sc.handleMessage)
	}
}

// Stop stops unix listener
func (sc *SocketCollector) Stop() {
	sc.listener.Close()
}

// RemoveMetrics deletes prometheus metrics from prometheus for hosts and
// host that are not available anymore.
// Ref: https://godoc.org/github.com/prometheus/client_golang/prometheus#CounterVec.Delete
func (sc *SocketCollector) RemoveMetrics(hosts []string, registry prometheus.Gatherer) {
	mfs, err := registry.Gather()
	if err != nil {
		logrus.Errorf("Error gathering metrics: %v", err)
		return
	}
	// 1. remove metrics of removed hosts
	logrus.Debugf("removing host %v from metrics", hosts)
	for _, mf := range mfs {
		metricName := mf.GetName()
		metric, ok := sc.metricMapping[metricName]
		if !ok {
			continue
		}
		toRemove := sets.NewString(hosts...)
		for _, m := range mf.GetMetric() {
			labels := make(map[string]string, len(m.GetLabel()))
			for _, labelPair := range m.GetLabel() {
				labels[*labelPair.Name] = *labelPair.Value
			}
			// remove labels that are constant
			deleteConstants(labels)
			ingKey, ok := labels["host"]
			if !toRemove.Has(ingKey) {
				continue
			}
			logrus.Infof("Removing prometheus metric from histogram %v for host %v", metricName, ingKey)
			h, ok := metric.(*prometheus.HistogramVec)
			if ok {
				removed := h.Delete(labels)
				if !removed {
					logrus.Debugf("metric %v for host %v with labels not removed: %v", metricName, ingKey, labels)
				}
			}
			s, ok := metric.(*prometheus.SummaryVec)
			if ok {
				removed := s.Delete(labels)
				if !removed {
					logrus.Debugf("metric %v for host %v with labels not removed: %v", metricName, ingKey, labels)
				}
			}
		}
	}
}

// Describe implements prometheus.Collector
func (sc SocketCollector) Describe(ch chan<- *prometheus.Desc) {
	sc.requestTime.Describe(ch)
	sc.requestLength.Describe(ch)
	sc.requests.Describe(ch)
	sc.upstreamLatency.Describe(ch)
	sc.responseTime.Describe(ch)
	sc.responseLength.Describe(ch)
	sc.bytesSent.Describe(ch)
}

// Collect implements the prometheus.Collector interface.
func (sc SocketCollector) Collect(ch chan<- prometheus.Metric) {
	sc.requestTime.Collect(ch)
	sc.requestLength.Collect(ch)
	sc.requests.Collect(ch)
	sc.upstreamLatency.Collect(ch)
	sc.responseTime.Collect(ch)
	sc.responseLength.Collect(ch)
	sc.bytesSent.Collect(ch)
}

// handleMessages process the content received in a network connection
func handleMessages(conn io.ReadCloser, fn func([]byte)) {
	defer conn.Close()
	data, err := ioutil.ReadAll(conn)
	if err != nil {
		return
	}
	fn(data)
}

func deleteConstants(labels prometheus.Labels) {
	delete(labels, "controller_namespace")
	delete(labels, "controller_class")
	delete(labels, "controller_pod")
}

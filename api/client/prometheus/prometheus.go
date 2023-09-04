// RAINBOND, Application Management Platform
// Copyright (C) 2020-2020 Goodrain Co., Ltd.

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

package prometheus

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/api"
	apiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

//Options prometheus options
type Options struct {
	Endpoint string `json:"endpoint,omitempty" yaml:"endpoint"`
}

// prometheus implements monitoring interface backed by Prometheus
type prometheus struct {
	client apiv1.API
}

//NewPrometheus new prometheus monitor
func NewPrometheus(options *Options) (Interface, error) {
	if options.Endpoint == "" {
		options.Endpoint = "http://rbd-monitor:9999"
	} else if !strings.HasPrefix(options.Endpoint, "http") {
		options.Endpoint = fmt.Sprintf("http://%s", options.Endpoint)
	}
	cfg := api.Config{
		Address: options.Endpoint,
		RoundTripper: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
	client, err := api.NewClient(cfg)
	return prometheus{client: apiv1.NewAPI(client)}, err
}

func (p prometheus) GetMetric(expr string, ts time.Time) Metric {
	var parsedResp Metric

	value, _, err := p.client.Query(context.Background(), expr, ts)
	if err != nil {
		parsedResp.Error = err.Error()
	} else {
		parsedResp.MetricData = parseQueryResp(value)
	}

	return parsedResp
}

func (p prometheus) GetMetricOverTime(expr string, start, end time.Time, step time.Duration) Metric {
	timeRange := apiv1.Range{
		Start: start,
		End:   end,
		Step:  step,
	}

	value, _, err := p.client.QueryRange(context.Background(), expr, timeRange)

	var parsedResp Metric
	if err != nil {
		parsedResp.Error = err.Error()
	} else {
		parsedResp.MetricData = parseQueryRangeResp(value)
	}
	return parsedResp
}

func (p prometheus) GetMetadata(namespace string) []Metadata {
	var meta []Metadata

	// Filter metrics available to members of this namespace
	matchTarget := fmt.Sprintf("{namespace=\"%s\"}", namespace)
	fmt.Println(matchTarget)
	items, err := p.client.TargetsMetadata(context.Background(), matchTarget, "", "")
	if err != nil {
		logrus.Error(err)
		return meta
	}

	// Deduplication
	set := make(map[string]bool)
	for _, item := range items {
		_, ok := set[item.Metric]
		if !ok {
			set[item.Metric] = true
			meta = append(meta, Metadata{
				Metric: item.Metric,
				Type:   string(item.Type),
				Help:   item.Help,
			})
		}
	}

	return meta
}

func (p prometheus) GetAppMetadata(namespace, appID string) []Metadata {
	var meta []Metadata

	// Filter metrics available to members of this namespace
	matchTarget := fmt.Sprintf("{namespace=\"%s\",app_id=\"%s\"}", namespace, appID)
	items, err := p.client.TargetsMetadata(context.Background(), matchTarget, "", "")
	if err != nil {
		logrus.Error(err)
		return meta
	}

	// Deduplication
	set := make(map[string]bool)
	for _, item := range items {
		_, ok := set[item.Metric]
		if !ok {
			set[item.Metric] = true
			meta = append(meta, Metadata{
				Metric: item.Metric,
				Type:   string(item.Type),
				Help:   item.Help,
			})
		}
	}
	return meta
}

func (p prometheus) GetComponentMetadata(namespace, componentID string) []Metadata {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var meta []Metadata

	// Filter metrics available to members of this namespace
	matchTarget := fmt.Sprintf("{namespace=\"%s\",service_id=\"%s\"}", namespace, componentID)
	items, err := p.client.TargetsMetadata(ctx, matchTarget, "", "")
	if err != nil {
		logrus.Error(err)
		return meta
	}
	// Deduplication
	set := make(map[string]bool)
	for _, item := range items {
		_, ok := set[item.Metric]
		if !ok {
			set[item.Metric] = true
			meta = append(meta, Metadata{
				Metric: item.Metric,
				Type:   string(item.Type),
				Help:   item.Help,
			})
		}
	}

	commonItems, err := p.client.TargetsMetadata(ctx, "{job=~\"gateway|cadvisor\"}", "", "")
	if err != nil {
		logrus.Error(err)
		return meta
	}
	for _, item := range commonItems {
		if !strings.HasPrefix(item.Metric, "container") && !strings.HasPrefix(item.Metric, "gateway") {
			continue
		}
		_, ok := set[item.Metric]
		if !ok {
			set[item.Metric] = true
			meta = append(meta, Metadata{
				Metric: item.Metric,
				Type:   string(item.Type),
				Help:   item.Help,
			})
		}
	}

	return meta
}

func (p prometheus) GetMetricLabelSet(expr string, start, end time.Time) []map[string]string {
	var res []map[string]string

	labelSet, _, err := p.client.Series(context.Background(), []string{expr}, start, end)
	if err != nil {
		logrus.Error(err)
		return []map[string]string{}
	}

	for _, item := range labelSet {
		var tmp = map[string]string{}
		for key, val := range item {
			if key == "__name__" {
				continue
			}
			tmp[string(key)] = string(val)
		}

		res = append(res, tmp)
	}

	return res
}

func parseQueryRangeResp(value model.Value) MetricData {
	res := MetricData{MetricType: MetricTypeMatrix}

	data, _ := value.(model.Matrix)

	for _, v := range data {
		mv := MetricValue{
			Metadata: make(map[string]string),
		}

		for k, v := range v.Metric {
			mv.Metadata[string(k)] = string(v)
		}

		for _, k := range v.Values {
			mv.Series = append(mv.Series, Point{float64(k.Timestamp) / 1000, float64(k.Value)})
		}

		res.MetricValues = append(res.MetricValues, mv)
	}

	return res
}

func parseQueryResp(value model.Value) MetricData {
	res := MetricData{MetricType: MetricTypeVector}

	data, _ := value.(model.Vector)

	for _, v := range data {
		mv := MetricValue{
			Metadata: make(map[string]string),
		}

		for k, v := range v.Metric {
			mv.Metadata[string(k)] = string(v)
		}

		mv.Sample = &Point{float64(v.Timestamp) / 1000, float64(v.Value)}

		res.MetricValues = append(res.MetricValues, mv)
	}

	return res
}

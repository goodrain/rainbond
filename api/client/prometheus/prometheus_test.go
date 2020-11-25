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
	"testing"
	"time"
)

var cli Interface

func init() {
	cli, _ = NewPrometheus(&Options{
		Endpoint: "9999.grc42f14.8wsfp0ji.a24839.grapps.cn",
	})
}
func TestGetMetric(t *testing.T) {
	metric := cli.GetMetric("up{job=\"rbdapi\"}", time.Now())
	if len(metric.MetricData.MetricValues) == 0 {
		t.Fatal("not found metric")
	}
	t.Log(metric.MetricData.MetricValues[0].Sample.Value())
}

func TestGetMetricOverTime(t *testing.T) {
	metric := cli.GetMetricOverTime("up{job=\"rbdapi\"}", time.Now().Add(-time.Second*60), time.Now(), time.Second*10)
	if len(metric.MetricData.MetricValues) == 0 {
		t.Fatal("not found metric")
	}
	if len(metric.MetricData.MetricValues[0].Series) < 6 {
		t.Fatalf("metric series length %d is less than 6", len(metric.MetricData.MetricValues[0].Series))
	}
	t.Log(metric.MetricData.MetricValues[0].Series)
}

func TestGetMetadata(t *testing.T) {
	metas := cli.GetMetadata("rbd-system")
	if len(metas) == 0 {
		t.Fatal("meta length is 0")
	}
	for _, meta := range metas {
		t.Log(meta.Metric)
	}
}

func TestGetAppMetadata(t *testing.T) {
	metas := cli.GetAppMetadata("rbd-system", "482")
	if len(metas) == 0 {
		t.Fatal("meta length is 0")
	}
	for _, meta := range metas {
		t.Log(meta.Metric)
	}
}

func TestGetComponentMetadata(t *testing.T) {
	metas := cli.GetComponentMetadata("3be96e95700a480c9b37c6ef5daf3566", "d89ffc075ca74476b6040c8e8bae9756")
	if len(metas) == 0 {
		t.Fatal("meta length is 0")
	}
	for _, meta := range metas {
		t.Log(meta.Metric)
	}
}

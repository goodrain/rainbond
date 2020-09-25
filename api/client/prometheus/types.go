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
	"errors"
	"fmt"
	"strconv"

	jsoniter "github.com/json-iterator/go"
)

const (
	MetricTypeMatrix = "matrix"
	MetricTypeVector = "vector"
)

type Metadata struct {
	Metric string `json:"metric,omitempty" description:"metric name"`
	Type   string `json:"type,omitempty" description:"metric type"`
	Help   string `json:"help,omitempty" description:"metric description"`
}

type Metric struct {
	MetricName string `json:"metric_name,omitempty" description:"metric name, eg. scheduler_up_sum"`
	MetricData `json:"data,omitempty" description:"actual metric result"`
	Error      string `json:"error,omitempty"`
}

type MetricData struct {
	MetricType   string        `json:"resultType,omitempty" description:"result type, one of matrix, vector"`
	MetricValues []MetricValue `json:"result,omitempty" description:"metric data including labels, time series and values"`
}

// The first element is the timestamp, the second is the metric value.
// eg, [1585658599.195, 0.528]
type Point [2]float64

type MetricValue struct {
	Metadata map[string]string `json:"metric,omitempty" description:"time series labels"`
	// The type of Point is a float64 array with fixed length of 2.
	// So Point will always be initialized as [0, 0], rather than nil.
	// To allow empty Sample, we should declare Sample to type *Point
	Sample *Point  `json:"value,omitempty" description:"time series, values of vector type"`
	Series []Point `json:"values,omitempty" description:"time series, values of matrix type"`
}

func (p Point) Timestamp() float64 {
	return p[0]
}

func (p Point) Value() float64 {
	return p[1]
}

// MarshalJSON implements json.Marshaler. It will be called when writing JSON to HTTP response
// Inspired by prometheus/client_golang
func (p Point) MarshalJSON() ([]byte, error) {
	t, err := jsoniter.Marshal(p.Timestamp())
	if err != nil {
		return nil, err
	}
	v, err := jsoniter.Marshal(strconv.FormatFloat(p.Value(), 'f', -1, 64))
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf("[%s,%s]", t, v)), nil
}

// UnmarshalJSON implements json.Unmarshaler. This is for unmarshaling test data.
func (p *Point) UnmarshalJSON(b []byte) error {
	var v []interface{}
	if err := jsoniter.Unmarshal(b, &v); err != nil {
		return err
	}

	if v == nil {
		return nil
	}

	if len(v) != 2 {
		return errors.New("unsupported array length")
	}

	ts, ok := v[0].(float64)
	if !ok {
		return errors.New("failed to unmarshal [timestamp]")
	}
	valstr, ok := v[1].(string)
	if !ok {
		return errors.New("failed to unmarshal [value]")
	}
	valf, err := strconv.ParseFloat(valstr, 64)
	if err != nil {
		return err
	}

	p[0] = ts
	p[1] = valf
	return nil
}

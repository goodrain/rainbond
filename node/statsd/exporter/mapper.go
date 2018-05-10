// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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

package exporter

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	yaml "gopkg.in/yaml.v2"
)

var (
	identifierRE   = `[a-zA-Z_][a-zA-Z0-9_]+`
	statsdMetricRE = `[a-zA-Z_](-?[a-zA-Z0-9_])+`

	metricLineRE = regexp.MustCompile(`^(\*\.|` + statsdMetricRE + `\.)+(\*|` + statsdMetricRE + `)$`)
	labelLineRE  = regexp.MustCompile(`^(` + identifierRE + `)\s*=\s*"(.*)"$`)
	metricNameRE = regexp.MustCompile(`^` + identifierRE + `$`)
)

type mapperConfigDefaults struct {
	TimerType timerType `yaml:"timer_type"`
	Buckets   []float64 `yaml:"buckets"`
	MatchType matchType `yaml:"match_type"`
}

//MetricMapper MetricMapper
type MetricMapper struct {
	Defaults mapperConfigDefaults `yaml:"defaults"`
	Mappings []metricMapping      `yaml:"mappings"`
	mutex    sync.Mutex
}

type metricMapping struct {
	Match     string `yaml:"match"`
	Name      string `yaml:"name"`
	regex     *regexp.Regexp
	Labels    prometheus.Labels `yaml:"labels"`
	TimerType timerType         `yaml:"timer_type"`
	Buckets   []float64         `yaml:"buckets"`
	MatchType matchType         `yaml:"match_type"`
	HelpText  string            `yaml:"help"`
}

//InitFromYAMLString InitFromYAMLString
func (m *MetricMapper) InitFromYAMLString(fileContents string) error {
	var n MetricMapper

	if err := yaml.Unmarshal([]byte(fileContents), &n); err != nil {
		return err
	}

	if n.Defaults.Buckets == nil || len(n.Defaults.Buckets) == 0 {
		n.Defaults.Buckets = prometheus.DefBuckets
	}

	if n.Defaults.MatchType == matchTypeDefault {
		n.Defaults.MatchType = matchTypeGlob
	}

	for i := range n.Mappings {
		currentMapping := &n.Mappings[i]

		// check that label is correct
		for k := range currentMapping.Labels {
			if !metricNameRE.MatchString(k) {
				return fmt.Errorf("invalid label key: %s", k)
			}
		}

		if currentMapping.Name == "" {
			return fmt.Errorf("line %d: metric mapping didn't set a metric name", i)
		}

		if !metricNameRE.MatchString(currentMapping.Name) {
			return fmt.Errorf("metric name '%s' doesn't match regex '%s'", currentMapping.Name, metricNameRE)
		}

		if currentMapping.MatchType == "" {
			currentMapping.MatchType = n.Defaults.MatchType
		}

		if currentMapping.MatchType == matchTypeGlob {
			if !metricLineRE.MatchString(currentMapping.Match) {
				return fmt.Errorf("invalid match: %s", currentMapping.Match)
			}
			// Translate the glob-style metric match line into a proper regex that we
			// can use to match metrics later on.
			metricRe := strings.Replace(currentMapping.Match, ".", "\\.", -1)
			metricRe = strings.Replace(metricRe, "*", "([^.]*)", -1)
			currentMapping.regex = regexp.MustCompile("^" + metricRe + "$")
		} else {
			currentMapping.regex = regexp.MustCompile(currentMapping.Match)
		}

		if currentMapping.TimerType == "" {
			currentMapping.TimerType = n.Defaults.TimerType
		}

		if currentMapping.Buckets == nil || len(currentMapping.Buckets) == 0 {
			currentMapping.Buckets = n.Defaults.Buckets
		}

	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.Defaults = n.Defaults
	m.Mappings = n.Mappings

	mappingsCount.Set(float64(len(n.Mappings)))

	return nil
}

//InitFromFile InitFromFile
func (m *MetricMapper) InitFromFile(fileName string) error {
	mappingStr, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}
	return m.InitFromYAMLString(string(mappingStr))
}

func (m *MetricMapper) getMapping(statsdMetric string) (*metricMapping, prometheus.Labels, bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for _, mapping := range m.Mappings {
		matches := mapping.regex.FindStringSubmatchIndex(statsdMetric)
		if len(matches) == 0 {
			continue
		}

		labels := prometheus.Labels{}
		for label, valueExpr := range mapping.Labels {
			value := mapping.regex.ExpandString([]byte{}, valueExpr, statsdMetric, matches)
			labels[label] = string(value)
		}
		return &mapping, labels, true
	}

	return nil, nil, false
}

//InitMapping init mapping config
func InitMapping() (*MetricMapper, error) {
	var n MetricMapper
	m1 := metricMapping{
		Match:  "*.*.*.request.*",
		Name:   "app_request",
		Labels: prometheus.Labels{"service_id": "$1", "port": "$2", "protocol": "$3", "method": "$4"},
	}
	m2 := metricMapping{
		Match:  "*.*.*.request.unusual.*",
		Name:   "app_request_unusual",
		Labels: prometheus.Labels{"service_id": "$1", "port": "$2", "protocol": "$3", "code": "$4"},
	}
	m3 := metricMapping{
		Match:  "*.*.*.requesttime.*",
		Name:   "app_requesttime",
		Labels: prometheus.Labels{"service_id": "$1", "port": "$2", "protocol": "$3", "mode": "$4"},
	}
	m4 := metricMapping{
		Match:  "*.*.*.requestclient",
		Name:   "app_requestclient",
		Labels: prometheus.Labels{"service_id": "$1", "port": "$2", "protocol": "$3"},
	}
	m5 := metricMapping{
		Match:  "*.*.*.client-request.*",
		Name:   "app_client_request",
		Labels: prometheus.Labels{"service_id": "$1", "port": "$2", "protocol": "$3", "client": "$4"},
	}
	m6 := metricMapping{
		Match:  "*.*.*.client-requesttime.*",
		Name:   "app_client_requesttime",
		Labels: prometheus.Labels{"service_id": "$1", "port": "$2", "protocol": "$3", "client": "$4"},
	}
	n.Mappings = append(n.Mappings, m1, m2, m3, m4, m5, m6)
	if n.Defaults.Buckets == nil || len(n.Defaults.Buckets) == 0 {
		n.Defaults.Buckets = prometheus.DefBuckets
	}

	if n.Defaults.MatchType == matchTypeDefault {
		n.Defaults.MatchType = matchTypeGlob
	}
	for i := range n.Mappings {
		currentMapping := &n.Mappings[i]

		// check that label is correct
		for k := range currentMapping.Labels {
			if !metricNameRE.MatchString(k) {
				return nil, fmt.Errorf("invalid label key: %s", k)
			}
		}

		if currentMapping.Name == "" {
			return nil, fmt.Errorf("line %d: metric mapping didn't set a metric name", i)
		}

		if !metricNameRE.MatchString(currentMapping.Name) {
			return nil, fmt.Errorf("metric name '%s' doesn't match regex '%s'", currentMapping.Name, metricNameRE)
		}

		if currentMapping.MatchType == "" {
			currentMapping.MatchType = n.Defaults.MatchType
		}

		if currentMapping.MatchType == matchTypeGlob {
			if !metricLineRE.MatchString(currentMapping.Match) {
				return nil, fmt.Errorf("invalid match: %s", currentMapping.Match)
			}
			// Translate the glob-style metric match line into a proper regex that we
			// can use to match metrics later on.
			metricRe := strings.Replace(currentMapping.Match, ".", "\\.", -1)
			metricRe = strings.Replace(metricRe, "*", "([^.]*)", -1)
			currentMapping.regex = regexp.MustCompile("^" + metricRe + "$")
		} else {
			currentMapping.regex = regexp.MustCompile(currentMapping.Match)
		}

		if currentMapping.TimerType == "" {
			currentMapping.TimerType = n.Defaults.TimerType
		}

		if currentMapping.Buckets == nil || len(currentMapping.Buckets) == 0 {
			currentMapping.Buckets = n.Defaults.Buckets
		}

	}
	return &n, nil
}

// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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

package service

import (
	"time"

	"github.com/goodrain/rainbond/util"

	yaml "gopkg.in/yaml.v2"
)

const (
	Stat_healthy   string = "healthy"   //健康
	Stat_unhealthy string = "unhealthy" //出现异常
	Stat_death     string = "death"     //请求不通
)

//Service Service
type Service struct {
	Name            string      `yaml:"name"`
	Endpoints       []*Endpoint `yaml:"endpoints,omitempty"`
	ServiceHealth   *Health     `yaml:"health"`
	OnlyHealthCheck bool        `yaml:"only_health_check"`
	IsInitStart     bool        `yaml:"is_init_start"`
	Disable         bool        `yaml:"disable"`
	After           []string    `yaml:"after"`
	Requires        []string    `yaml:"requires"`
	Type            string      `yaml:"type,omitempty"`
	PreStart        string      `yaml:"pre_start,omitempty"`
	Start           string      `yaml:"start"`
	Stop            string      `yaml:"stop,omitempty"`
	RestartPolicy   string      `yaml:"restart_policy,omitempty"`
	RestartSec      string      `yaml:"restart_sec,omitempty"`
}

//Equal equal
func (s *Service) Equal(e *Service) bool {
	sb, err := yaml.Marshal(s)
	if err != nil {
		return false
	}
	eb, err := yaml.Marshal(e)
	if err != nil {
		return false
	}
	if util.BytesSliceEqual(sb, eb) {
		return true
	}
	return false
}

//Services default config of all services
type Services struct {
	Version  string     `yaml:"version"`
	Services []*Service `yaml:"services"`
}

//Endpoint endpoint
type Endpoint struct {
	Name     string `yaml:"name"`
	Protocol string `yaml:"protocol"`
	Port     string `yaml:"port"`
}

//Health ServiceHealth
type Health struct {
	Name         string `yaml:"name"`
	Model        string `yaml:"model"`
	Address      string `yaml:"address"`
	TimeInterval int    `yaml:"time_interval"`
	MaxErrorsNum int    `yaml:"max_errors_num"`
}

//HealthStatus health status
type HealthStatus struct {
	Name           string
	Status         string
	ErrorNumber    int
	ErrorDuration  time.Duration
	StartErrorTime time.Time
	Info           string
}

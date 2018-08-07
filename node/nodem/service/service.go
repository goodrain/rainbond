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

import "time"

const (
	Stat_healthy   string = "healthy"   //健康
	Stat_unhealthy string = "unhealthy" //出现异常
	Stat_death     string = "death"     //请求不通
)

//Service Service
type Service struct {
	Name          string      `yaml:"name"`
	Endpoints     []*Endpoint `yaml:"endpoints,omitempty"`
	ServiceHealth *Health     `yaml:"health"`
	After         []string    `yaml:"after"`
	Requires      []string    `yaml:"requires"`
	Type          string      `yaml:"type,omitempty"`
	PreStart      string      `yaml:"pre_start,omitempty"`
	Start         string      `yaml:"start"`
	Stop          string      `yaml:"stop,omitempty"`
	RestartPolicy string      `yaml:"restart_policy,omitempty"`
	RestartSec    string      `yaml:"restart_sec,omitempty"`
}

// default config of all services
type Services struct {
	Version  string     `yaml:"version"`
	Services []*Service `yaml:"services"`
}

// service list of the node
type ServiceList struct {
	Version  string `yaml:"version"`
	Services []struct {
		Name string `yaml:"name"`
	} `yaml:"services"`
}

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
}

type HealthStatus struct {
	Name        string
	Status      string
	ErrorNumber int
	ErrorTime   time.Duration
	Info        string
}

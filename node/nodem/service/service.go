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

//Service Service
type Service struct {
	Name          string   `yaml:"name"`
	ServiceHealth *Health  `yaml:"service_health"`
	Unit          []string `yaml:"unit"`
	Service       []string `yaml:"service"`
	Install       []string `yaml:"install"`
}

// default config for all services
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

//Health ServiceHealth
type Health struct {
	Name string`yaml:"name"`
	Model string `yaml:"model"`
	Address string `yaml:"addr"`
	Path string `yaml:"path"`
}

type HealthStatus struct {
	Name string`yaml:"name"`
	Status string `yaml:"status"`
	Info string `yaml:"info"`
}

type ProbeResult struct {
	Name string`yaml:"name"`
	Status string `yaml:"name"`
	Info   string `yaml:"name"`
}
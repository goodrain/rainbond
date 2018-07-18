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

import "fmt"

//Service Service
type Service struct {
	Name            string    `yaml:"name"`
	ServiceRegistry *Registry `yaml:"registry,omitempty"`
	ServiceHealth   *Health   `yaml:"health"`
	Dependences     []string  `yaml:"dependences"`
	Type            string    `yaml:"type,omitempty"`
	PreStart        string    `yaml:"pre_start,omitempty"`
	Start           string    `yaml:"start"`
	Stop            string    `yaml:"stop,omitempty"`
	RestartPolicy   string    `yaml:"restart_policy,omitempty"`
	RestartSec      string    `yaml:"restart_sec,omitempty"`
}

func (s *Service) GetRegKey() string {
	return s.Name
}

func (s *Service) GetRegValue(ip string) string {
	if s.ServiceRegistry.Protocol == "" {
		return fmt.Sprintf("%s:%s", ip, s.ServiceRegistry.Port)
	}
	return fmt.Sprintf("%s://%s:%s", s.ServiceRegistry.Protocol, ip, s.ServiceRegistry.Port)
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

type Registry struct {
	Protocol string `yaml:"protocol,omitempty"`
	Port     string `yaml:"port,omitempty"`
}

//Health ServiceHealth
type Health struct {
	Name    string `yaml:"name,omitempty"`
	Model   string `yaml:"model"`
	Address string `yaml:"addr"`
	Path    string `yaml:"path,omitempty"`
}

type HealthStatus struct {
	Name   string `yaml:"name"`
	Status string `yaml:"status"`
	Info   string `yaml:"info"`
}

type ProbeResult struct {
	Name   string `yaml:"name"`
	Status string `yaml:"name"`
	Info   string `yaml:"name"`
}

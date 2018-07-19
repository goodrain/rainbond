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

package controller

import (
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/node/nodem/service"
	"fmt"
)

type ManagerService struct {
	controller Controller
	cluster    client.ClusterClient
}

func (m *ManagerService) GetAllService() ([]*service.Service, error) {
	return m.controller.GetAllService(), nil
}

// start manager
func (m *ManagerService) Start() error {
	logrus.Info("Starting node controller manager.")
	return m.Online()
}

// stop manager
func (m *ManagerService) Stop() error {
	return nil
}

// start all service of on the node
func (m *ManagerService) Online() error {
	// registry local services endpoint into cluster manager
	hostIp := m.cluster.GetOptions().HostIP
	services, _ := m.GetAllService()
	for _, s := range services {
		for _, end := range s.Endpoints {
			endpoint := toEndpoint(end, hostIp)
			oldEndpoints := m.cluster.GetEndpoints(end.Name)
			if exist := isExistEndpoint(oldEndpoints, endpoint); !exist {
				oldEndpoints = append(oldEndpoints, endpoint)
				m.cluster.SetEndpoints(end.Name, oldEndpoints)
			}
		}
	}

	if err := m.controller.ReLoadServices(); err != nil {
		return err
	}

	return nil
}

// stop all service of on the node
func (m *ManagerService) Offline() error {
	// Anti-registry local services endpoint from cluster manager
	hostIp := m.cluster.GetOptions().HostIP
	services, _ := m.GetAllService()
	for _, s := range services {
		for _, end := range s.Endpoints {
			endpoint := toEndpoint(end, hostIp)
			oldEndpoints := m.cluster.GetEndpoints(end.Name)
			if exist := isExistEndpoint(oldEndpoints, endpoint); exist {
				m.cluster.SetEndpoints(end.Name, rmEndpointFrom(oldEndpoints, endpoint))
			}
		}
	}

	if err := m.controller.StopAll(); err != nil {
		return err
	}

	return nil
}

func isExistEndpoint(etcdEndPoints []string, end string) bool {
	for _, v := range etcdEndPoints {
		if v == end {
			return true
		}
	}
	return false
}

func rmEndpointFrom(etcdEndPoints []string, end string) []string {
	endPoints := make([]string, 0, 5)
	for _, v := range etcdEndPoints {
		if v != end {
			endPoints = append(endPoints, v)
		}
	}
	return endPoints
}

func toEndpoint(reg *service.Endpoint, ip string) string {
	if reg.Protocol == "" {
		return fmt.Sprintf("%s:%s", ip, reg.Port)
	}
	return fmt.Sprintf("%s://%s:%s", reg.Protocol, ip, reg.Port)
}

func NewManagerService(conf *option.Conf, cluster client.ClusterClient) *ManagerService {
	return &ManagerService{
		NewControllerSystemd(conf, cluster),
		cluster,
	}
}

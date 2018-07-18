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
	config, _ := m.cluster.GetDataCenterConfig()
	hostIp := config.GetOptions().HostIP
	services, _ := m.GetAllService()
	for _, s := range services {
		key := s.GetRegKey()
		oldEndpoints := m.cluster.GetEndpoints(key)
		if exist := isExistEndpoint(oldEndpoints, s.GetRegValue(hostIp)); !exist {
			oldEndpoints = append(oldEndpoints, s.GetRegValue(hostIp))
			m.cluster.SetEndpoints(key, oldEndpoints)
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
	config, _ := m.cluster.GetDataCenterConfig()
	hostIp := config.GetOptions().HostIP
	services, _ := m.GetAllService()
	for _, s := range services {
		key := s.GetRegKey()
		endPoint := s.GetRegValue(hostIp)
		oldEndpoints := m.cluster.GetEndpoints(key)
		if exist := isExistEndpoint(oldEndpoints, endPoint); exist {
			m.cluster.SetEndpoints(key, rmEndpointFrom(oldEndpoints, endPoint))
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

func NewManagerService(conf *option.Conf, cluster client.ClusterClient) *ManagerService {
	return &ManagerService{
		NewControllerSystemd(conf, cluster),
		cluster,
	}
}

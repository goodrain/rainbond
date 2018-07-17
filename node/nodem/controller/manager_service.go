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
	"github.com/goodrain/rainbond/node/nodem/service"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/nodem/client"
)

type ManagerService struct {
	controller Controller
}

func (m *ManagerService) GetAllService() ([]*service.Service, error) {
	return m.controller.GetAllService(), nil
}

func (m *ManagerService) Start() error {
	logrus.Info("Starting node controller manager with linux.")
	return m.controller.ReLoadServices()
}

func (m *ManagerService) Stop() error {
	return nil
}

func NewManagerService(conf *option.Conf, cluster client.ClusterClient) *ManagerService {
	return &ManagerService{
		NewControllerSystemd(conf, cluster),
	}
}
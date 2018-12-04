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

//+build windows
package controller

import (
	"fmt"
	"strings"

	"github.com/Sirupsen/logrus"

	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/nodem/service"
	"github.com/goodrain/rainbond/util/windows"
)

//NewController At the stage you want to load the configurations of all rainbond components
func NewController(conf *option.Conf, manager *ManagerService) Controller {
	logrus.Infof("Create windows service controller")
	return &windowsServiceController{
		conf:    conf,
		manager: manager,
	}
}

type windowsServiceController struct {
	conf    *option.Conf
	manager *ManagerService
}

func (w *windowsServiceController) InitStart(services []*service.Service) error {
	for _, s := range services {
		if s.IsInitStart {
			if err := w.WriteConfig(s); err != nil {
				return err
			}
			if err := w.StartService(s.Name); err != nil {
				return fmt.Errorf("start windows service %s failure %s", s.Name, err.Error())
			}
		}
	}
	return nil
}

func (w *windowsServiceController) StartService(name string) error {
	return windows.StartService(name)
}
func (w *windowsServiceController) StopService(name string) error {
	return windows.StopService(name)
}
func (w *windowsServiceController) StartList(list []*service.Service) error {
	for _, s := range list {
		if err := w.StartService(s.Name); err != nil {
			logrus.Errorf("start service %s failure %s", s.Name, err.Error())
		}
	}
	return nil
}
func (w *windowsServiceController) StopList(list []*service.Service) error {
	for _, s := range list {
		w.StopService(s.Name)
	}
	return nil
}
func (w *windowsServiceController) RestartService(serviceName string) error {
	return windows.RestartService(serviceName)
}
func (w *windowsServiceController) WriteConfig(s *service.Service) error {
	cmds := strings.Split(s.Start, " ")
	return windows.RegisterService(s.Name, cmds[0], "Rainbond "+s.Name, s.Requires, cmds)
}
func (w *windowsServiceController) RemoveConfig(name string) error {
	return windows.UnRegisterService(name)
}
func (w *windowsServiceController) EnableService(name string) error {
	return nil
}
func (w *windowsServiceController) DisableService(name string) error {
	return nil
}
func (w *windowsServiceController) CheckBeforeStart() bool {
	return false
}

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

	"github.com/sirupsen/logrus"

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
		if s.IsInitStart && !s.Disable && !s.OnlyHealthCheck {
			if err := w.writeConfig(s, false); err != nil {
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
	if err := windows.StartService(name); err != nil {
		logrus.Errorf("windows service controller start service %s failure %s", name, err.Error())
		return err
	}
	logrus.Infof("windows service controller start service %s success", name)
	return nil
}
func (w *windowsServiceController) StopService(name string) error {
	if err := windows.StopService(name); err != nil && !strings.Contains(err.Error(), "service has not been started") {
		logrus.Errorf("windows service controller stop service %s failure %s", name, err.Error())
		return err
	}
	logrus.Infof("windows service controller stop service %s success", name)
	return nil
}
func (w *windowsServiceController) StartList(list []*service.Service) error {
	for _, s := range list {
		w.StartService(s.Name)
	}
	return nil
}
func (w *windowsServiceController) StopList(list []*service.Service) error {
	for _, s := range list {
		w.StopService(s.Name)
	}
	return nil
}
func (w *windowsServiceController) RestartService(s *service.Service) error {
	if err := windows.RestartService(s.Name); err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			if _, err := w.WriteConfig(s); err != nil {
				return fmt.Errorf("ReWrite service config failure %s", err.Error())
			}
		}
		logrus.Errorf("windows service controller restart service %s failure %s", s.Name, err.Error())
		return err
	}
	logrus.Infof("windows service controller restart service %s success", s.Name)
	return nil
}
func (w *windowsServiceController) WriteConfig(s *service.Service) (bool, error) {
	return true, w.writeConfig(s, true)
}
func (w *windowsServiceController) writeConfig(s *service.Service, parseAndCoverOld bool) error {
	cmdstr := s.Start
	if parseAndCoverOld {
		cmdstr = w.manager.InjectConfig(s.Start)
	}
	cmds := strings.Split(cmdstr, " ")
	logrus.Debugf("write service %s config args %s", s.Name, cmds)
	if err := windows.RegisterService(s.Name, cmds[0], "Rainbond "+s.Name, s.Requires, cmds); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			if parseAndCoverOld {
				w.RemoveConfig(s.Name)
				err = windows.RegisterService(s.Name, cmds[0], "Rainbond "+s.Name, s.Requires, cmds)
			} else {
				logrus.Infof("windows service controller register service %s success(exist)", s.Name)
				return nil
			}
		}
		if err != nil {
			logrus.Errorf("windows service controller register service %s failure %s", s.Name, err.Error())
			return err
		}
	}
	logrus.Infof("windows service controller register service %s success", s.Name)
	return nil
}
func (w *windowsServiceController) RemoveConfig(name string) error {
	return windows.UnRegisterService(name)
}
func (w *windowsServiceController) EnableService(name string) error {
	return nil
}
func (w *windowsServiceController) DisableService(name string) error {
	//return windows.UnRegisterService(name)
	return nil
}
func (w *windowsServiceController) CheckBeforeStart() bool {
	return true
}

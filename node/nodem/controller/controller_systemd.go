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
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/node/nodem/service"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
)

type ControllerSystemd struct {
	SysConfigDir string
	NodeType     string
	StartType    string
	conf         *option.Conf
	cluster      client.ClusterClient
	regBlock     *regexp.Regexp
	ServiceCli   string
}

// At the stage you want to load the configurations of all rainbond components
func NewControllerSystemd(conf *option.Conf, cluster client.ClusterClient) *ControllerSystemd {
	cli, err := exec.LookPath("systemctl")
	if err != nil {
		panic(err)
	}
	return &ControllerSystemd{
		conf:         conf,
		cluster:      cluster,
		ServiceCli:   cli,
		SysConfigDir: "/etc/systemd/system",
	}
}

func (m *ControllerSystemd) CheckBeforeStart() bool {
	logrus.Info("Checking environments.")

	return true
}

func (m *ControllerSystemd) StartService(serviceName string) error {
	err := exec.Command(m.ServiceCli, "start", serviceName).Run()
	if err != nil {
		logrus.Errorf("Start service %s: %v", serviceName, err)
		return err
	}
	return nil
}

func (m *ControllerSystemd) StopService(serviceName string) error {
	err := exec.Command(m.ServiceCli, "stop", serviceName).Run()
	if err != nil {
		logrus.Errorf("Stop service %s: %v", serviceName, err)
		return err
	}
	return nil
}

func (m *ControllerSystemd) RestartService(serviceName string) error {
	err := exec.Command(m.ServiceCli, "restart", serviceName).Run()
	if err != nil {
		logrus.Errorf("Restart service %s: %v", serviceName, err)
		return err
	}

	return nil
}

func (m *ControllerSystemd) StartList(list []*service.Service) error {
	logrus.Info("Starting all services.")

	err := exec.Command(m.ServiceCli, "start", "multi-user.target").Run()
	if err != nil {
		logrus.Errorf("Start target multi-user: %v", err)
		return err
	}

	return nil
}

func (m *ControllerSystemd) StopList(list []*service.Service) error {
	logrus.Info("Stop all services.")
	for _, s := range list {
		err := exec.Command(m.ServiceCli, "stop", s.Name).Run()
		if err != nil {
			logrus.Errorf("Enable service %s: %v", s.Name, err)
		}
	}

	return nil
}

func (m *ControllerSystemd) EnableService(name string) error {
	logrus.Info("Enable service config by systemd.")
	err := exec.Command(m.ServiceCli, "enable", name).Run()
	if err != nil {
		logrus.Errorf("Enable service %s: %v", name, err)
	}

	return nil
}

func (m *ControllerSystemd) DisableService(name string) error {
	logrus.Info("Disable service config by systemd.")
	err := exec.Command(m.ServiceCli, "disable", name).Run()
	if err != nil {
		logrus.Errorf("Disable service %s: %v", name, err)
	}

	return nil
}

func (m *ControllerSystemd) WriteConfig(s *service.Service) error {
	fileName := fmt.Sprintf("%s/%s.service", m.SysConfigDir, s.Name)
	content := service.ToConfig(s)
	content = service.InjectConfig(content, m.cluster)
	if content == "" {
		err := fmt.Errorf("can not generate config for service %s", s.Name)
		logrus.Error(err)
		return err
	}

	if err := ioutil.WriteFile(fileName, []byte(content), 0644); err != nil {
		logrus.Errorf("Generate config file %s: %v", fileName, err)
		return err
	}

	logrus.Info("Reload config for systemd by daemon-reload.")
	err := exec.Command(m.ServiceCli, "daemon-reload").Run()
	if err != nil {
		logrus.Errorf("Reload config by systemd daemon-reload for %s: %v ", s.Name, err)
		return err
	}

	return nil
}

func (m *ControllerSystemd) RemoveConfig(name string) error {
	logrus.Info("Remote service config by systemd.")
	fileName := fmt.Sprintf("%s/%s.service", m.SysConfigDir, name)
	_, err := os.Stat(fileName)
	if err == nil {
		os.Remove(fileName)
	}

	logrus.Info("Reload config for systemd by daemon-reload.")
	err = exec.Command(m.ServiceCli, "daemon-reload").Run()
	if err != nil {
		logrus.Errorf("Reload config by systemd daemon-reload for %s: %v ", name, err)
		return err
	}

	return nil
}

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

// +build linux
package controller

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/node/nodem/service"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
)

type ControllerSystemd struct {
	SysConfigDir string
	NodeType     string
	StartType    string

	// all services
	services []*service.Service
	conf     *option.Conf
	cluster  client.ClusterClient
	regBlock *regexp.Regexp
}

// At the stage you want to load the configurations of all rainbond components
func NewControllerSystemd(conf *option.Conf, cluster client.ClusterClient) *ControllerSystemd {
	return &ControllerSystemd{
		conf:         conf,
		cluster:      cluster,
		SysConfigDir: "/etc/systemd/system",
	}
}

// for all rainbond components generate config file of systemd
func (m *ControllerSystemd) WriteAllConfig() error {
	logrus.Info("Write all service config to systemd.")
	for _, v := range m.services {
		fileName := fmt.Sprintf("%s/%s.service", m.SysConfigDir, v.Name)
		content := service.ToConfig(v, m.cluster)
		if content == nil {
			logrus.Error("can not generate config for service ", v.Name)
			continue
		}
		if err := ioutil.WriteFile(fileName, content, 0644); err != nil {
			logrus.Warnf("Generate config file %s: %v, has been ignored.", fileName, err)
		}
	}

	err := exec.Command("/usr/bin/systemctl", "daemon-reload").Run()
	if err != nil {
		logrus.Errorf("reload all services %s: %v", err)
	}

	return nil
}

func (m *ControllerSystemd) RemoveAllConfig() error {
	logrus.Info("Remote all service config to systemd.")
	for _, v := range m.services {
		fileName := fmt.Sprintf("%s/%s.service", m.SysConfigDir, v.Name)
		_, err := os.Stat(fileName)
		if err == nil {
			os.Remove(fileName)
		}
	}

	return nil
}

func (m *ControllerSystemd) EnableAll() error {
	logrus.Info("Enable all services.")
	for _, s := range m.services {
		err := exec.Command("/usr/bin/systemctl", "enable", s.Name).Run()
		if err != nil {
			logrus.Errorf("Enable service %s: %v", s.Name, err)
		}
	}

	return nil
}

func (m *ControllerSystemd) DisableAll() error {
	logrus.Info("Disable all service config to systemd.")
	for _, s := range m.services {
		err := exec.Command("/usr/bin/systemctl", "disable", s.Name).Run()
		if err != nil {
			logrus.Errorf("Disable service %s: %v", s.Name, err)
		}
	}

	return nil
}

func (m *ControllerSystemd) CheckBeforeStart() bool {
	logrus.Info("Checking environments.")

	return true
}

func (m *ControllerSystemd) StartAll() error {
	logrus.Info("Starting all services.")

	err := exec.Command("/usr/bin/systemctl", "start", "multi-user.target").Run()
	if err != nil {
		logrus.Errorf("Start target multi-user: %v", err)
		return err
	}

	return nil
}

func (m *ControllerSystemd) StartByName(serviceName string) error {
	err := exec.Command("/usr/bin/systemctl", "start", serviceName).Run()
	if err != nil {
		logrus.Errorf("Start service %s: %v", serviceName, err)
		return err
	}
	return nil
}

func (m *ControllerSystemd) StopAll() error {
	logrus.Info("Stop all services.")
	for _, s := range m.services {
		err := exec.Command("/usr/bin/systemctl", "stop", s.Name).Run()
		if err != nil {
			logrus.Errorf("Enable service %s: %v", s.Name, err)
		}
	}

	return nil
}

func (m *ControllerSystemd) StopByName(serviceName string) error {
	err := exec.Command("/usr/bin/systemctl", "stop", serviceName).Run()
	if err != nil {
		logrus.Errorf("Stop service %s: %v", serviceName, err)
		return err
	}
	return nil
}

func LoadServices(defaultConfigFile, serviceListFile string) ([]*service.Service, error) {
	logrus.Info("Loading all services.")

	services, err := loadServicesFromLocal(defaultConfigFile, serviceListFile)
	if err != nil {
		return nil, err
	}

	return services, nil
}

func (m *ControllerSystemd) GetAllService() []*service.Service {
	return m.services
}

/*
1. reload services config from local file system
2. regenerate systemd config
3. start all services of status is not running
*/
func (m *ControllerSystemd) ReLoadServices() error {
	services, err := LoadServices(m.conf.DefaultConfigFile, m.conf.ServiceListFile)
	if err != nil {
		logrus.Error("Failed to load all services: ", err)
		return err
	}
	m.services = services

	if err := m.WriteAllConfig(); err != nil {
		return err
	}

	m.DisableAll()
	if err := m.EnableAll(); err != nil {
		return err
	}

	if ok := m.CheckBeforeStart(); !ok {
		return fmt.Errorf("check environments is not passed")
	}

	m.StartAll()

	return nil
}

func loadServicesFromLocal(defaultConfigFile, serviceListFile string) ([]*service.Service, error) {
	logrus.Info("Loading all services from local.")

	// load default-configs.yaml
	content, err := ioutil.ReadFile(defaultConfigFile)
	if err != nil {
		logrus.Error("Failed to read default configs file: ", err)
		return nil, err
	}
	var defaultConfigs service.Services
	err = yaml.Unmarshal(content, &defaultConfigs)
	if err != nil {
		logrus.Error("Failed to parse default configs yaml file: ", err)
		return nil, err
	}
	// to map, reduce time complexity
	defaultConfigsMap := make(map[string]*service.Service, len(defaultConfigs.Services))
	for _, v := range defaultConfigs.Services {
		defaultConfigsMap[v.Name] = v
	}

	// load type-service.yaml, e.g. manager-service.yaml
	content, err = ioutil.ReadFile(serviceListFile)
	if err != nil {
		logrus.Error("Failed to read service list file: ", err)
		return nil, err
	}
	var serviceList service.ServiceList
	err = yaml.Unmarshal(content, &serviceList)
	if err != nil {
		logrus.Error("Failed to parse service list yaml file: ", err)
		return nil, err
	}

	// parse services with the node type
	services := make([]*service.Service, 0, len(defaultConfigs.Services))
	for _, item := range serviceList.Services {
		if s, ok := defaultConfigsMap[item.Name]; ok {
			services = append(services, s)
		} else {
			logrus.Warn("Not found the service %s in default config list, ignore it.", item.Name)
		}
	}

	return services, nil
}

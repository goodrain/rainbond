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
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os/exec"
	"os"
	"github.com/goodrain/rainbond/node/nodem/service"
)

type LinuxManager struct {
	SysConfigDir string
	NodeType     string
	StartType    string

	// all services
	services []*service.Service
	conf       *option.Conf
}

// At the stage you want to load the configurations of all rainbond components
func NewLinuxManager(conf *option.Conf) *LinuxManager {
	services, err := LoadServices(conf.DefaultConfigFile, conf.ServiceListFile)
	if err != nil {
		logrus.Error("Failed to new linux manager: ", err)
		panic(err)
	}

	return &LinuxManager{
		conf:         conf,
		SysConfigDir: "/etc/systemd/system",
		services:     services,
	}
}

func (m *LinuxManager) GetAllService() ([]*service.Service, error) {
	return m.services, nil
}

func (m *LinuxManager) Start() error {
	if err := m.GenerateAndOverwriteAllConfig(); err != nil {
		return err
	}

	m.DisableAll()
	if err := m.EnableAll(); err != nil {
		return err
	}

	m.CheckBeforeUp()

	m.StartAll()

	return nil
}

func (m *LinuxManager) Stop() error {
	return nil
}

// for all rainbond components generate config file of systemd
func (m *LinuxManager) GenerateAndOverwriteAllConfig() error {
	for _, v := range m.services {
		fileName := fmt.Sprintf("%s/%s.service", m.SysConfigDir, v.Name)
		if err := ioutil.WriteFile(fileName, m.ToConfig(v), 0644); err != nil {
			logrus.Warnf("Generate config file %s: %v, has been ignored.", fileName, err)
		}
	}

	return nil
}

func (m *LinuxManager) ToConfig(s *service.Service) []byte {
	result := "[Unit]"
	for i := range s.Unit {
		result = fmt.Sprintf("%s\n%s", result, s.Unit[i])
	}

	result = fmt.Sprintf("%s\n[Service]", result)
	for i := range s.Service {
		result = fmt.Sprintf("%s\n%s", result, s.Service[i])
	}

	result = fmt.Sprintf("%s\n[Install]", result)
	for i := range s.Install {
		result = fmt.Sprintf("%s\n%s", result, s.Install[i])
	}

	return []byte(result)
}

func (m *LinuxManager) RemoveAllConfig() error {
	for _, v := range m.services {
		fileName := fmt.Sprintf("%s/%s.service", m.SysConfigDir, v.Name)
		_, err := os.Stat(fileName)
		if err == nil {
			os.Remove(fileName)
		}
	}

	return nil
}

// TODO
func (m *LinuxManager) EnableAll() error {
	for _, s := range m.services {
		err := exec.Command("/usr/bin/systemctl", "enable", s.Name).Run()
		if err != nil {
			logrus.Errorf("Enable service %s: %v", s.Name, err)
		}
	}

	return nil
}

func (m *LinuxManager) DisableAll() error {
	for _, s := range m.services {
		err := exec.Command("/usr/bin/systemctl", "disable", s.Name).Run()
		if err != nil {
			logrus.Errorf("Disable service %s: %v", s.Name, err)
		}
	}

	return nil
}

// TODO
func (m *LinuxManager) CheckBeforeUp() bool {

	return true
}

func (m *LinuxManager) StartAll() error {
	m.DisableAll()
	err := m.EnableAll()
	if err != nil {
		logrus.Errorf("Start all service: %v", err)
		return err
	}

	err = exec.Command("/usr/bin/systemctl", "start", "multi-user.target").Run()
	if err != nil {
		logrus.Errorf("Start target multi-user: %v", err)
		return err
	}

	return nil
}

func (m *LinuxManager) StartByName(serviceName string) error {
	err := exec.Command("/usr/bin/systemctl", "start", serviceName).Run()
	if err != nil {
		logrus.Errorf("Start service %s: %v", serviceName, err)
		return err
	}
	return nil
}

func (m *LinuxManager) StopAll() error {
	for _, s := range m.services {
		err := exec.Command("/usr/bin/systemctl", "stop", s.Name).Run()
		if err != nil {
			logrus.Errorf("Enable service %s: %v", s.Name, err)
		}
	}

	return nil
}

func (m *LinuxManager) StopByName(serviceName string) error {
	err := exec.Command("/usr/bin/systemctl", "stop", serviceName).Run()
	if err != nil {
		logrus.Errorf("Stop service %s: %v", serviceName, err)
		return err
	}
	return nil
}

// TODO
func GetStartType() string {

	return ""
}

// TODO
func LoadServices(defaultConfigFile, serviceListFile string) ([]*service.Service, error) {
	t := GetStartType()
	switch t {
	case Init:
		return loadServicesFromLocal(defaultConfigFile, serviceListFile)
	case Add:

	case Start:

	}
	return nil, nil
}

func (m *LinuxManager) ReLoadServices() error {
	defaultConfigFile := m.conf.DefaultConfigFile
	serviceListFile := m.conf.ServiceListFile

	services, err := LoadServices(defaultConfigFile, serviceListFile)
	if err != nil {
		logrus.Error("Filed to reload services info: ", err)
		return err
	}
	m.services = services

	return nil
}

func loadServicesFromLocal(defaultConfigFile, serviceListFile string) ([]*service.Service, error) {
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
	services := make([]*service.Service, len(defaultConfigs.Services))
	for _, item := range serviceList.Services {
		if s, ok := defaultConfigsMap[item.Name]; ok {
			services = append(services, s)
		} else {
			logrus.Warn("Not found the service %s in default config list, ignore it.", item.Name)
		}
	}

	return services, nil
}

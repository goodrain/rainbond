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
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/nodem/service"
	"github.com/sirupsen/logrus"
)

type ControllerSystemd struct {
	SysConfigDir string
	NodeType     string
	StartType    string
	conf         *option.Conf
	manager      *ManagerService
	regBlock     *regexp.Regexp
	ServiceCli   string
}

//NewController At the stage you want to load the configurations of all rainbond components
func NewController(conf *option.Conf, manager *ManagerService) Controller {
	logrus.Infof("Create linux systemd controller")
	cli, err := exec.LookPath("systemctl")
	if err != nil {
		logrus.Errorf("current machine do not have systemctl utils")
	}
	return &ControllerSystemd{
		conf:         conf,
		manager:      manager,
		ServiceCli:   cli,
		SysConfigDir: "/etc/systemd/system",
	}
}

//CheckBeforeStart check before start
func (m *ControllerSystemd) CheckBeforeStart() bool {
	logrus.Info("Checking environments.")
	if m.ServiceCli == "" {
		logrus.Errorf("current machine do not have systemctl utils")
		return false
	}
	return true
}

//StartService start service
func (m *ControllerSystemd) StartService(serviceName string) error {
	err := m.run("start", serviceName)
	if err != nil {
		logrus.Errorf("Start service %s: %v", serviceName, err)
		return err
	}
	return nil
}

//StopService stop service
func (m *ControllerSystemd) StopService(serviceName string) error {
	err := m.run("stop", serviceName)
	if err != nil {
		logrus.Errorf("Stop service %s: %v", serviceName, err)
		return err
	}
	return nil
}

//RestartService restart service
func (m *ControllerSystemd) RestartService(s *service.Service) error {
	err := m.run("restart", s.Name)
	if err != nil {
		logrus.Errorf("Restart service %s: %v", s.Name, err)
		return err
	}

	return nil
}

//StartList start some service
func (m *ControllerSystemd) StartList(list []*service.Service) error {
	logrus.Infof("Starting %d all services.", len(list))
	for _, s := range list {
		m.StartService(s.Name)
	}
	return nil
}

//StopList stop some service
func (m *ControllerSystemd) StopList(list []*service.Service) error {
	logrus.Info("Stop all services.")
	for _, s := range list {
		m.StopService(s.Name)
	}
	return nil
}

//EnableService enable service
func (m *ControllerSystemd) EnableService(serviceName string) error {
	logrus.Info("Enable service config by systemd.")
	err := m.run("enable", serviceName)
	if err != nil {
		logrus.Errorf("Enable service %s: %v", serviceName, err)
	}

	return nil
}

//DisableService disable service
func (m *ControllerSystemd) DisableService(serviceName string) error {
	logrus.Info("Disable service config by systemd.")
	err := m.run("disable", serviceName)
	if err != nil {
		logrus.Errorf("Disable service %s: %v", serviceName, err)
	}

	return nil
}

//WriteConfig write config
//The first parameter returned represents whether there has been a change
//If I write it the first time, there is no change
func (m *ControllerSystemd) WriteConfig(s *service.Service) (bool, error) {
	var isChange = false
	fileName := fmt.Sprintf("%s/%s.service", m.SysConfigDir, s.Name)
	content := ToConfig(s)
	content = m.manager.InjectConfig(content)
	if content == "" {
		err := fmt.Errorf("can not generate config for service %s", s.Name)
		logrus.Error(err)
		return isChange, err
	}
	if old, err := ioutil.ReadFile(fileName); err == nil && old != nil {
		if string(old) != content {
			isChange = true
		} else {
			return isChange, nil
		}
	}
	if err := ioutil.WriteFile(fileName, []byte(content), 0644); err != nil {
		logrus.Errorf("Generate config file %s: %v", fileName, err)
		return isChange, err
	}
	logrus.Info("Reload config for systemd by daemon-reload.")
	err := m.run("daemon-reload")
	if err != nil {
		logrus.Errorf("Reload config by systemd daemon-reload for %s: %v ", s.Name, err)
		return isChange, err
	}
	return isChange, nil
}

//RemoveConfig remove config
func (m *ControllerSystemd) RemoveConfig(name string) error {
	logrus.Info("Remote service config by systemd.")
	fileName := fmt.Sprintf("%s/%s.service", m.SysConfigDir, name)
	_, err := os.Stat(fileName)
	if err == nil {
		os.Remove(fileName)
	}

	logrus.Info("Reload config for systemd by daemon-reload.")
	err = m.run("daemon-reload")
	if err != nil {
		logrus.Errorf("Reload config by systemd daemon-reload for %s: %v ", name, err)
		return err
	}

	return nil
}

func (m *ControllerSystemd) run(args ...string) error {
	err := exec.Command(m.ServiceCli, args...).Run()
	if err != nil {
		logrus.Errorf("systemd run error: %v", err)
		return err
	}
	return nil
}

//InitStart init start. will start some required service
func (m *ControllerSystemd) InitStart(services []*service.Service) error {
	for _, s := range services {
		if s.IsInitStart && !s.Disable && !s.OnlyHealthCheck {
			fileName := fmt.Sprintf("/etc/systemd/system/%s.service", s.Name)
			//init start can not read cluster endpoint.
			//so do not change the configuration file as much as possible
			_, err := os.Open(fileName)
			if err != nil && os.IsNotExist(err) {
				content := ToConfig(s)
				if content == "" {
					err := fmt.Errorf("can not generate config for service %s", s.Name)
					fmt.Println(err)
					return err
				}
				//init service start before etcd ready. so it can not set
				//content = m.manager.InjectConfig(content)
				if err := ioutil.WriteFile(fileName, []byte(content), 0644); err != nil {
					fmt.Printf("Generate config file %s: %v", fileName, err)
					return err
				}
			}
			if err := m.run("start", s.Name); err != nil {
				return fmt.Errorf("systemctl start %s error:%s", s.Name, err.Error())
			}
		}
	}
	return nil
}

func ToConfig(svc *service.Service) string {
	if svc.Start == "" {
		logrus.Error("service start command is empty.")
		return ""
	}

	s := ConfigWriter{writer: bytes.NewBuffer(nil)}
	s.AddTitle("[Unit]")
	s.Add("Description", svc.Name)
	for _, d := range svc.After {
		dpd := d
		if !strings.Contains(dpd, ".") {
			dpd += ".service"
		}
		s.Add("After", dpd)
	}

	for _, d := range svc.Requires {
		dpd := d
		if !strings.Contains(dpd, ".") {
			dpd += ".service"
		}
		s.Add("Requires", dpd)
	}

	s.AddTitle("[Service]")
	if svc.Type == "oneshot" {
		s.Add("Type", svc.Type)
		s.Add("RemainAfterExit", "yes")
	}
	s.Add("ExecStartPre", fmt.Sprintf(`-/bin/bash -c '%s'`, svc.PreStart))
	s.Add("ExecStart", fmt.Sprintf(`/bin/bash -c '%s'`, svc.Start))
	s.Add("ExecStop", fmt.Sprintf(`/bin/bash -c '%s'`, svc.Stop))
	s.Add("Restart", svc.RestartPolicy)
	s.Add("RestartSec", svc.RestartSec)

	s.AddTitle("[Install]")
	s.Add("WantedBy", "multi-user.target")

	logrus.Debugf("check is need inject args into service %s", svc.Name)

	return s.Get()
}

type ConfigWriter struct {
	writer *bytes.Buffer
}

func (l *ConfigWriter) AddTitle(line string) {
	l.writer.WriteString("\n")
	l.writer.WriteString(line)
}

func (l *ConfigWriter) Add(k, v string) {
	if v == "" {
		return
	}
	l.writer.WriteString("\n")
	l.writer.WriteString(k)
	l.writer.WriteString("=")
	l.writer.WriteString(v)
}

func (l *ConfigWriter) Get() string {
	return l.writer.String()
}

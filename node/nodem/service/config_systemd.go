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

package service

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/goodrain/rainbond/util"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/node/nodem/client"
	"gopkg.in/yaml.v2"
)

var (
	ArgsReg = regexp.MustCompile(`\$\{(\w+)\|{0,1}(.{0,1})\}`)
)

//LoadServicesFromLocal load all service config from config file
func LoadServicesFromLocal(serviceListFile string) []*Service {
	var serviceList []*Service
	ok, err := util.IsDir(serviceListFile)
	if err != nil {
		logrus.Errorf("read service config file error,%s", err.Error())
		return nil
	}
	if !ok {
		services, err := loadServicesFromFile(serviceListFile)
		if err != nil {
			logrus.Errorf("read service config file %s error,%s", serviceListFile, err.Error())
			return nil
		}
		return services
	}
	filepath.Walk(serviceListFile, func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(path, "yaml") && !info.IsDir() {
			services, err := loadServicesFromFile(path)
			if err != nil {
				logrus.Errorf("read service config file %s error,%s", path, err.Error())
				return nil
			}
			serviceList = append(serviceList, services...)
		}
		return nil
	})
	result := removeRepByLoop(serviceList)
	logrus.Infof("load service config file success. load %d service", len(result))
	return result
}

func removeRepByLoop(source []*Service) (target []*Service) {
	for i, s := range source {
		flag := true
		for _, t := range target {
			if s.Name == t.Name {
				flag = false
				break
			}
		}
		if flag {
			target = append(target, source[i])
		}
	}
	return
}
func loadServicesFromFile(serviceListFile string) ([]*Service, error) {
	// load default-configs.yaml
	content, err := ioutil.ReadFile(serviceListFile)
	if err != nil {
		err = fmt.Errorf("Failed to read service list file: %s", err.Error())
		return nil, err
	}
	var defaultConfigs Services
	err = yaml.Unmarshal(content, &defaultConfigs)
	if err != nil {
		logrus.Error("Failed to parse default configs yaml file: ", err)
		return nil, err
	}
	return defaultConfigs.Services, nil
}

func ToConfig(svc *Service) string {
	if svc.Start == "" {
		logrus.Error("service start command is empty.")
		return ""
	}

	s := Lines{"[Unit]"}
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

func InjectConfig(content string, cluster client.ClusterClient) string {
	if cluster == nil {
		logrus.Error("cluster client can not is nil, at to config.")
		return ""
	}

	for _, parantheses := range ArgsReg.FindAllString(content, -1) {
		logrus.Debugf("discover inject args template %s", parantheses)
		group := ArgsReg.FindStringSubmatch(parantheses)
		if group == nil || len(group) < 2 {
			logrus.Warnf("Not found group for ", parantheses)
			continue
		}
		endpoints := cluster.GetEndpoints(group[1])
		if len(endpoints) < 1 {
			logrus.Warnf("Failed to inject endpoints of key %s", group[1])
			continue
		}
		sep := ","
		if len(group) >= 3 && group[2] != "" {
			sep = group[2]
		}
		line := ""
		for _, end := range endpoints {
			if line == "" {
				line = end
			} else {
				line += sep
				line += end
			}
		}
		content = strings.Replace(content, group[0], line, 1)
		logrus.Debugf("inject args into service %s => %v", group[1], endpoints)
	}

	return content
}

type Lines struct {
	str string
}

func (l *Lines) AddTitle(line string) {
	l.str = fmt.Sprintf("%s\n\n%s", l.str, line)
}

func (l *Lines) Add(k, v string) {
	if v == "" {
		return
	}
	l.str = fmt.Sprintf("%s\n%s=%s", l.str, k, v)
}

func (l *Lines) Get() string {
	return l.str
}

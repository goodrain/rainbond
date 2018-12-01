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
	"strings"

	"github.com/goodrain/rainbond/util"

	"github.com/Sirupsen/logrus"
	"gopkg.in/yaml.v2"
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

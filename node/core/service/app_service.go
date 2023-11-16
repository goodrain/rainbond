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
	"github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/discover/config"
	"github.com/goodrain/rainbond/node/core/store"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// AppService app service
type AppService struct {
	Prefix string
	c      *option.Conf
}

// CreateAppService create
func CreateAppService(c *option.Conf) *AppService {
	return &AppService{
		c:      c,
		Prefix: "/traefik",
	}
}

// FindAppEndpoints 获取app endpoint
func (a *AppService) FindAppEndpoints(appName string) []*config.Endpoint {
	var ends = make(map[string]*config.Endpoint)
	res, err := store.DefalutClient.Get(fmt.Sprintf("%s/backends/%s/servers", a.Prefix, appName), clientv3.WithPrefix())
	if err != nil {
		logrus.Errorf("list all servers of %s error.%s", appName, err.Error())
		return nil
	}
	if res.Count == 0 {
		return nil
	}
	for _, kv := range res.Kvs {
		if strings.HasSuffix(string(kv.Key), "/url") { //获取服务地址
			kstep := strings.Split(string(kv.Key), "/")
			if len(kstep) > 2 {
				serverName := kstep[len(kstep)-2]
				serverURL := string(kv.Value)
				if en, ok := ends[serverName]; ok {
					en.URL = serverURL
				} else {
					ends[serverName] = &config.Endpoint{Name: serverName, URL: serverURL}
				}
			}
		}
		if strings.HasSuffix(string(kv.Key), "/weight") { //获取服务权重
			kstep := strings.Split(string(kv.Key), "/")
			if len(kstep) > 2 {
				serverName := kstep[len(kstep)-2]
				serverWeight := string(kv.Value)
				if en, ok := ends[serverName]; ok {
					var err error
					en.Weight, err = strconv.Atoi(serverWeight)
					if err != nil {
						logrus.Error("get server weight error.", err.Error())
					}
				} else {
					weight, err := strconv.Atoi(serverWeight)
					if err != nil {
						logrus.Error("get server weight error.", err.Error())
					}
					ends[serverName] = &config.Endpoint{Name: serverName, Weight: weight}
				}
			}
		}
	}
	result := []*config.Endpoint{}
	for _, v := range ends {
		result = append(result, v)
	}
	return result
}

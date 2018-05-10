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

package discover

import (
	"errors"
	"github.com/goodrain/rainbond/eventlog/conf"
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/client"
)

var keyAPI client.KeysAPI
var dconf conf.DiscoverConf

//CreateETCDClient 创建etcd api
func CreateETCDClient(conf conf.DiscoverConf) (client.KeysAPI, error) {
	dconf = conf
	cfg := client.Config{
		Endpoints:               conf.EtcdAddr,
		Username:                conf.EtcdUser,
		Password:                conf.EtcdPass,
		HeaderTimeoutPerRequest: time.Second * 5,
	}
	c, err := client.New(cfg)
	if err != nil {
		return nil, err
	}
	keyAPI = client.NewKeysAPI(c)
	return keyAPI, nil
}

//SaveDockerLogInInstance 存储service和node 的对应关系
func SaveDockerLogInInstance(ctx context.Context, serviceID, instanceID string) error {
	if keyAPI == nil {
		return errors.New("etcd client is nil")
	}
	_, err := keyAPI.Set(ctx, dconf.HomePath+"/dockerloginstacne/"+serviceID, instanceID, &client.SetOptions{})
	if err != nil {
		if cerr, ok := err.(client.Error); ok {
			if cerr.Code == client.ErrorCodeNodeExist {
				_, err := keyAPI.Update(ctx, dconf.HomePath+"/dockerloginstacne/"+serviceID, instanceID)
				if err != nil {
					return err
				}
				return nil
			}
		}
		return err
	}
	return nil
}

//GetDokerLogInInstance 获取应用日志接收节点
func GetDokerLogInInstance(ctx context.Context, serviceID string) (string, error) {
	if keyAPI == nil {
		return "", errors.New("etcd client is nil")
	}
	res, err := keyAPI.Get(ctx, dconf.HomePath+"/dockerloginstacne/"+serviceID, &client.GetOptions{})
	if err != nil {
		if cerr, ok := err.(client.Error); ok {
			if cerr.Code == client.ErrorCodeKeyNotFound {
				return "", nil
			}
		}
		return "", err
	}
	return res.Node.Value, nil
}

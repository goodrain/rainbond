
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

package config

import (
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/goodrain/rainbond/pkg/node/core/store"

	"github.com/Sirupsen/logrus"
)

//DataCenterConfig 数据中心配置
type DataCenterConfig struct {
	config  *model.GlobalConfig
	options *option.Conf
}

//CreateDataCenterConfig 创建
func CreateDataCenterConfig(options *option.Conf) *DataCenterConfig {
	return &DataCenterConfig{
		options: options,
	}
}

//GetDataCenterConfig 获取配置
func (d *DataCenterConfig) GetDataCenterConfig() (*model.GlobalConfig, error) {
	res, err := store.DefalutClient.Get(d.options.ConfigStorage + "/datacenter")
	if err != nil {
		logrus.Error("get datacenter config error,", err.Error())
		return nil, err
	}
	logrus.Info(res)
	if res.Count == 0 {
		dgc := model.CreateDefaultGlobalConfig()
		err = d.PutDataCenterConfig(dgc)
		if err != nil {
			logrus.Error("put datacenter config error,", err.Error())
			return nil, err
		}
		d.config = dgc
	} else {
		for _, kv := range res.Kvs {
			dgc, err := model.CreateGlobalConfig(kv.Value)
			if err != nil {
				logrus.Error("get datacenter config error,", err.Error())
				return nil, err
			}
			d.config = dgc
			break
		}
	}
	return d.config, nil
}

//PutDataCenterConfig 更改配置
func (d *DataCenterConfig) PutDataCenterConfig(c *model.GlobalConfig) (err error) {
	_, err = store.DefalutClient.Put(d.options.ConfigStorage+"/datacenter", c.String())
	return err
}

//GetNetWorkConfig 获取网络配置
func (d *DataCenterConfig) GetNetWorkConfig() model.ConfigUnit {
	return d.config.NetWork
}

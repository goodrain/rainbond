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

package config

import (
	"context"
	"fmt"
	"github.com/goodrain/rainbond/pkg/gogo"

	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/pquerna/ffjson/ffjson"

	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/api/model"
	"github.com/goodrain/rainbond/node/core/store"
	"github.com/goodrain/rainbond/util"

	client "github.com/coreos/etcd/clientv3"
	"github.com/sirupsen/logrus"
)

// DataCenterConfig 数据中心配置
type DataCenterConfig struct {
	config  *model.GlobalConfig
	options *option.Conf
	ctx     context.Context
	cancel  context.CancelFunc
	//group config 不持久化
	groupConfigs map[string]*GroupContext
}

var dataCenterConfig *DataCenterConfig

// GetDataCenterConfig 获取
func GetDataCenterConfig() *DataCenterConfig {
	if dataCenterConfig == nil {
		return CreateDataCenterConfig()
	}
	return dataCenterConfig
}

// CreateDataCenterConfig 创建
func CreateDataCenterConfig() *DataCenterConfig {
	ctx, cancel := context.WithCancel(context.Background())
	dataCenterConfig = &DataCenterConfig{
		options: option.Config,
		ctx:     ctx,
		cancel:  cancel,
		config: &model.GlobalConfig{
			Configs: make(map[string]*model.ConfigUnit),
		},
		groupConfigs: make(map[string]*GroupContext),
	}
	res, err := store.DefalutClient.Get(dataCenterConfig.options.ConfigStoragePath+"/global", client.WithPrefix())
	if err != nil {
		logrus.Error("load datacenter config error.", err.Error())
	}
	if res != nil {
		if len(res.Kvs) < 1 {
			dgc := &model.GlobalConfig{
				Configs: make(map[string]*model.ConfigUnit),
			}
			dataCenterConfig.config = dgc
		} else {
			for _, kv := range res.Kvs {
				dataCenterConfig.PutConfigKV(kv)
			}
		}
	}
	return dataCenterConfig
}

// Start 启动，监听配置变化
func (d *DataCenterConfig) Start() {
	_ = gogo.Go(func(ctx context.Context) error {
		return util.Exec(d.ctx, func() error {
			ctx, cancel := context.WithCancel(d.ctx)
			defer cancel()
			logrus.Info("datacenter config listener start")
			ch := store.DefalutClient.WatchByCtx(ctx, d.options.ConfigStoragePath+"/global", client.WithPrefix())
			for event := range ch {
				for _, e := range event.Events {
					switch {
					case e.IsCreate(), e.IsModify():
						d.PutConfigKV(e.Kv)
					case e.Type == client.EventTypeDelete:
						d.DeleteConfig(util.GetIDFromKey(string(e.Kv.Key)))
					}
				}
			}
			return nil
		}, 1)
	})
}

// Stop 停止监听
func (d *DataCenterConfig) Stop() {
	d.cancel()
	logrus.Info("datacenter config listener stop")
}

// GetDataCenterConfig 获取配置
func (d *DataCenterConfig) GetDataCenterConfig() (*model.GlobalConfig, error) {
	return d.config, nil
}

// PutDataCenterConfig 更改配置
func (d *DataCenterConfig) PutDataCenterConfig(c *model.GlobalConfig) (err error) {
	if c == nil {
		return
	}
	for k, v := range c.Configs {
		d.config.Add(*v)
		_, err = store.DefalutClient.Put(d.options.ConfigStoragePath+"/global/"+k, v.String())
	}
	return err
}

// GetConfig 获取全局配置
func (d *DataCenterConfig) GetConfig(name string) *model.ConfigUnit {
	return d.config.Get(name)
}

// CacheConfig 更新配置缓存
func (d *DataCenterConfig) CacheConfig(c *model.ConfigUnit) error {
	if c.Name == "" {
		return fmt.Errorf("config name can not be empty")
	}
	logrus.Debugf("add config %v", c)
	//将值类型由[]interface{} 转 []string
	if c.ValueType == "array" {
		switch c.Value.(type) {
		case []interface{}:
			var data []string
			for _, v := range c.Value.([]interface{}) {
				data = append(data, v.(string))
			}
			c.Value = data
		}
		oldC := d.config.Get(c.Name)
		if oldC != nil {

			switch oldC.Value.(type) {
			case string:
				value := append(c.Value.([]string), oldC.Value.(string))
				util.Deweight(&value)
				c.Value = value
			case []string:
				value := append(c.Value.([]string), oldC.Value.([]string)...)
				util.Deweight(&value)
				c.Value = value
			default:
			}
		}
	}
	d.config.Add(*c)
	return nil
}

// PutConfig 增加or更新配置
func (d *DataCenterConfig) PutConfig(c *model.ConfigUnit) error {
	if c.Name == "" {
		return fmt.Errorf("config name can not be empty")
	}
	logrus.Debugf("add config %v", c)
	//将值类型由[]interface{} 转 []string
	if c.ValueType == "array" {
		switch c.Value.(type) {
		case []interface{}:
			var data []string
			for _, v := range c.Value.([]interface{}) {
				data = append(data, v.(string))
			}
			c.Value = data
		}
		oldC := d.config.Get(c.Name)
		if oldC != nil {

			switch oldC.Value.(type) {
			case string:
				value := append(c.Value.([]string), oldC.Value.(string))
				util.Deweight(&value)
				c.Value = value
			case []string:
				value := append(c.Value.([]string), oldC.Value.([]string)...)
				util.Deweight(&value)
				c.Value = value
			default:
			}
		}
	}
	d.config.Add(*c)
	//持久化
	_, err := store.DefalutClient.Put(d.options.ConfigStoragePath+"/global/"+c.Name, c.String())
	if err != nil {
		logrus.Error("put datacenter config to etcd error.", err.Error())
		return err
	}
	return nil
}

// PutConfigKV 更新
func (d *DataCenterConfig) PutConfigKV(kv *mvccpb.KeyValue) {
	var cn model.ConfigUnit
	if err := ffjson.Unmarshal(kv.Value, &cn); err == nil {
		d.CacheConfig(&cn)
	} else {
		logrus.Errorf("parse config error,%s", err.Error())
	}
}

// DeleteConfig 删除配置
func (d *DataCenterConfig) DeleteConfig(name string) {
	d.config.Delete(name)
}

// GetGroupConfig get group config
func (d *DataCenterConfig) GetGroupConfig(groupID string) *GroupContext {
	if c, ok := d.groupConfigs[groupID]; ok {
		return c
	}
	c := NewGroupContext(groupID)
	d.groupConfigs[groupID] = c
	return c
}

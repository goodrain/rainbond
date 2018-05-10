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

package store

import (
	"github.com/goodrain/rainbond/entrance/api/model"
	"github.com/goodrain/rainbond/cmd/entrance/option"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/clientv3"
)

type Manager struct {
	keysAPI        client.KeysAPI
	client         client.Client
	clientv3       *clientv3.Client
	conf           option.Config
	ctx            context.Context
	cancel         context.CancelFunc
	sourceTypeMaps map[string]reflect.Type
}

//NewManager create a manager
func NewManager(conf option.Config) (*Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	c, err := client.New(client.Config{
		Endpoints:               conf.EtcdEndPoints,
		HeaderTimeoutPerRequest: time.Second * time.Duration(conf.EtcdTimeout),
	})
	if err != nil {
		cancel()
		return nil, err
	}
	cliv3, err := clientv3.New(clientv3.Config{
		Endpoints:   conf.EtcdEndPoints,
		DialTimeout: time.Second * time.Duration(conf.EtcdTimeout),
	})
	if err != nil {
		cancel()
		return nil, err
	}
	m := &Manager{
		client:         c,
		clientv3:       cliv3,
		ctx:            ctx,
		cancel:         cancel,
		keysAPI:        client.NewKeysAPI(c),
		sourceTypeMaps: make(map[string]reflect.Type),
	}
	return m, nil
}

//Register 注册资源类型
func (m *Manager) Register(sourceName string, source interface{}) {
	m.sourceTypeMaps[sourceName] = reflect.TypeOf(source).Elem()
}

//New 创建资源实体
func (m *Manager) New(sourceName string) (c interface{}, err error) {
	if v, ok := m.sourceTypeMaps[sourceName]; ok {
		c = reflect.New(v).Interface()
	} else {
		err = fmt.Errorf("not found %s struct", sourceName)
	}
	return
}

//AddSource 添加资源
func (m *Manager) AddSource(key string, object interface{}) error {
	if key == "" {
		return errors.New("key or source can not nil")
	}
	var data string
	var err error
	switch object.(type) {
	case string:
		data = object.(string)
		break
	case int:
		data = fmt.Sprintf("%d", object)
		break
	case []byte:
		data = string(object.([]byte))
		break
	default:
		dataB, err := json.Marshal(object)
		if err != nil {
			return err
		}
		data = string(dataB)
	}
	_, err = m.keysAPI.Set(m.ctx, key, data, &client.SetOptions{
		PrevExist: client.PrevNoExist,
	})
	if err != nil {
		return err
	}
	return nil
}

//UpdateSource 更新资源
func (m *Manager) UpdateSource(key string, object interface{}) error {
	if key == "" {
		return errors.New("key or source can not nil")
	}
	var data string
	var err error
	switch object.(type) {
	case string:
		data = object.(string)
		break
	case int:
		data = fmt.Sprintf("%d", object)
		break
	case []byte:
		data = string(object.([]byte))
		break
	default:
		dataB, err := json.Marshal(object)
		if err != nil {
			return err
		}
		data = string(dataB)
	}
	_, err = m.keysAPI.Update(m.ctx, key, data)
	if err != nil {
		return err
	}
	return nil
}

//GetSource 获取资源
func (m *Manager) GetSource(key string, object interface{}) error {
	if key == "" || object == nil {
		return errors.New("key or source can not nil")
	}
	res, err := m.keysAPI.Get(m.ctx, key, &client.GetOptions{})
	if err != nil {
		return err
	}
	if res.Node != nil {
		return json.Unmarshal([]byte(res.Node.Value), object)
	}
	return errors.New("the value is null")
}

//GetV3Client v3API
func (m *Manager) GetV3Client() *clientv3.Client {
	return m.clientv3
}

//GetDomainList 获取资源列表
func (m *Manager) GetDomainList(key string) ([]interface{}, error) {
	if key == "" {
		return nil, errors.New("key  can not nil")
	}
	res, err := m.keysAPI.Get(m.ctx, key, &client.GetOptions{
		Recursive: true,
	})
	if err != nil {
		return nil, err
	}
	if !res.Node.Dir {
		return nil, fmt.Errorf("%s is not dir. don't list", key)
	}
	var list []interface{}
	if res.Node != nil {
		for _, node := range res.Node.Nodes {
			if node == nil {
				continue
			}
			m := model.Domain{}
			err := json.Unmarshal([]byte(node.Value), &m)
			if err != nil {
				logrus.Error("Unmarshal etcd value error.", err.Error())
			} else {
				list = append(list, m)
			}
		}
		return list, nil
	}
	return nil, errors.New("the value is null")
}

//GetSourceList 获取资源列表
//使用反射方式创建对象
//性能远低于直接创建对象
func (m *Manager) GetSourceList(key, sourceType string) ([]interface{}, error) {
	if key == "" {
		return nil, errors.New("key  can not nil")
	}
	res, err := m.keysAPI.Get(m.ctx, key, &client.GetOptions{
		Recursive: true,
	})
	if err != nil {
		return nil, err
	}
	if !res.Node.Dir {
		return nil, fmt.Errorf("%s is not dir. don't list", key)
	}
	var list []interface{}
	if res.Node != nil {
		for _, node := range res.Node.Nodes {
			source, err := m.New(sourceType)
			if err != nil {
				return nil, err
			}
			err = json.Unmarshal([]byte(node.Value), &source)
			if err != nil {
				logrus.Error("Unmarshal etcd value error.", err.Error())
			} else {
				list = append(list, source)
			}
		}
		return list, nil
	}
	return nil, errors.New("the value is null")
}

//DeleteSource 删除资源
func (m *Manager) DeleteSource(key string, dir bool) error {
	if key == "" {
		return errors.New("key or source can not nil")
	}
	_, err := m.keysAPI.Delete(m.ctx, key, &client.DeleteOptions{
		Recursive: dir,
		Dir:       dir,
	})
	if err != nil {
		return err
	}
	return nil
}

//GetDomainKey get domain key
func (m *Manager) GetDomainKey(tenantID, serviceID, domainID string) string {
	return fmt.Sprintf("/store/tenants/%s/services/%s/domains/%s", tenantID, serviceID, domainID)
}

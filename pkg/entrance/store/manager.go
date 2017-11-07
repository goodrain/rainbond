
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

package store

import (
	"github.com/goodrain/rainbond/pkg/entrance/cluster"
	"github.com/goodrain/rainbond/cmd/entrance/option"
	"github.com/goodrain/rainbond/pkg/entrance/core/object"
	"encoding/json"
	"errors"
	"time"

	"golang.org/x/net/context"

	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/client"
)

//Manager store manager
type Manager struct {
	cluster *cluster.Manager
	keysAPI client.KeysAPI
	client  client.Client
	conf    option.Config
	ctx     context.Context
	cancel  context.CancelFunc
}

//ErrTypeUnknown 资源类型未知
var ErrTypeUnknown = errors.New("object type unknown")

//NewManager create a manager
func NewManager(conf option.Config, cluster *cluster.Manager) (*Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	c, err := client.New(client.Config{
		Endpoints:               conf.EtcdEndPoints,
		HeaderTimeoutPerRequest: time.Second * time.Duration(conf.EtcdTimeout),
	})
	if err != nil {
		return nil, err
	}
	m := &Manager{
		client:  c,
		cluster: cluster,
		ctx:     ctx,
		cancel:  cancel,
		keysAPI: client.NewKeysAPI(c),
	}
	m.checkHealth()
	logrus.Info("store manager create.")
	return m, nil
}

//checkHealth 检测相关初始化资源是否创建
func (m *Manager) checkHealth() error {
	if err := m.createDir(m.cluster.GetPrefix() + "/pool"); err != nil {
		return fmt.Errorf("init etcd dir %s error.%s", m.cluster.GetPrefix()+"/pool", err.Error())
	}
	if err := m.createDir(m.cluster.GetPrefix() + "/vs"); err != nil {
		return fmt.Errorf("init etcd dir %s error.%s", m.cluster.GetPrefix()+"/vs", err.Error())
	}
	if err := m.createDir(m.cluster.GetPrefix() + "/rule/https"); err != nil {
		return fmt.Errorf("init etcd dir %s error.%s", m.cluster.GetPrefix()+"/rule/https", err.Error())
	}
	if err := m.createDir(m.cluster.GetPrefix() + "/rule/http"); err != nil {
		return fmt.Errorf("init etcd dir %s error.%s", m.cluster.GetPrefix()+"/rule/http", err.Error())
	}
	if err := m.createDir(m.cluster.GetPrefix() + "/domain"); err != nil {
		return fmt.Errorf("init etcd dir %s error.%s", m.cluster.GetPrefix()+"/domain", err.Error())
	}
	if err := m.createDir(m.cluster.GetPrefix() + "/node"); err != nil {
		return fmt.Errorf("init etcd dir %s error.%s", m.cluster.GetPrefix()+"/node", err.Error())
	}
	return nil
}
func (m *Manager) createDir(key string) error {
	res, err := m.keysAPI.Get(m.ctx, key, &client.GetOptions{})
	if err != nil {
		if cerr, ok := err.(client.Error); ok {
			if cerr.Code == client.ErrorCodeKeyNotFound {
				_, err := m.keysAPI.Set(m.ctx, key, "", &client.SetOptions{Dir: true})
				if err != nil {
					return err
				}
				return nil
			}
		}
		return err
	}
	if !res.Node.Dir {
		_, err := m.keysAPI.Delete(m.ctx, key, &client.DeleteOptions{})
		if err != nil {
			return err
		}
		_, err = m.keysAPI.Set(m.ctx, key, "", &client.SetOptions{Dir: true})
		if err != nil {
			return err
		}
		return nil
	}
	return err
}

//CreateMutex 创建分布式锁
func (m *Manager) CreateMutex(key string) *cluster.Mutex {
	return cluster.New(m.cluster.GetPrefix()+key, 20, m.client)
}

//GetSource 获取资源
func (m *Manager) GetSource(o object.Object) (object.Object, error) {
	switch o.(type) {
	case *object.PoolObject:
		i := &SourceInfo{
			Data: &object.PoolObject{},
		}
		err := m.get(m.cluster.GetPrefix()+"/pool/"+o.GetName(), i)
		if err != nil {
			return nil, err
		}
		return i.Data, nil
	case *object.VirtualServiceObject:
		i := &SourceInfo{
			Data: &object.VirtualServiceObject{},
		}
		err := m.get(m.cluster.GetPrefix()+"/vs/"+o.GetName(), i)
		if err != nil {
			return nil, err
		}
		return i.Data, nil
	case *object.RuleObject:
		i := &SourceInfo{
			Data: &object.RuleObject{},
		}
		err := m.get(m.cluster.GetPrefix()+"/rule/"+o.GetName(), i)
		if err != nil {
			return nil, err
		}
		return i.Data, nil
	case *object.NodeObject:
		node := o.(*object.NodeObject)
		i := &SourceInfo{
			Data: &object.NodeObject{},
		}
		err := m.get(m.cluster.GetPrefix()+"/node/"+node.PoolName+"/"+o.GetName(), i)
		if err != nil {
			return nil, err
		}
		return i.Data, nil
	case *object.DomainObject:
		i := &SourceInfo{
			Data: &object.DomainObject{},
		}
		err := m.get(m.cluster.GetPrefix()+"/domain/"+o.GetName(), i)
		if err != nil {
			return nil, err
		}
		return i.Data, nil
	default:
		return nil, ErrTypeUnknown
	}
}

//DeleteSource 删除资源
//if result=false and err==nil 资源无权操作
func (m *Manager) DeleteSource(o object.Object) (bool, error) {
	switch o.(type) {
	case *object.PoolObject:
		var ok bool
		var err error
		if ok, err = m.delete(m.cluster.GetPrefix() + "/pool/" + o.GetName()); err == nil {
			if _, err := m.delete(m.cluster.GetPrefix() + "/node/" + o.GetName()); err == nil {
				m.delete(m.cluster.GetPrefix() + "/rule/" + o.GetName())
			}
		}
		return ok, err
	case *object.VirtualServiceObject:
		return m.delete(m.cluster.GetPrefix() + "/vs/" + o.GetName())
	case *object.RuleObject:
		rule := o.(*object.RuleObject)
		if rule.HTTPS {
			key := fmt.Sprintf("%s/rule/https/%s/%s", m.cluster.GetPrefix(), rule.PoolName, rule.Name)
			return m.delete(key)
		}
		key := fmt.Sprintf("%s/rule/http/%s/%s", m.cluster.GetPrefix(), rule.PoolName, rule.Name)
		return m.delete(key)
	case *object.NodeObject:
		node := o.(*object.NodeObject)
		if len(node.Host) == 0 || node.Port == 0 {
			//更新源资源中没有host ip ,从旧数据中获取
			old, err := m.GetSource(o)
			if err == nil {
				//保证缓存数据中存在host 和port
				node.Host = old.(*object.NodeObject).Host
				node.Port = old.(*object.NodeObject).Port
				logrus.Infof("When delete node.Host is nil then get node.host from old sources. node.host is %v", node.Host)
			} else {
				logrus.Error("Get old node source on update node error.", err.Error())
			}
		}
		return m.delete(m.cluster.GetPrefix() + "/node/" + node.PoolName + "/" + o.GetName())
	case *object.DomainObject:
		return m.delete(m.cluster.GetPrefix() + "/domain/" + o.GetName())
	case *object.Certificate:
		return m.delete(m.cluster.GetPrefix() + "/certificate/" + o.GetName())
	default:
		return false, ErrTypeUnknown
	}
}

//AddSource 存储资源
// if  result=false and err==nil 无权操作资源
// if result=true and err==nil 操作资源成功
// if result=false and err!=nil 操作错误，需要重试或报错。
func (m *Manager) AddSource(o object.Object) (bool, error) {
	switch o.(type) {
	case *object.PoolObject:
		return m.save(m.cluster.GetPrefix()+"/pool/"+o.GetName(), o)
	case *object.VirtualServiceObject:
		return m.save(m.cluster.GetPrefix()+"/vs/"+o.GetName(), o)
	case *object.RuleObject:
		rule := o.(*object.RuleObject)
		if rule.HTTPS {
			key := fmt.Sprintf("%s/rule/https/%s/%s", m.cluster.GetPrefix(), rule.PoolName, rule.Name)
			return m.save(key, o)
		}
		key := fmt.Sprintf("%s/rule/http/%s/%s", m.cluster.GetPrefix(), rule.PoolName, rule.Name)
		return m.save(key, o)
	case *object.NodeObject:
		node := o.(*object.NodeObject)
		return m.save(m.cluster.GetPrefix()+"/node/"+node.PoolName+"/"+o.GetName(), o)
	case *object.DomainObject:
		return m.save(m.cluster.GetPrefix()+"/domain/"+o.GetName(), o)
	case *object.Certificate:
		return m.save(m.cluster.GetPrefix()+"/certificate/"+o.GetName(), o)
	default:
		return false, ErrTypeUnknown
	}
}

//UpdateSource 更新资源
// if  result=false and err==nil 无权操作资源
// if result=true and err==nil 操作资源成功
// if result=false and err!=nil 操作错误，需要重试或报错。
//返回值：是否已上线／是否有操作权／错误
func (m *Manager) UpdateSource(o object.Object) (bool, bool, error) {
	switch o.(type) {
	case *object.PoolObject:
		i := &SourceInfo{
			Data: &object.PoolObject{},
		}
		return m.update(m.cluster.GetPrefix()+"/pool/"+o.GetName(), o, i)
	case *object.VirtualServiceObject:
		i := &SourceInfo{
			Data: &object.VirtualServiceObject{},
		}
		return m.update(m.cluster.GetPrefix()+"/vs/"+o.GetName(), o, i)
	case *object.RuleObject:
		i := &SourceInfo{
			Data: &object.RuleObject{},
		}
		rule := o.(*object.RuleObject)
		if rule.HTTPS {
			key := fmt.Sprintf("%s/rule/https/%s/%s", m.cluster.GetPrefix(), rule.PoolName, rule.Name)
			return m.update(key, o, i)
		}
		key := fmt.Sprintf("%s/rule/http/%s/%s", m.cluster.GetPrefix(), rule.PoolName, rule.Name)
		return m.update(key, o, i)
	case *object.NodeObject:
		i := &SourceInfo{
			Data: &object.NodeObject{},
		}
		node := o.(*object.NodeObject)
		if len(node.Host) == 0 || node.Port == 0 {
			//更新源资源中没有host ip ,从旧数据中获取
			old, err := m.GetSource(o)
			if err == nil {
				//保证缓存数据中存在host 和port
				node.Host = old.(*object.NodeObject).Host
				node.Port = old.(*object.NodeObject).Port
				logrus.Infof("node.Host is nil then get node.host from old sources. node.host is %v", node.Host)
			} else {
				logrus.Error("Get old node source on update node error.", err.Error())
			}
		}
		return m.update(m.cluster.GetPrefix()+"/node/"+node.PoolName+"/"+o.GetName(), o, i)
	case *object.DomainObject:
		i := &SourceInfo{
			Data: &object.DomainObject{},
		}
		return m.update(m.cluster.GetPrefix()+"/domain/"+o.GetName(), o, i)
	default:
		return false, false, ErrTypeUnknown
	}
}

func (m *Manager) updateOnline(key string, i *SourceInfo) error {
	data, err := json.Marshal(i)
	if err != nil {
		return err
	}
	_, err = m.keysAPI.Update(m.ctx, key, string(data))
	if err != nil {
		return err
	}
	return nil
}

//UpdateSourceOnline 更新资源是否上线状态
func (m *Manager) UpdateSourceOnline(o object.Object, IsOnline bool) error {
	logrus.Infof("update source %s online status is %v", o.GetName(), IsOnline)
	i := &SourceInfo{
		Data:       o,
		IsOnline:   IsOnline,
		Operation:  m.cluster.GetName(),
		UpdateTime: time.Now().Format(time.RFC3339),
		Index:      o.GetIndex(),
	}
	switch o.(type) {
	case *object.PoolObject:
		return m.updateOnline(m.cluster.GetPrefix()+"/pool/"+o.GetName(), i)
	case *object.VirtualServiceObject:
		return m.updateOnline(m.cluster.GetPrefix()+"/vs/"+o.GetName(), i)
	case *object.RuleObject:
		rule := o.(*object.RuleObject)
		if rule.HTTPS {
			key := fmt.Sprintf("%s/rule/https/%s/%s", m.cluster.GetPrefix(), rule.PoolName, rule.Name)
			return m.updateOnline(key, i)
		}
		key := fmt.Sprintf("%s/rule/http/%s/%s", m.cluster.GetPrefix(), rule.PoolName, rule.Name)
		return m.updateOnline(key, i)
	case *object.NodeObject:
		node := o.(*object.NodeObject)
		return m.updateOnline(m.cluster.GetPrefix()+"/node/"+node.PoolName+"/"+o.GetName(), i)
	case *object.DomainObject:
		return m.updateOnline(m.cluster.GetPrefix()+"/domain/"+o.GetName(), i)
	default:
		return ErrTypeUnknown
	}
}

func (m *Manager) save(key string, source object.Object) (bool, error) {
	if key == "" || source == nil {
		return false, errors.New("key or source can not nil")
	}
	dataMap := make(map[string]interface{}, 0)
	dataMap["data"] = source
	dataMap["operation"] = m.cluster.GetName()
	dataMap["update_time"] = time.Now().Format(time.RFC3339)
	dataMap["index"] = source.GetIndex()
	data, err := json.Marshal(dataMap)
	if err != nil {
		return false, err
	}
	_, err = m.keysAPI.Set(m.ctx, key, string(data), &client.SetOptions{
		PrevExist: client.PrevNoExist,
	})
	if err != nil {
		if cerr, ok := err.(client.Error); ok {
			if cerr.Code == client.ErrorCodeNodeExist {
				return false, nil
			}
		}
		return false, err
	}
	return true, nil
}

//update
//返回值：是否已上线／是否有操作权／错误
func (m *Manager) update(key string, source object.Object, info *SourceInfo) (bool, bool, error) {
	if key == "" || source == nil {
		return false, false, errors.New("key or source can not nil")
	}
	i := SourceInfo{
		Data:       source,
		Operation:  m.cluster.GetName(),
		UpdateTime: time.Now().Format(time.RFC3339),
		Index:      source.GetIndex(),
	}

	locker := cluster.New(key+"-lock", 20, m.client)
	err := locker.Lock()
	if err != nil {
		return false, false, err
	}
	defer locker.Unlock()
	res, err := m.keysAPI.Get(m.ctx, key, &client.GetOptions{})
	if err != nil {
		if client.IsKeyNotFound(err) {
			// logrus.Debugf("key %s not exist. will new save it.", key)
			// return m.save(key, source)
			//TODO:思考
			//如果更新操作不添加，node非ready时会被删除，ready时再次更新会失败
			//需要判断时启动时的更新操作还是启动成功后的更新操作
			//更新操作如果新建，多实例工作时可能造成删除后又添加
			//解决方案：
			//更新操作永不删除etcd中的资源。只会更新负载均衡状态
			//更新操作不进行创建。
			return false, false, nil
		}
		return false, false, err
	}
	oldData := res.Node.Value
	err = json.Unmarshal([]byte(oldData), &info)
	if err != nil {
		return false, false, err
	}
	if info.Index < source.GetIndex() {
		i.IsOnline = info.IsOnline
		data, err := json.Marshal(i)
		if err != nil {
			return false, false, err
		}
		_, err = m.keysAPI.Update(m.ctx, key, string(data))
		if err != nil {
			return info.IsOnline, false, err
		}
		//判断是否是NODE资源，如果是当host:port没有变化且ready无变化时，操作权返回false
		switch info.Data.(type) {
		case *object.NodeObject:
			node := info.Data.(*object.NodeObject)
			newNode := source.(*object.NodeObject)
			oldKey := fmt.Sprintf("%s:%d %v", node.Host, node.Port, node.Ready)
			newKey := fmt.Sprintf("%s:%d %v", newNode.Host, newNode.Port, newNode.Ready)
			logrus.Debugf("oldkey is %s, newkey is %s", oldKey, newKey)
			if oldKey == newKey {
				return info.IsOnline, false, nil
			}
		}
		return info.IsOnline, true, nil
	}
	return info.IsOnline, false, nil
}

func (m *Manager) delete(key string) (bool, error) {
	if key == "" {
		return false, errors.New("key  can not nil")
	}
	_, err := m.keysAPI.Delete(m.ctx, key, &client.DeleteOptions{Recursive: true})
	if err != nil {
		if cerr, ok := err.(client.Error); ok {
			if cerr.Code == client.ErrorCodeKeyNotFound {
				return false, nil
			}
		}
		return false, err
	}
	return true, nil
}

//SourceInfo 资源信息
type SourceInfo struct {
	Data       object.Object `json:"data"`
	Index      int64         `json:"index"`
	UpdateTime string        `json:"update_time"`
	Operation  string        `json:"operation"`
	//TODO:增加此字段处理
	//此资源是否已经在负载均衡中处理成功
	//操作后更新本字段
	IsOnline bool `json:"is_handle"`
}

func (m *Manager) get(key string, i *SourceInfo) error {
	res, err := m.keysAPI.Get(m.ctx, key, &client.GetOptions{})
	if err != nil {
		return err
	}
	oldData := res.Node.Value
	err = json.Unmarshal([]byte(oldData), i)
	if err != nil {
		return err
	}
	return nil
}

//GetAllRule 获取全部rule,通过协议
// protocol could be http or https
func (m *Manager) GetAllRule(protocol string) ([]*object.RuleObject, error) {
	if protocol != "http" && protocol != "https" {
		return nil, errors.New("Protocol is not support")
	}
	res, err := m.keysAPI.Get(m.ctx, m.cluster.GetPrefix()+"/rule/"+protocol, &client.GetOptions{
		Recursive: true,
	})
	if err != nil {
		return nil, err
	}
	var rules []*object.RuleObject
	for _, node := range res.Node.Nodes {
		for _, n := range node.Nodes {
			info := SourceInfo{
				Data: &object.RuleObject{},
			}
			err = json.Unmarshal([]byte(n.Value), &info)
			if err != nil { //若一个发生错误，则不返回数据并返回错误
				return nil, err
			}
			rules = append(rules, info.Data.(*object.RuleObject))
		}
	}
	return rules, nil
}

//GetRuleByPool 获取指定pool下的所有rule
func (m *Manager) GetRuleByPool(protocol string, poolName string) ([]*object.RuleObject, error) {
	res, err := m.keysAPI.Get(m.ctx, m.cluster.GetPrefix()+"/rule/"+protocol+"/"+poolName, &client.GetOptions{
		Recursive: true,
	})
	if err != nil {
		return nil, err
	}
	var rules []*object.RuleObject
	for _, node := range res.Node.Nodes {
		info := SourceInfo{
			Data: &object.RuleObject{},
		}
		err = json.Unmarshal([]byte(node.Value), &info)
		if err != nil { //若一个发生错误，则不返回数据并返回错误
			return nil, err
		}
		rules = append(rules, info.Data.(*object.RuleObject))
	}
	return rules, nil
}

//GetNodeByPool 通过池名获取全部node
func (m *Manager) GetNodeByPool(poolName string) ([]*object.NodeObject, error) {
	var nodes []*object.NodeObject
	res, err := m.keysAPI.Get(m.ctx, m.cluster.GetPrefix()+"/node/"+poolName, &client.GetOptions{
		Recursive: true,
	})
	if err != nil {
		if client.IsKeyNotFound(err) { //如果键不存在，不返回错误
			return nodes, nil
		}
		return nil, err
	}
	for _, node := range res.Node.Nodes {
		info := SourceInfo{
			Data: &object.NodeObject{},
		}
		err = json.Unmarshal([]byte(node.Value), &info)
		if err != nil { //若一个发生错误，则不返回数据并返回错误
			return nil, err
		}
		nodes = append(nodes, info.Data.(*object.NodeObject))
	}
	return nodes, nil
}

//GetPools 获取全部pools信息，不重复
func (m *Manager) GetPools(poolNames map[string]string) ([]*object.PoolObject, error) {
	var pools []*object.PoolObject
	for poolName := range poolNames {
		i := &SourceInfo{
			Data: &object.PoolObject{},
		}
		err := m.get(m.cluster.GetPrefix()+"/pool/"+poolName, i)
		if err != nil {
			return nil, err
		}
		pools = append(pools, i.Data.(*object.PoolObject))
	}
	return pools, nil
}

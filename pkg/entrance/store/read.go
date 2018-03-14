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
	"github.com/goodrain/rainbond/pkg/entrance/core/object"
	"encoding/json"
	"fmt"

	"github.com/coreos/etcd/client"
)

//ReadStore 只读存储接口
type ReadStore interface {
	//根据协议查询所有rule
	GetAllRule(protocol string) ([]*object.RuleObject, error)
	GetNodeByPool(poolName string) ([]*object.NodeObject, error)
	GetPools(poolNames map[string]string) ([]*object.PoolObject, error)
	GetRule(rule *object.RuleObject) (*object.RuleObject, error)
	GetRuleByPool(protocol string, poolName string) ([]*object.RuleObject, error)
}

//GetAllPools 获取全部pools
func (m *Manager) GetAllPools() ([]*object.PoolObject, error) {
	var pools []*object.PoolObject
	res, err := m.keysAPI.Get(m.ctx, m.cluster.GetPrefix()+"/pool", &client.GetOptions{Recursive: true})
	if err != nil {
		return nil, err
	}
	for _, node := range res.Node.Nodes {
		oldData := node.Value
		i := &SourceInfo{
			Data: &object.PoolObject{},
		}
		err = json.Unmarshal([]byte(oldData), i)
		if err != nil {
			return nil, err
		}
		pools = append(pools, i.Data.(*object.PoolObject))
	}
	return pools, nil
}

//GetAllNodes get all node data
func (m *Manager) GetAllNodes() ([]*object.NodeObject, error) {
	var nodes []*object.NodeObject
	res, err := m.keysAPI.Get(m.ctx, m.cluster.GetPrefix()+"/node", &client.GetOptions{Recursive: true})
	if err != nil {
		return nil, err
	}
	for _, node := range res.Node.Nodes {
		for _, n := range node.Nodes {
			oldData := n.Value
			i := &SourceInfo{
				Data: &object.NodeObject{},
			}
			err = json.Unmarshal([]byte(oldData), i)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, i.Data.(*object.NodeObject))
		}
	}
	return nodes, nil
}

//GetAllVSs get all vs data
func (m *Manager) GetAllVSs() ([]*object.VirtualServiceObject, error) {
	var vs []*object.VirtualServiceObject
	res, err := m.keysAPI.Get(m.ctx, m.cluster.GetPrefix()+"/vs", &client.GetOptions{Recursive: true})
	if err != nil {
		return nil, err
	}
	for _, node := range res.Node.Nodes {
		oldData := node.Value
		i := &SourceInfo{
			Data: &object.VirtualServiceObject{},
		}
		err = json.Unmarshal([]byte(oldData), i)
		if err != nil {
			return nil, err
		}
		vs = append(vs, i.Data.(*object.VirtualServiceObject))
	}
	return vs, nil
}

//GetRule get rule by *object.RuleObject
func (m *Manager) GetRule(rule *object.RuleObject) (*object.RuleObject, error) {
	key := fmt.Sprintf("%s/rule/https/%s/%s", m.cluster.GetPrefix(), rule.PoolName, rule.Name)
	i := &SourceInfo{
		Data: &object.RuleObject{},
	}
	err := m.get(key, i)
	if err != nil {
		return nil, err
	}
	return i.Data.(*object.RuleObject), nil
}

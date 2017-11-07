
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

package core

import (
	"encoding/json"
	"fmt"
	"strings"

	conf "github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/core/store"
	"github.com/goodrain/rainbond/pkg/node/utils"

	"github.com/Sirupsen/logrus"
	client "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
)

// 结点类型分组
// 注册到 /cronsun/group/<id>
type Group struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	NodeIDs []string `json:"nids"`
}

func GetGroupById(gid string) (g *Group, err error) {
	if len(gid) == 0 {
		return
	}
	resp, err := store.DefalutClient.Get(conf.Config.Group + gid)
	if err != nil || resp.Count == 0 {
		return
	}

	err = json.Unmarshal(resp.Kvs[0].Value, &g)
	return
}

// GetGroups 获取包含 nid 的 group
// 如果 nid 为空，则获取所有的 group
func GetGroups(nid string) (groups map[string]*Group, err error) {
	resp, err := store.DefalutClient.Get(conf.Config.Group, client.WithPrefix())
	if err != nil {
		return
	}

	count := len(resp.Kvs)
	groups = make(map[string]*Group, count)
	if count == 0 {
		return
	}

	for _, g := range resp.Kvs {
		group := new(Group)
		if e := json.Unmarshal(g.Value, group); e != nil {
			logrus.Warnf("group[%s] umarshal err: %s", string(g.Key), e.Error())
			continue
		}
		if len(nid) == 0 || group.Included(nid) {
			groups[group.ID] = group
		}
	}
	return
}

func WatchGroups() client.WatchChan {
	return store.DefalutClient.Watch(conf.Config.Group, client.WithPrefix(), client.WithPrevKV())
}

func GetGroupFromKv(kv *mvccpb.KeyValue) (g *Group, err error) {
	g = new(Group)
	if err = json.Unmarshal(kv.Value, g); err != nil {
		err = fmt.Errorf("group[%s] umarshal err: %s", string(kv.Key), err.Error())
	}
	return
}

func DeleteGroupById(id string) (*client.DeleteResponse, error) {
	return store.DefalutClient.Delete(GroupKey(id))
}

func GroupKey(id string) string {
	return conf.Config.Group + id
}

func (g *Group) Key() string {
	return GroupKey(g.ID)
}

func (g *Group) Put(modRev int64) (*client.PutResponse, error) {
	b, err := json.Marshal(g)
	if err != nil {
		return nil, err
	}

	return store.DefalutClient.PutWithModRev(g.Key(), string(b), modRev)
}

func (g *Group) Check() error {
	g.ID = strings.TrimSpace(g.ID)
	if !store.IsValidAsKeyPath(g.ID) {
		return utils.ErrIllegalNodeGroupId
	}

	g.Name = strings.TrimSpace(g.Name)
	if len(g.Name) == 0 {
		return utils.ErrEmptyNodeGroupName
	}

	return nil
}

func (g *Group) Included(nid string) bool {
	for i, count := 0, len(g.NodeIDs); i < count; i++ {
		if nid == g.NodeIDs[i] {
			return true
		}
	}

	return false
}

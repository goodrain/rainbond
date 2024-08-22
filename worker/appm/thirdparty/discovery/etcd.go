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

// 该文件实现了基于Etcd的服务发现机制。Etcd是一种分布式键值存储系统，广泛用于服务注册与发现。
// 通过Etcd，Rainbond平台能够实时获取第三方服务的端点信息，监控这些服务的状态变化，并相应地做出处理。

// 文件中的主要功能包括：
// 1. `etcd` 结构体：封装了Etcd客户端以及相关配置信息，负责与Etcd服务器进行通信。
//    该结构体包括了服务ID、Etcd服务器端点、认证信息、监听通道和存储的服务端点记录等。
// 2. `NewEtcd` 方法：根据传入的配置创建并返回一个Etcd类型的服务发现器实例。
//    该实例能够根据配置信息连接到指定的Etcd服务器，并开始服务发现操作。
// 3. `Connect` 方法：负责与Etcd服务器建立连接。它使用Etcd客户端库中的配置选项，连接到指定的Etcd服务器。
// 4. `Fetch` 方法：从Etcd中获取存储的服务端点信息，并将其解析为Rainbond平台使用的 `RbdEndpoint` 结构体。
//    这些端点信息包括服务的IP地址、端口号等重要信息。
// 5. `Close` 方法：关闭与Etcd服务器的连接，释放相关资源。
// 6. `Watch` 方法：持续监控Etcd中服务端点的变化。当有新的服务注册、更新或删除时，能够及时捕捉到变化并进行处理。
//    该方法通过Etcd的Watch机制实现，确保服务状态的变化能够被实时感知，并通过通道通知其他组件。

// 总的来说，该文件通过实现基于Etcd的服务发现机制，为Rainbond平台提供了一个可靠的方式来监控第三方服务的状态。
// 这对于确保平台的高可用性和服务的稳定性至关重要，能够让平台在服务状态发生变化时及时做出响应。

package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	c "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/db/model"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
)

type etcd struct {
	cli     *c.Client
	version int64

	sid       string
	endpoints []string
	key       string
	username  string
	password  string

	updateCh *channels.RingChannel
	stopCh   chan struct{}
	records  map[string]*v1.RbdEndpoint
}

// NewEtcd creates a new Discorvery which implemeted by etcd.
func NewEtcd(cfg *model.ThirdPartySvcDiscoveryCfg,
	updateCh *channels.RingChannel,
	stopCh chan struct{}) Discoverier {
	// TODO: validate endpoints
	return &etcd{
		sid:       cfg.ServiceID,
		endpoints: strings.Split(cfg.Servers, ","),
		key:       cfg.Key,
		username:  cfg.Username,
		password:  cfg.Password,
		updateCh:  updateCh,
		stopCh:    stopCh,
		records:   make(map[string]*v1.RbdEndpoint),
	}
}

// Connect connects a etcdv3 client with a given configuration.
func (e *etcd) Connect() error {
	cli, err := c.New(c.Config{
		Endpoints:   e.endpoints,
		DialTimeout: 5 * time.Second,
		Username:    e.username,
		Password:    e.password,
	})
	if err != nil {
		return fmt.Errorf("error connecting etcd: %v", err)
	}
	e.cli = cli
	return nil
}

// Fetch fetches data from Etcd.
func (e *etcd) Fetch() ([]*v1.RbdEndpoint, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if e.cli == nil {
		return nil, fmt.Errorf("can't fetching data from etcd without etcdv3 client")
	}

	resp, err := e.cli.Get(ctx, e.key, c.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("error fetching endpoints form etcd: %v", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("error fetching endpoints form etcd: empty GetResponse")
	}

	type ep struct {
		IP   string `json:"ip"`
		Port int    `json:"port"`
	}
	var res []*v1.RbdEndpoint
	for _, kv := range resp.Kvs {
		var ep ep
		if err := json.Unmarshal(kv.Value, &ep); err != nil {
			return nil, fmt.Errorf("error getting data from etcd: %v", err)
		}
		endpoint := &v1.RbdEndpoint{
			UUID:     strings.Replace(string(kv.Key), e.key+"/", "", -1),
			IP:       ep.IP,
			Port:     ep.Port,
			Sid:      e.sid,
			IsOnline: true,
		}
		ip := net.ParseIP(ep.IP)
		if ip == nil {
			// domain endpoint
			res = []*v1.RbdEndpoint{endpoint}
			e.records = make(map[string]*v1.RbdEndpoint)
			e.records[string(kv.Key)] = endpoint
			break
		}
		res = append(res, endpoint)
		e.records[string(kv.Key)] = endpoint
	}
	if resp.Header != nil {
		e.version = resp.Header.GetRevision()
	}
	return res, nil
}

// Close shuts down the client's etcd connections.
func (e *etcd) Close() error {
	if e.cli != nil {
		return nil
	}
	return e.cli.Close()
}

func (e *etcd) Watch() { // todo: separate stop
	logrus.Infof("Start watching third-party endpoints. Watch key: %s", e.key)
	ctx, cancel := context.WithCancel(context.Background())
	watch := e.cli.Watch(ctx, e.key, c.WithPrefix(), c.WithRev(e.version))
	for {
		select {
		case <-e.stopCh:
			cancel()
			return
		case watResp := <-watch:
			if err := watResp.Err(); err != nil {
				logrus.Errorf("error watching event from etcd: %v", err)
				continue
			}
			logrus.Infof("Received watch response: %+v", watResp)
			for _, event := range watResp.Events {
				switch event.Type {
				case mvccpb.DELETE:
					obj := &v1.RbdEndpoint{
						UUID: strings.Replace(string(event.Kv.Key), e.key+"/", "", -1),
						Sid:  e.sid,
					}
					ep, ok := e.records[string(event.Kv.Key)]
					if ok {
						obj.IP = ep.IP
					}
					delete(e.records, string(event.Kv.Key))
					e.updateCh.In() <- Event{
						Type: DeleteEvent,
						Obj:  obj,
					}
				case mvccpb.PUT:
					type ep struct {
						IP   string `json:"ip"`
						Port int    `json:"port"`
					}
					var foo ep
					logrus.Infof("received data: %s", string(event.Kv.Value))
					if err := json.Unmarshal(event.Kv.Value, &foo); err != nil {
						logrus.Warningf("error getting endpoints from etcd: %v", err)
						continue
					}
					obj := &v1.RbdEndpoint{
						UUID:     strings.Replace(string(event.Kv.Key), e.key+"/", "", -1),
						Sid:      e.sid,
						IP:       foo.IP,
						Port:     foo.Port,
						IsOnline: true,
					}
					endpointList, err := e.Fetch()
					if err != nil {
						logrus.Errorf("error fatch endpoints: %v", err)
						continue
					}
					for _, ep := range endpointList {
						ip := net.ParseIP(ep.IP)
						if ip == nil {
							logrus.Debugf("etcd found domain endpoints: %s", ep.IP)
							obj.IP = ep.IP
							obj.Port = ep.Port
							obj.UUID = ep.UUID
							obj.Sid = ep.Sid
							obj.IsOnline = ep.IsOnline
							break
						}
					}
					if event.IsCreate() {
						e.updateCh.In() <- Event{
							Type: CreateEvent,
							Obj:  obj,
						}
					} else {
						e.updateCh.In() <- Event{
							Type: UpdateEvent,
							Obj:  obj,
						}
					}
				}
			}
		}
	}
}

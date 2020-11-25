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

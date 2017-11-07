
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

package nodeserver

import (
	"acp_node/pkg/api/model"
	"fmt"
	"os"
	"strconv"
	"time"

	conf "github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/core"
	corenode "github.com/goodrain/rainbond/pkg/node/core/node"
	"github.com/goodrain/rainbond/pkg/node/core/store"
	"github.com/goodrain/rainbond/pkg/node/node/cron"
	"github.com/goodrain/rainbond/pkg/node/utils"

	"github.com/Sirupsen/logrus"
	client "github.com/coreos/etcd/clientv3"
)

//Config config server
type Config struct {
	EtcdEndPoints        []string
	EtcdTimeout          int
	EtcdPrefix           string
	ClusterName          string
	APIAddr              string
	K8SConfPath          string
	EventServerAddress   []string
	PrometheusMetricPath string
	TTL                  int64
}

//NodeServer node manager server
type NodeServer struct {
	*store.Client
	*model.Node
	tasks      map[string]*model.Task
	ttl        int64
	lID        client.LeaseID // lease id
	regLeaseID client.LeaseID // lease id
	done       chan struct{}
	*conf.Conf
}

func (n *NodeServer) set() error {
	resp, err := n.Client.Grant(n.ttl + 2)
	if err != nil {
		return err
	}
	if n.RunMode == "master" {
		//n.Node.Put()
		n.Node.PutMaster(client.WithLease(resp.ID))
	}
	if _, err = n.Node.Put(client.WithLease(resp.ID)); err != nil {
		return err
	}

	n.lID = resp.ID

	return nil
}

//Run 启动
func (n *NodeServer) Run() (err error) {
	go n.keepAlive()
	defer func() {
		if err != nil {
			n.Stop(nil)
		}
	}()
	if err = n.loadTasks(); err != nil {
		return
	}
	go n.watchTasks()
	return
}
func (n *NodeServer) loadTasks() (err error) {
	return nil
}
func (n *NodeServer) pass(task *model.Task) bool {
	return false
}
func (n *NodeServer) watchTasks() {
	rch := store.WatchTasks()
	for wresp := range rch {
		for _, ev := range wresp.Events {
			switch {
			case ev.IsCreate():
				task, err := store.GetTaskFromKv(ev.Kv)
				if err != nil {
					logrus.Warnf("err: %s, kv: %s", err.Error(), ev.Kv.String())
					continue
				}
				if n.pass(task) {
					n.addTask(task)
				}
			case ev.IsModify():
				task, err := store.GetTaskFromKv(ev.Kv)
				if err != nil {
					logrus.Warnf("err: %s, kv: %s", err.Error(), ev.Kv.String())
					continue
				}
				if n.pass(task) {
					n.modTask(task)
				}
			case ev.Type == client.EventTypeDelete:
				n.delTask(store.GetIDFromKey(string(ev.Kv.Key)))
			default:
				logrus.Warnf("unknown event type[%v] from job[%s]", ev.Type, string(ev.Kv.Key))
			}
		}
	}
}
func (n *NodeServer) addTask(task *model.Task) {
	return
}

func (n *NodeServer) delTask(id string) {

}

func (n *NodeServer) modTask(task *model.Task) {

}

//Stop 停止服务
func (n *NodeServer) Stop(i interface{}) {
	n.Node.Down()
	close(n.done)
	n.Node.Del()
	n.Client.Close()
	n.Cron.Stop()
}

func (n *NodeServer) keepAlive() {
	duration := time.Duration(n.ttl) * time.Second
	timer := time.NewTimer(duration)
	for {
		select {
		case <-n.done:
			return
		case <-timer.C:
			if n.lID > 0 {
				_, err := n.Client.KeepAliveOnce(n.lID)
				if err == nil {
					timer.Reset(duration)
					continue
				}

				logrus.Warnf("%s lid[%x] keepAlive err: %s, try to reset...", n.String(), n.lID, err.Error())
				n.lID = 0
			}

			if err := n.set(); err != nil {
				logrus.Warnf("%s set lid err: %s, try to reset after %d seconds...", n.String(), err.Error(), n.ttl)
			} else {
				logrus.Infof("%s set lid[%x] success", n.String(), n.lID)
			}
			timer.Reset(duration)
		}
	}
}
func (m *NodeServer) httpKey() string {
	return fmt.Sprintf("/traefik/backends/acp_node/servers/%s/url", m.ID)
}
func (m *NodeServer) httpValue() string {
	return m.ID + conf.Config.APIAddr
}

func (m *NodeServer) Reg() error {
	resp, err := m.Client.Grant(m.ttl + 2)
	if err != nil {
		return err
	}
	if _, err := m.Client.Put(
		m.httpKey(),
		m.httpValue(),
		client.WithLease(resp.ID)); err != nil {
		return err
	}
	m.regLeaseID = resp.ID
	return nil
}

func (m *NodeServer) RegKeepAlive() {
	duration := time.Duration(m.ttl) * time.Second
	timer := time.NewTimer(duration)
	for {
		select {
		case <-m.done:
			return
		case <-timer.C:
			if m.regLeaseID > 0 {
				_, err := m.Client.KeepAliveOnce(m.regLeaseID)
				if err == nil {
					timer.Reset(duration)
					continue
				}
				logrus.Warnf("%s lid[%x] keepAlive err: %s, try to reset...", m.ID, m.regLeaseID, err.Error())
				m.regLeaseID = 0
			}
			if err := m.Reg(); err != nil {
				logrus.Warnf("%s set lid err: %s, try to reset after %d seconds...", m.ID, err.Error(), m.ttl)
			} else {
				logrus.Infof("%s set lid[%x] success", m.ID, m.regLeaseID)
			}
			timer.Reset(duration)
		}
	}

}

//NewNodeServer new server
func NewNodeServer(cfg *conf.Conf) (*NodeServer, error) {
	ip, err := utils.LocalIP()
	if err != nil {
		return nil, err
	}

	n := &NodeServer{
		Client: store.DefalutClient,
		Node: &corenode.Node{
			ID:  ip.String(),
			PID: strconv.Itoa(os.Getpid()),
		},
		Cron: cron.New(),

		jobs: make(Jobs, 8),
		cmds: make(map[string]*core.Cmd),

		link:   newLink(8),
		delIDs: make(map[string]bool, 8),
		Conf:   cfg,
		ttl:    cfg.Ttl,

		done: make(chan struct{}),
	}
	return n, nil
}

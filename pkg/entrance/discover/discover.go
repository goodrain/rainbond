
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

package discover

import (
	"github.com/goodrain/rainbond/cmd/entrance/option"
	"context"
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"

	"github.com/coreos/etcd/clientv3"
)

//Manager discover manager
type Manager struct {
	IP           string
	Port         int
	RegTime      int64
	Etcdclientv3 *clientv3.Client
	Ctx          context.Context
	Cancel       func()
	LID          clientv3.LeaseID
	Done         chan struct{}
	//EventServerAddress   []string
}

//NewDiscoverManager create discover manager
func NewDiscoverManager(config option.Config) (*Manager, error) {
	etcdclientv3, err := clientv3.New(clientv3.Config{
		Endpoints: config.EtcdEndPoints,
	})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		Etcdclientv3: etcdclientv3,
		Ctx:          ctx,
		Cancel:       cancel,
		IP:           config.BindIP,
		Port:         config.BindPort,
		RegTime:      5,
	}, nil
}

//Start start
func (m *Manager) Start() {
	go m.keepAlive()
	defer m.CancelIP()
}

//CancelIP CancelIP
func (m *Manager) CancelIP() error {
	ctx, cancel := context.WithTimeout(m.Ctx, time.Second*5)
	defer cancel()
	if _, err := m.Etcdclientv3.Delete(ctx, m.httpKey()); err != nil {
		return err
	}
	logrus.Info(fmt.Sprintf("cancel host %s into etcd", fmt.Sprintf("%s:%d", m.IP, m.Port)))
	return nil
}

func (m *Manager) keepAlive() {
	duration := time.Duration(m.RegTime) * time.Second
	timer := time.NewTimer(duration)
	for {
		select {
		case <-m.Done:
			return
		case <-timer.C:
			if m.LID > 0 {
				_, err := m.Etcdclientv3.KeepAliveOnce(m.Ctx, m.LID)
				if err == nil {
					timer.Reset(duration)
					continue
				}
				logrus.Warnf("%s lid[%x] keepAlive err: %s, try to reset...", m.domain(), m.LID, err.Error())
				m.LID = 0
			}
			if err := m.set(); err != nil {
				logrus.Warnf("%s set lid err: %s, try to reset after %d seconds...", m.domain(), err.Error(), m.RegTime)
			} else {
				logrus.Infof("%s set lid[%x] success", m.domain(), m.LID)
			}
			timer.Reset(duration)
		}
	}
}

func (m *Manager) domain() string {
	return fmt.Sprintf("%s:%v", m.IP, m.Port)
}

func (m *Manager) httpKey() string {
	return fmt.Sprintf("/traefik/backends/acp_entrance/servers/%s/url", m.IP)
}

func (m *Manager) set() error {
	resp, err := m.Etcdclientv3.Grant(m.Ctx, m.RegTime+2)
	if err != nil {
		return err
	}
	if _, err := m.Etcdclientv3.Put(m.Ctx,
		m.httpKey(),
		m.domain(),
		clientv3.WithLease(resp.ID)); err != nil {
		return err
	}
	m.LID = resp.ID
	return nil
}

// RAINBOND, Application Management Platform
// Copyright (C) 2014-2019 Goodrain Co., Ltd.

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

package util

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

//IPEVENTTYPE ip change event type
type IPEVENTTYPE int

//IPEVENT ip change event
type IPEVENT struct {
	Type IPEVENTTYPE
	IP   net.IP
}

//IPPool ip pool
type IPPool struct {
	ctx                 context.Context
	cancel              context.CancelFunc
	lock                sync.Mutex
	HostIPs             map[string]net.IP
	EventCh             chan IPEVENT
	StopCh              chan struct{}
	ignoreInterfaceName []string
	startReady          chan struct{}
	once                sync.Once
}

const (
	//ADD add event
	ADD IPEVENTTYPE = iota
	//DEL del event
	DEL
	//UPDATE update event
	UPDATE
)

//NewIPPool new ip pool
func NewIPPool(ignoreInterfaceName []string) *IPPool {
	ctx, cancel := context.WithCancel(context.Background())
	ippool := &IPPool{
		ctx:                 ctx,
		cancel:              cancel,
		HostIPs:             map[string]net.IP{},
		EventCh:             make(chan IPEVENT, 1024),
		StopCh:              make(chan struct{}),
		ignoreInterfaceName: ignoreInterfaceName,
		startReady:          make(chan struct{}),
	}
	return ippool
}

//Ready ready
func (i *IPPool) Ready() bool {
	logrus.Info("waiting ip pool start ready")
	<-i.startReady
	return true
}

//GetHostIPs get host ips
func (i *IPPool) GetHostIPs() []net.IP {
	i.lock.Lock()
	defer i.lock.Unlock()
	var ips []net.IP
	for _, ip := range i.HostIPs {
		ips = append(ips, ip)
	}
	return ips
}

//GetWatchIPChan watch ip change
func (i *IPPool) GetWatchIPChan() <-chan IPEVENT {
	return i.EventCh
}

//Close close
func (i *IPPool) Close() {
	i.cancel()
}

//LoopCheckIPs loop check ips
func (i *IPPool) LoopCheckIPs() {
	Exec(i.ctx, func() error {
		logrus.Debugf("start loop watch ips from all interface")
		ips, err := i.getInterfaceIPs()
		if err != nil {
			logrus.Errorf("get ip address from interface failure %s, will retry", err.Error())
			return nil
		}
		i.lock.Lock()
		defer i.lock.Unlock()
		var newIP = make(map[string]net.IP)
		for _, v := range ips {
			if v.To4() == nil {
				continue
			}
			_, ok := i.HostIPs[v.To4().String()]
			if ok {
				i.EventCh <- IPEVENT{Type: UPDATE, IP: v.To4()}
			}
			if !ok {
				i.EventCh <- IPEVENT{Type: ADD, IP: v.To4()}
			}
			newIP[v.To4().String()] = v.To4()
		}
		for k, v := range i.HostIPs {
			if _, ok := newIP[k]; !ok {
				i.EventCh <- IPEVENT{Type: DEL, IP: v.To4()}
			}
		}
		logrus.Debugf("loop watch ips from all interface, find %d ips", len(newIP))
		i.HostIPs = newIP
		i.once.Do(func() {
			close(i.startReady)
		})
		return nil
	}, time.Second*5)
}

func (i *IPPool) getInterfaceIPs() ([]net.IP, error) {
	var ips []net.IP
	tables, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, table := range tables {
		if StringArrayContains(i.ignoreInterfaceName, table.Name) {
			logrus.Debugf("ip address from interface %s ignore", table.Name)
			continue
		}
		addrs, err := table.Addrs()
		if err != nil {
			return nil, err
		}
		for _, address := range addrs {
			if ipnet := checkIPAddress(address); ipnet != nil {
				if ipnet.IP.To4() != nil {
					ips = append(ips, ipnet.IP.To4())
				}
			}
		}
	}
	return ips, nil
}

func checkIPAddress(addr net.Addr) *net.IPNet {
	ipnet, ok := addr.(*net.IPNet)
	if !ok {
		return nil
	}
	if ipnet.IP.IsLoopback() {
		return nil
	}
	return ipnet
}

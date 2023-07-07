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

package monitormessage

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/goodrain/rainbond/discover"
	"github.com/goodrain/rainbond/discover/config"
	etcdutil "github.com/goodrain/rainbond/util/etcd"
	"github.com/sirupsen/logrus"

	"github.com/prometheus/common/log"
)

//UDPServer udp server
type UDPServer struct {
	ctx                 context.Context
	ListenerHost        string
	ListenerPort        int
	eventServerEndpoint []string
	client              net.Conn
	etcdClientArgs      *etcdutil.ClientArgs
}

//CreateUDPServer create udpserver
func CreateUDPServer(ctx context.Context, lisHost string, lisPort int, etcdClientArgs *etcdutil.ClientArgs) *UDPServer {
	return &UDPServer{
		ctx:            ctx,
		ListenerHost:   lisHost,
		ListenerPort:   lisPort,
		etcdClientArgs: etcdClientArgs,
	}
}

//Start start
func (u *UDPServer) Start() error {
	dis, err := discover.GetDiscover(config.DiscoverConfig{Ctx: u.ctx, EtcdClientArgs: u.etcdClientArgs})
	if err != nil {
		return err
	}
	dis.AddProject("event_log_event_udp", u)
	if err := u.server(); err != nil {
		return err
	}
	return nil
}

//UpdateEndpoints update event server address
func (u *UDPServer) UpdateEndpoints(endpoints ...*config.Endpoint) {
	var eventServerEndpoint []string
	for _, e := range endpoints {
		eventServerEndpoint = append(eventServerEndpoint, e.URL)
		u.eventServerEndpoint = eventServerEndpoint
	}
	if len(u.eventServerEndpoint) > 0 {
		for i := range u.eventServerEndpoint {
			info := strings.Split(u.eventServerEndpoint[i], ":")
			if len(info) == 2 {
				dip := net.ParseIP(info[0])
				port, err := strconv.Atoi(info[1])
				if err != nil {
					continue
				}
				srcAddr := &net.UDPAddr{IP: net.IPv4zero, Port: 0}
				dstAddr := &net.UDPAddr{IP: dip, Port: port}
				conn, err := net.DialUDP("udp", srcAddr, dstAddr)
				if err != nil {
					logrus.Error(err)
					continue
				}
				logrus.Debugf("Update event server address is %s", u.eventServerEndpoint[i])
				u.client = conn
				break
			}
		}

	}
}

//Error
func (u *UDPServer) Error(err error) {

}

//Server 服务
func (u *UDPServer) server() error {
	listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(u.ListenerHost), Port: u.ListenerPort})
	if err != nil {
		fmt.Println(err)
		return err
	}
	log.Infof("UDP Server Listener: %s", listener.LocalAddr().String())
	buf := make([]byte, 65535)
	go func() {
		defer listener.Close()
		for {
			n, _, err := listener.ReadFromUDP(buf)
			if err != nil {
				logrus.Errorf("read message from udp error,%s", err.Error())
				time.Sleep(time.Second * 2)
				continue
			}
			u.handlePacket(buf[0:n])
		}
	}()
	return nil
}

func (u *UDPServer) handlePacket(packet []byte) {
	lines := strings.Split(string(packet), "\n")
	for _, line := range lines {
		if line != "" && u.client != nil {
			u.client.Write([]byte(line))
		}
	}
}

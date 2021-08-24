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

	"github.com/sirupsen/logrus"
)

//UDPServer udp server
type UDPServer struct {
	ctx             context.Context
	ListenerHost    string
	ListenerPort    int
	client          net.Conn
	eventServerAddr string
}

//CreateUDPServer create udpserver
func CreateUDPServer(ctx context.Context, lisHost string, lisPort int) *UDPServer {
	return &UDPServer{
		ctx:             ctx,
		ListenerHost:    lisHost,
		ListenerPort:    lisPort,
		eventServerAddr: "rbd-eventlog.rbd-system:6166",
	}
}

//Start start
func (u *UDPServer) Start() error {
	if err := u.server(); err != nil {
		return err
	}
	return nil
}

//Server 服务
func (u *UDPServer) server() error {
	listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(u.ListenerHost), Port: u.ListenerPort})
	if err != nil {
		fmt.Println(err)
		return err
	}
	logrus.Infof("UDP Server Listener: %s", listener.LocalAddr().String())
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
	if u.client == nil {
		domain := u.eventServerAddr
		port := 6166
		if strings.Contains(domain, ":") {
			infos := strings.Split(u.eventServerAddr, ":")
			domain = infos[0]
			port, _ = strconv.Atoi(infos[1])
		}
		addr, err := net.ResolveIPAddr("ip", domain)
		if err != nil {
			logrus.Errorf("resolve event server domain %s failure %s", domain, err.Error())
			return
		}
		srcAddr := &net.UDPAddr{IP: net.IPv4zero, Port: 0}
		dstAddr := &net.UDPAddr{IP: addr.IP, Port: port}
		conn, err := net.DialUDP("udp", srcAddr, dstAddr)
		if err != nil {
			logrus.Error("connect event udp server failure %s", err.Error())
			return
		}
		u.client = conn
		logrus.Infof("connect event udp server %s:%d", addr.IP, port)
	}
	lines := strings.Split(string(packet), "\n")
	for _, line := range lines {
		if line != "" && u.client != nil {
			_, err := u.client.Write([]byte(line))
			if err != nil {
				logrus.Errorf("write udp message to event server failure %s", err.Error())
				u.client.Close()
				u.client = nil
				return
			}
		}
	}
}

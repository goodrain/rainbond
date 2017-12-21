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

package monitormessage

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"

	"github.com/prometheus/common/log"
)

//UDPServer udp server
type UDPServer struct {
	ListenerHost string
	ListenerPort int
}

//CreateUDPServer create udpserver
func CreateUDPServer(lisHost string, lisPort int) *UDPServer {
	return &UDPServer{
		ListenerHost: lisHost,
		ListenerPort: lisPort,
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
		if line != "" {
			fmt.Println("Message:" + line)
		}
	}
}

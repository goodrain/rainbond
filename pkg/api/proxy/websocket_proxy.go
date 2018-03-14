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

package proxy

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
)

//WebSocketProxy WebSocketProxy
type WebSocketProxy struct {
	name      string
	endpoints EndpointList
	lb        LoadBalance
}

//Proxy 代理
func (h *WebSocketProxy) Proxy(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		EnableCompression: true,
		Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {
			w.WriteHeader(500)
		},
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logrus.Error("Create web socket conn error.", err.Error())
		return
	}
	defer conn.Close()

	endpoint := h.lb.Select(r, h.endpoints)
	path := r.RequestURI
	if strings.Contains(path, "?") {
		path = path[:strings.Index(path, "?")]
	}
	u := url.URL{Scheme: "ws", Host: endpoint.String(), Path: path}

	logrus.Infof("connecting to %s", u.String())
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		logrus.Errorf("dial websocket endpoint %s error. %s", u.String(), err.Error())
		w.WriteHeader(500)
		return
	}
	defer c.Close()
	done := make(chan struct{})
	go func() {
		defer c.Close()
		defer close(done)
		for {
			select {
			case <-r.Context().Done():
				return
			default:
			}
			t, message, err := c.ReadMessage()
			if err != nil {
				logrus.Println("read proxy websocket message error: ", err)
				return
			}
			err = conn.WriteMessage(t, message)
			if err != nil {
				logrus.Println("write client websocket message error: ", err)
				return
			}
		}
	}()
	for {
		select {
		case <-r.Context().Done():
			// To cleanly close a connection, a client should send a close
			// frame and wait for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			c.Close()
			return
		case <-done:
			return
		default:
		}
		t, message, err := conn.ReadMessage()
		if err != nil {
			logrus.Errorln("read client websocket message error: ", err)
			return
		}
		err = c.WriteMessage(t, message)
		if err != nil {
			logrus.Errorln("write proxy websocket message error: ", err)
			return
		}
	}
}

//UpdateEndpoints 更新后端点
func (h *WebSocketProxy) UpdateEndpoints(endpoints ...string) {
	h.endpoints = CreateEndpoints(endpoints)
}

//Do do proxy
func (h *WebSocketProxy) Do(r *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("do not support")
}

func createWebSocketProxy(name string, endpoints []string) *WebSocketProxy {
	if name != "dockerlog" {
		return &WebSocketProxy{name, CreateEndpoints(endpoints), NewRoundRobin()}
	}
	return &WebSocketProxy{name, CreateEndpoints(endpoints), NewSelectBalance()}

}

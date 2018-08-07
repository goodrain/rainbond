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

package proxy

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
)

//WebSocketProxy WebSocketProxy
type WebSocketProxy struct {
	name      string
	endpoints EndpointList
	lb        LoadBalance
	upgrader  *websocket.Upgrader
}

//Proxy 代理
// func (h *WebSocketProxy) Proxy(w http.ResponseWriter, r *http.Request) {
// 	upgrader := websocket.Upgrader{
// 		EnableCompression: true,
// 		Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {
// 			w.WriteHeader(500)
// 		},
// 		CheckOrigin: func(r *http.Request) bool {
// 			return true
// 		},
// 	}
// 	conn, err := upgrader.Upgrade(w, r, nil)
// 	if err != nil {
// 		logrus.Error("Create web socket conn error.", err.Error())
// 		return
// 	}
// 	defer func() {
// 		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
// 		conn.Close()
// 	}()
// 	endpoint := h.lb.Select(r, h.endpoints)
// 	path := r.RequestURI
// 	if strings.Contains(path, "?") {
// 		path = path[:strings.Index(path, "?")]
// 	}
// 	u := url.URL{Scheme: "ws", Host: endpoint.String(), Path: path}
// 	logrus.Infof("connecting to %s", u.String())
// 	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
// 	if err != nil {
// 		logrus.Errorf("dial websocket endpoint %s error. %s", u.String(), err.Error())
// 		w.WriteHeader(500)
// 		return
// 	}
// 	defer c.Close()
// 	done := make(chan struct{})
// 	go func() {
// 		defer close(done)
// 		for {
// 			select {
// 			case <-r.Context().Done():
// 				return
// 			default:
// 			}
// 			t, message, err := c.ReadMessage()
// 			if err != nil {
// 				logrus.Println("read proxy websocket message error: ", err)
// 				return
// 			}
// 			err = conn.WriteMessage(t, message)
// 			if err != nil {
// 				logrus.Println("write client websocket message error: ", err)
// 				return
// 			}
// 		}
// 	}()
// 	for {
// 		select {
// 		case <-r.Context().Done():
// 			// To cleanly close a connection, a client should send a close
// 			// frame and wait for the server to close the connection.
// 			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
// 			if err != nil {
// 				log.Println("write close:", err)
// 				return
// 			}
// 			select {
// 			case <-done:
// 			case <-time.After(time.Second):
// 			}
// 			return
// 		case <-done:
// 			return
// 		default:
// 		}
// 		t, message, err := conn.ReadMessage()
// 		if err != nil {
// 			logrus.Errorln("read client websocket message error: ", err)
// 			return
// 		}
// 		err = c.WriteMessage(t, message)
// 		if err != nil {
// 			logrus.Errorln("write proxy websocket message error: ", err)
// 			return
// 		}
// 	}
// }

func (h *WebSocketProxy) Proxy(w http.ResponseWriter, req *http.Request) {
	endpoint := h.lb.Select(req, h.endpoints)
	logrus.Info("Proxy webSocket to: ", endpoint)
	path := req.RequestURI
	if strings.Contains(path, "?") {
		path = path[:strings.Index(path, "?")]
	}
	u := url.URL{Scheme: "ws", Host: endpoint.GetAddr(), Path: path}
	logrus.Infof("connecting to %s", u.String())
	// Pass headers from the incoming request to the dialer to forward them to
	// the final destinations.
	requestHeader := http.Header{}
	if origin := req.Header.Get("Origin"); origin != "" {
		requestHeader.Add("Origin", origin)
	}
	for _, prot := range req.Header[http.CanonicalHeaderKey("Sec-WebSocket-Protocol")] {
		requestHeader.Add("Sec-WebSocket-Protocol", prot)
	}
	for _, cookie := range req.Header[http.CanonicalHeaderKey("Cookie")] {
		requestHeader.Add("Cookie", cookie)
	}

	// Pass X-Forwarded-For headers too, code below is a part of
	// httputil.ReverseProxy. See http://en.wikipedia.org/wiki/X-Forwarded-For
	// for more information
	// TODO: use RFC7239 http://tools.ietf.org/html/rfc7239
	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		// If we aren't the first proxy retain prior
		// X-Forwarded-For information as a comma+space
		// separated list and fold multiple headers into one.
		if prior, ok := req.Header["X-Forwarded-For"]; ok {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		requestHeader.Set("X-Forwarded-For", clientIP)
	}
	// Connect to the backend URL, also pass the headers we get from the requst
	// together with the Forwarded headers we prepared above.
	// TODO: support multiplexing on the same backend connection instead of
	// opening a new TCP connection time for each request. This should be
	// optional:
	// http://tools.ietf.org/html/draft-ietf-hybi-websocket-multiplexing-01
	connBackend, resp, err := websocket.DefaultDialer.Dial(u.String(), requestHeader)
	if err != nil {
		log.Printf("websocketproxy: couldn't dial to remote backend url %s\n", err)
		return
	}
	defer connBackend.Close()

	// Only pass those headers to the upgrader.
	upgradeHeader := http.Header{}
	if hdr := resp.Header.Get("Sec-Websocket-Protocol"); hdr != "" {
		upgradeHeader.Set("Sec-Websocket-Protocol", hdr)
	}
	if hdr := resp.Header.Get("Set-Cookie"); hdr != "" {
		upgradeHeader.Set("Set-Cookie", hdr)
	}
	if h.upgrader == nil {
		h.upgrader = &websocket.Upgrader{
			ReadBufferSize:    1024,
			WriteBufferSize:   1024,
			EnableCompression: true,
			Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {
				w.WriteHeader(500)
			},
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		}
	}
	// Now upgrade the existing incoming request to a WebSocket connection.
	// Also pass the header that we gathered from the Dial handshake.
	connPub, err := h.upgrader.Upgrade(w, req, upgradeHeader)
	if err != nil {
		log.Printf("websocketproxy: couldn't upgrade %s\n", err)
		return
	}
	defer connPub.Close()
	errClient := make(chan error, 1)
	errBackend := make(chan error, 1)
	replicateWebsocketConn := func(dst, src *websocket.Conn, errc chan error) {
		for {
			msgType, msg, err := src.ReadMessage()
			if err != nil {
				m := websocket.FormatCloseMessage(websocket.CloseNormalClosure, fmt.Sprintf("%v", err))
				if e, ok := err.(*websocket.CloseError); ok {
					if e.Code != websocket.CloseNoStatusReceived {
						m = websocket.FormatCloseMessage(e.Code, e.Text)
					}
				}
				errc <- err
				dst.WriteMessage(websocket.CloseMessage, m)
				break
			}
			err = dst.WriteMessage(msgType, msg)
			if err != nil {
				errc <- err
				break
			}
		}
	}
	go replicateWebsocketConn(connPub, connBackend, errClient)
	go replicateWebsocketConn(connBackend, connPub, errBackend)
	var message string
	select {
	case err = <-errClient:
		message = "websocketproxy: Error when copying from backend to client: %v"
	case err = <-errBackend:
		message = "websocketproxy: Error when copying from client to backend: %v"
	}
	if e, ok := err.(*websocket.CloseError); !ok || e.Code == websocket.CloseAbnormalClosure {
		logrus.Errorf(message, err)
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
		return &WebSocketProxy{
			name:      name,
			endpoints: CreateEndpoints(endpoints),
			lb:        NewRoundRobin(),
		}
	}
	return &WebSocketProxy{
		name:      name,
		endpoints: CreateEndpoints(endpoints),
		lb:        NewSelectBalance(),
	}

}

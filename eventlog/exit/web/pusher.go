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

package web

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/goodrain/rainbond/eventlog/db"

	"github.com/goodrain/rainbond/util"

	"github.com/pquerna/ffjson/ffjson"

	"github.com/gorilla/websocket"
)

//WebsocketMessage websocket message
type WebsocketMessage struct {
	Event   string      `json:"event"`
	Data    interface{} `json:"data"`
	Channel string      `json:"channel,omitempty"`
}

//Encode return json encode data
func (w *WebsocketMessage) Encode() []byte {
	reb, _ := ffjson.Marshal(w)
	return reb
}

//PubContext websocket context
type PubContext struct {
	ID          string
	upgrader    websocket.Upgrader
	conn        *websocket.Conn
	httpWriter  http.ResponseWriter
	httpRequest *http.Request
	server      *SocketServer
	chans       map[string]*Chan
	lock        sync.Mutex
	close       chan struct{}
}

//Chan handle
type Chan struct {
	ch      chan *db.EventLogMessage
	id      string
	chtype  string
	reevent string
	channel string
	p       *PubContext
}

//NewPubContext create context
func NewPubContext(upgrader websocket.Upgrader,
	httpWriter http.ResponseWriter,
	httpRequest *http.Request,
	s *SocketServer,
) *PubContext {
	return &PubContext{
		ID:          util.NewUUID(),
		upgrader:    upgrader,
		httpWriter:  httpWriter,
		httpRequest: httpRequest,
		server:      s,
		chans:       make(map[string]*Chan, 2),
		close:       make(chan struct{}),
	}
}
func (p *PubContext) handleMessage(me []byte) {
	var wm WebsocketMessage
	if err := ffjson.Unmarshal(me, &wm); err != nil {
		p.SendMessage(WebsocketMessage{Event: "error", Data: "Invalid message"})
		return
	}
	switch wm.Event {
	case "pusher:subscribe":
		p.handleSubscribe(wm)
	}
}
func (p *PubContext) createChan(channel, chantype, id string) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if _, exist := p.chans[chantype+"-"+id]; exist {
		return
	}
	ch := p.server.storemanager.WebSocketMessageChan(chantype, id, p.ID)
	if ch != nil {
		c := &Chan{
			ch:      ch,
			channel: chantype + "-" + id,
			id:      id,
			chtype:  chantype,
			reevent: func() string {
				if chantype == "docker" {
					return "service:log"
				}
				if chantype == "newmonitor" {
					return "monitor"
				}
				if chantype == "event" {
					return "event:log"
				}
				return ""
			}(),
			p: p,
		}
		p.chans[chantype+"-"+id] = c
		// send success message
		p.SendMessage(WebsocketMessage{
			Event:   "pusher:succeeded",
			Channel: c.channel,
		})
		go c.handleChan()
	}
}
func (p *PubContext) removeChan(key string) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if _, exist := p.chans[key]; exist {
		delete(p.chans, key)
	}
	if len(p.chans) == 0 {
		p.Close()
	}
}
func (p *PubContext) handleSubscribe(wm WebsocketMessage) {
	data := wm.Data.(map[string]interface{})
	if channel, ok := data["channel"].(string); ok {
		channelInfo := strings.SplitN(channel, "-", 2)
		if len(channelInfo) < 2 {
			p.SendMessage(WebsocketMessage{Event: "error", Data: "Invalid message"})
			return
		}
		if channelInfo[0] == "s" {
			p.createChan(channel, "docker", channelInfo[1])
			p.createChan(channel, "newmonitor", channelInfo[1])
		}
		if channelInfo[0] == "e" {
			p.createChan(channel, "newmonitor", channelInfo[1])
		}
	}
}

func (c *Chan) handleChan() {
	defer func() {
		c.p.server.log.Infof("pubsub message chan %s closed", c.channel)
		c.p.removeChan(c.chtype + "-" + c.id)
	}()
	for {
		select {
		case <-c.p.close:
			c.p.server.log.Info("pub sub context closed")
			return
		case <-c.p.httpRequest.Context().Done():
			c.p.server.log.Info("pub sub request context cancel")
			return
		case message, ok := <-c.ch:
			if !ok {
				c.p.SendMessage(WebsocketMessage{Event: "pusher:close", Data: "{}", Channel: c.channel})
				c.p.SendWebsocketMessage(websocket.CloseMessage)
				return
			}
			if message != nil {
				if message.Step == "last" {
					c.p.SendMessage(WebsocketMessage{Event: "event:success", Data: message.Message, Channel: c.channel})
					c.p.SendMessage(WebsocketMessage{Event: "pusher:close", Data: message.Message, Channel: c.channel})
					return
				}
				if message.Step == "callback" {
					c.p.SendMessage(WebsocketMessage{Event: "event:failure", Data: message.Message, Channel: c.channel})
					c.p.SendMessage(WebsocketMessage{Event: "pusher:close", Data: message.Message, Channel: c.channel})
					return
				}
				if message.MonitorData != nil {
					c.p.SendMessage(WebsocketMessage{Event: c.reevent, Data: string(message.MonitorData), Channel: c.channel})
				} else {
					c.p.SendMessage(WebsocketMessage{Event: c.reevent, Data: string(message.Content), Channel: c.channel})
				}
			}
		}
	}
}

func (p *PubContext) readMessage(closed chan struct{}) {
	defer func() {
		close(closed)
	}()
	if p.conn == nil {
		p.server.log.Errorf("websocket connection is not connect")
	}
	for {
		messageType, me, err := p.conn.ReadMessage()
		if err != nil {
			p.server.log.Errorf("read websocket message failure %s", err.Error())
			break
		}
		if messageType == websocket.CloseMessage {
			break
		}
		if messageType == websocket.TextMessage {
			p.handleMessage(me)
			continue
		}
		if messageType == websocket.PingMessage {
			p.conn.WriteMessage(websocket.PongMessage, []byte{})
			continue
		}
		if messageType == websocket.BinaryMessage {
			continue
		}
	}
}

//SendMessage send websocket message
func (p *PubContext) SendMessage(message WebsocketMessage) error {
	p.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return p.conn.WriteMessage(websocket.TextMessage, message.Encode())
}

//SendWebsocketMessage send websocket message
func (p *PubContext) SendWebsocketMessage(message int) error {
	p.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return p.conn.WriteMessage(message, []byte{})
}

func (p *PubContext) sendPing(closed chan struct{}) {
	err := util.Exec(p.httpRequest.Context(), func() error {
		if err := p.SendWebsocketMessage(websocket.PingMessage); err != nil {
			return err
		}
		return nil
	}, time.Second*20)
	if err != nil {
		p.server.log.Errorf("send ping message failure %s will closed the connect", err.Error())
		close(closed)
	}
}

//Start start context
func (p *PubContext) Start() {
	var err error
	p.conn, err = p.upgrader.Upgrade(p.httpWriter, p.httpRequest, nil)
	if err != nil {
		p.server.log.Error("Create web socket conn error.", err.Error())
		return
	}
	pingclosed := make(chan struct{})
	readclosed := make(chan struct{})
	go p.sendPing(pingclosed)
	go p.readMessage(readclosed)
	select {
	case <-pingclosed:
	case <-readclosed:
	case <-p.close:
	}
}

//Stop close context
func (p *PubContext) Stop() {
	if p.conn != nil {
		p.conn.Close()
	}
	for _, v := range p.chans {
		p.server.storemanager.RealseWebSocketMessageChan(v.chtype, v.id, p.ID)
	}
}

//Close close the context
func (p *PubContext) Close() {
	close(p.close)
}

func (s *SocketServer) pubsub(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:    s.conf.ReadBufferSize,
		WriteBufferSize:   s.conf.WriteBufferSize,
		EnableCompression: s.conf.EnableCompression,
		Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {

		},
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	context := NewPubContext(upgrader, w, r, s)
	defer context.Stop()
	s.log.Infof("websocket pubsub context running %s", context.ID)
	context.Start()
	s.log.Infof("websocket pubsub context closed %s", context.ID)
}

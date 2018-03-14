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

package web

import (
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"fmt"

	"github.com/gorilla/websocket"
)

func TestWebSocket(t *testing.T) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	u := url.URL{Scheme: "ws", Host: "127.0.0.1:1233", Path: "/event_log"}
	log.Printf("connecting to %s", u.String())
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatal("dial:", err)
	}
	defer c.Close()
	err = c.WriteMessage(websocket.TextMessage, []byte("event_id=qwertyuiuiosadfkbjasdv"))
	if err != nil {
		t.Fatal(err)
		return
	}
	_, message, err := c.ReadMessage()
	if err != nil {
		t.Log("read:", err)
		return
	}
	if string(message) != "ok" {
		t.Fatal("请求失败")
		return
	}
	done := make(chan struct{})

	defer c.Close()
	defer close(done)

	go func() {
		for {
			select {
			case <-interrupt:
				log.Println("interrupt")
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
			}
		}
	}()

	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			t.Log("read:", err)
			return
		}
		fmt.Printf("recv: %s \n", message)
	}
}

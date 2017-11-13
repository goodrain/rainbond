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

package cmd
import (
	"github.com/urfave/cli"
	"github.com/Sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/gorilla/websocket"
	"fmt"
	"net/url"
	"strings"

	"github.com/goodrain/rainbond/pkg/grctl/clients"
	"encoding/json"

)

func NewCmdEvent() cli.Command {
	c:=cli.Command{
		Name: "event",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "f",
				Usage: "添加此参数日志持续输出。",
			},
			cli.StringFlag{
				Name:  "event_log_server",
				Usage: "event log server address",
			},
		},
		Usage: "获取某个操作的日志 grctl event eventID 123/gr2a2e1b ",
		Action: func(c *cli.Context) error {
			Common(c)
			return getEventLog(c)
		},
	}
	return c
}

func GetEventLogf(eventID ,server string) {

	//if c.String("event_log_server") != "" {
	//	server = c.String("event_log_server")
	//}
	u := url.URL{Scheme: "ws", Host: server, Path: "event_log"}
	logrus.Infof("connecting to %s", u.String())
	con, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		logrus.Errorf("dial websocket endpoint %s error. %s", u.String(), err.Error())
		//return err
	}
	defer con.Close()

	con.WriteMessage(websocket.TextMessage, []byte("event_id="+eventID))
	defer con.Close()
	for {
		_, message, err := con.ReadMessage()
		if err != nil {
			logrus.Println("read proxy websocket message error: ", err)
			return
		}
		time := gjson.GetBytes(message, "time").String()
		m := gjson.GetBytes(message, "message").String()
		level := gjson.GetBytes(message, "level").String()
		fmt.Printf("[%s](%s) %s \n", strings.ToUpper(level), time, m)
	}
}
func getEventLog(c *cli.Context) error {
	eventID := c.Args().First()
	if c.Bool("f") {

		server := "127.0.0.1:6363"
		if c.String("event_log_server") != "" {
			server = c.String("event_log_server")
		}
		u := url.URL{Scheme: "ws", Host: server, Path: "event_log"}
		logrus.Infof("connecting to %s", u.String())
		con, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			logrus.Errorf("dial websocket endpoint %s error. %s", u.String(), err.Error())
			return err
		}
		defer con.Close()
		done := make(chan struct{})
		con.WriteMessage(websocket.TextMessage, []byte("event_id="+eventID))
		defer con.Close()
		defer close(done)
		for {
			_, message, err := con.ReadMessage()
			if err != nil {
				logrus.Println("read proxy websocket message error: ", err)
				return err
			}
			time := gjson.GetBytes(message, "time").String()
			m := gjson.GetBytes(message, "message").String()
			level := gjson.GetBytes(message, "level").String()
			fmt.Printf("[%s](%s) %s \n", strings.ToUpper(level), time, m)
		}
	} else {
		ts:=c.Args().Get(1)
		tas:=strings.Split(ts,"/")
		dl,err:=clients.RegionClient.Tenants().Get(tas[0]).Services().EventLog(tas[1],eventID,"debug")
		if err != nil {
			return err
		}

		for _,v:=range dl{
			aa,_:=json.Marshal(v)
			fmt.Println(string(aa))
		}
	}
	return nil
}


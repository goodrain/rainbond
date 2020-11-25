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

package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/pebbe/zmq4"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/tidwall/gjson"
	"github.com/twinj/uuid"
)

const (
	REQUEST_TIMEOUT = 1000 * time.Millisecond
	MAX_RETRIES     = 3 //  Before we abandon
)

var endpoint string
var coreNum int
var t string

func AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&endpoint, "endpoint", "tcp://127.0.0.1:6363", "docker log server url")
	fs.IntVar(&coreNum, "core", 1, "core number")
	fs.StringVar(&t, "t", "1s", "时间间隔")
}

func main() {
	AddFlags(pflag.CommandLine)
	pflag.Parse()
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	re, _ := http.NewRequest("GET", "http://127.0.0.1:6363/docker-instance?service_id=asdasdadsasdasdassd", nil)
	res, err := http.DefaultClient.Do(re)
	if err != nil {
		logrus.Error(err)
		return
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logrus.Error(err)
		return
	}
	status := gjson.Get(string(body), "status")
	host := gjson.Get(string(body), "host")
	if status.String() == "success" {
		endpoint = host.String()
	} else {
		logrus.Error("获取日志接收节点失败." + gjson.Get(string(body), "host").String())
		return
	}
	var wait sync.WaitGroup
	d, _ := time.ParseDuration(t)
	for i := 0; i < coreNum; i++ {
		wait.Add(1)
		go func(en string) {
			client, _ := zmq4.NewSocket(zmq4.PUB)
			client.Monitor("inproc://monitor.rep", zmq4.EVENT_ALL)
			go monitor()
			client.Connect(en)
			defer client.Close()
			id := uuid.NewV4()
		Continuous:
			for {
				request := fmt.Sprintf(`{"event_id":"%s","message":"hello word2","time":"%s"}`, id, time.Now().Format(time.RFC3339))
				client.Send("servasd223123123123", zmq4.SNDMORE)
				_, err := client.Send(request, zmq4.DONTWAIT)
				if err != nil {
					logrus.Error("Send Error:", err)
				}
				select {
				case <-interrupt:
					break Continuous
				case <-time.Tick(d):
				}
			}
			wait.Done()
		}(endpoint)
	}
	wait.Wait()
}

func monitor() {
	mo, _ := zmq4.NewSocket(zmq4.PAIR)
	mo.Connect("inproc://monitor.rep")
	for {
		a, b, c, err := mo.RecvEvent(0)
		if err != nil {
			logrus.Error(err)
			return
		}
		logrus.Infof("A:%s B:%s C:%d", a, b, c)
	}

}

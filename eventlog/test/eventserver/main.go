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
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/pebbe/zmq4"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
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
	fs.StringVar(&endpoint, "endpoint", "tcp://127.0.0.1:6366", "event server url")
	fs.IntVar(&coreNum, "core", 10, "core number")
	fs.StringVar(&t, "t", "1s", "时间间隔")
}

func main() {
	AddFlags(pflag.CommandLine)
	pflag.Parse()
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	var wait sync.WaitGroup
	d, _ := time.ParseDuration(t)
	for i := 0; i < coreNum; i++ {
		wait.Add(1)
		go func(en string) {
			client, _ := zmq4.NewSocket(zmq4.REQ)
			client.Connect(en)
			defer client.Close()
			id := uuid.NewV4()
		Continuous:
			for {
				request := []string{fmt.Sprintf(`{"event_id":"%s","message":"hello word2","time":"%s"}`, id, time.Now().Format(time.RFC3339))}
				_, err := client.SendMessage(request)
				if err != nil {
					logrus.Error("Send:", err)
				}
				poller := zmq4.NewPoller()
				poller.Add(client, zmq4.POLLIN)
				polled, err := poller.Poll(REQUEST_TIMEOUT)
				if err != nil {
					logrus.Error("Red:", err)
				}
				reply := []string{}
				if len(polled) > 0 {
					reply, err = client.RecvMessage(0)
				} else {
					err = errors.New("Time out")
				}
				if len(reply) > 0 {
					logrus.Info(en, ":", reply[0])
				}
				if err != nil {
					logrus.Error(err)
					break Continuous
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

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

package entry

import (
	"errors"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"fmt"

	"sync"

	"github.com/pebbe/zmq4"
	"github.com/sirupsen/logrus"
	"github.com/twinj/uuid"
)

const (
	REQUEST_TIMEOUT = 1000 * time.Millisecond
	MAX_RETRIES     = 3 //  Before we abandon
)

func TestServer(t *testing.T) {
	t.SkipNow()
	request := []string{`{"event_id":"qwertyuiuiosadfkbjasdv","message":"hello word2"}`}
	reply := []string{}
	var err error
	//  For one endpoint, we retry N times
	for retries := 0; retries < MAX_RETRIES; retries++ {
		endpoint := "tcp://127.0.0.1:4320"
		reply, err = try_request(endpoint, request)
		if err == nil {
			break //  Successful
		}
		t.Errorf("W: no response from %s, %s\n", endpoint, err.Error())
	}

	if len(reply) > 0 {
		t.Logf("Service is running OK: %q\n", reply)
	}

}

func TestContinuousServer(t *testing.T) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	coreNum := 30
	var wait sync.WaitGroup
	endpoint := "tcp://127.0.0.1:4321"
	for i := 0; i < coreNum; i++ {
		if i < 15 {
			endpoint = "tcp://127.0.0.1:4320"
		} else {
			endpoint = "tcp://127.0.0.1:4321"
		}
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
				case <-time.Tick(time.Second):
				}
			}
			wait.Done()
		}(endpoint)
	}
	wait.Wait()
}

//go test -v -bench=“.”
func BenchmarkServer(b *testing.B) {
	for i := 0; i < b.N; i++ {
		request := []string{fmt.Sprintf(`{"event_id":"qwertyuiuiosadfkbjasdv","message":"hello word","step":"%d"}`, i)}
		reply := []string{}
		var err error
		//  For one endpoint, we retry N times
		for retries := 0; retries < MAX_RETRIES; retries++ {
			endpoint := "tcp://127.0.0.1:6366"
			reply, err = try_request(endpoint, request)
			if err == nil {
				break //  Successful
			}
			b.Errorf("W: no response from %s, %s\n", endpoint, err.Error())
		}
		if len(reply) > 0 {
			b.Logf("Service is running OK: %q\n", reply)
		}
	}
}

func try_request(endpoint string, request []string) (reply []string, err error) {
	logrus.Infof("I: trying echo service at %s...\n", endpoint)
	client, _ := zmq4.NewSocket(zmq4.REQ)
	client.Connect(endpoint)
	defer client.Close()
	for i := 0; i < 5; i++ {
		//  Send request, wait safely for reply
		client.SendMessage(request)
		poller := zmq4.NewPoller()
		poller.Add(client, zmq4.POLLIN)
		polled, err := poller.Poll(REQUEST_TIMEOUT)
		reply = []string{}
		if len(polled) == 1 {
			reply, err = client.RecvMessage(0)
		} else {
			err = errors.New("Time out")
		}
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

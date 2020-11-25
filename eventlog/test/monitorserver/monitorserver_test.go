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

package monitorserver

import (
	"testing"

	"github.com/pebbe/zmq4"
	"github.com/sirupsen/logrus"
)

var urlData = `
2017-05-19 11:33:34 APPS SumTimeByUrl [{"tenant":"o2o","service":"zzcplus","url":"/active/js/wx_share.js","avgtime":"1.453","sumtime":"1.453","counts":"1"}]
`
var newMonitorMessage = `
[{"ServiceID":"test",
	"Port":"5000",
	"MessageType":"http",
	"Key":"/test",
	"CumulativeTime":0.1,
	"AverageTime":0.1,
	"MaxTime":0.1,
	"Count":1,
	"AbnormalCount":0}
,{"ServiceID":"test",
	"Port":"5000",
	"MessageType":"http",
	"Key":"/test2",
	"CumulativeTime":0.36,
	"AverageTime":0.18,
	"MaxTime":0.2,
	"Count":2,
	"AbnormalCount":2}
]
`

func BenchmarkMonitorServer(t *testing.B) {
	client, _ := zmq4.NewSocket(zmq4.PUB)
	// client.Monitor("inproc://monitor.rep", zmq4.EVENT_ALL)
	// go monitor()
	client.Bind("tcp://0.0.0.0:9442")
	defer client.Close()
	var size int64
	for i := 0; i < t.N; i++ {
		client.Send("ceptop", zmq4.SNDMORE)
		_, err := client.Send(urlData, zmq4.DONTWAIT)
		if err != nil {
			logrus.Error("Send Error:", err)
		}
		size++
	}
	logrus.Info(size)
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

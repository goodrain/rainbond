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
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/goodrain/rainbond/mq/api/grpc/pb"
	"github.com/goodrain/rainbond/mq/client"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

var server string
var topic string
var taskbody string
var taskfile string
var tasktype string
var mode string

func main() {
	AddFlags(pflag.CommandLine)
	pflag.Parse()
	c, err := client.NewMqClient(server)
	if err != nil {
		logrus.Error("new mq client error.", err.Error())
		os.Exit(1)
	}
	defer c.Close()
	if mode == "enqueue" {
		if taskbody == "" && taskfile != "" {
			body, _ := ioutil.ReadFile(taskfile)
			taskbody = string(body)
		}
		fmt.Println("taskbody:" + taskbody)
		re, err := c.Enqueue(context.Background(), &pb.EnqueueRequest{
			Topic: topic,
			Message: &pb.TaskMessage{
				TaskType:   tasktype,
				CreateTime: time.Now().Format(time.RFC3339),
				TaskBody:   []byte(taskbody),
				User:       "goodrain",
			},
		})
		if err != nil {
			logrus.Error("enqueue error.", err.Error())
			os.Exit(1)
		}
		logrus.Info(re.String())
	}
	if mode == "dequeue" {
		re, err := c.Dequeue(context.Background(), &pb.DequeueRequest{
			Topic:      topic,
			ClientHost: "cli",
		})
		if err != nil {
			logrus.Error("dequeue error.", err.Error())
			os.Exit(1)
		}
		logrus.Info(re.String())
	}

}

// AddFlags -
func AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&server, "server", "127.0.0.1:6300", "mq server")
	fs.StringVar(&topic, "topic", "builder", "mq topic")
	fs.StringVar(&taskbody, "task-body", "", "mq task body")
	fs.StringVar(&taskfile, "task-file", "", "mq task body file")
	fs.StringVar(&tasktype, "task-type", "", "mq task type")
	fs.StringVar(&mode, "mode", "enqueue", "mq task type")
}

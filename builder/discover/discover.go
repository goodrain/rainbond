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

package discover

import (
	"context"
	"os"
	"time"

	"github.com/goodrain/rainbond/cmd/builder/option"
	"github.com/goodrain/rainbond/builder/exector"
	"github.com/goodrain/rainbond/mq/api/grpc/client"
	"github.com/goodrain/rainbond/mq/api/grpc/pb"
	"github.com/Sirupsen/logrus"
	grpc1 "google.golang.org/grpc"
	"fmt"
)

//WTOPIC is builder
const WTOPIC string = "builder"

var healthStatus = make(map[string]string,1)


//TaskManager task
type TaskManager struct {
	ctx          context.Context
	cancel       context.CancelFunc
	config       option.Config
	client       *client.MQClient
	exec         exector.Manager
}

//NewTaskManager return *TaskManager
func NewTaskManager(c option.Config, exec exector.Manager) *TaskManager {
	ctx, cancel := context.WithCancel(context.Background())
	healthStatus["status"] = "health"
	healthStatus["info"] = "builder service health"
	return &TaskManager{
		ctx:          ctx,
		cancel:       cancel,
		config:       c,
		exec:         exec,
	}
}

//Start 启动
func (t *TaskManager) Start() error {
	client, err := client.NewMqClient(t.config.EtcdEndPoints, t.config.MQAPI)
	if err != nil {
		logrus.Errorf("new Mq client error, %v", err)
		healthStatus["status"] = "unusual"
		healthStatus["info"] = fmt.Sprintf("new Mq client error, %v", err)
		return err
	}
	t.client = client
	go t.Do()
	logrus.Info("start discover success.")
	return nil
}

//Do do
func (t *TaskManager) Do() {
	hostName, _ := os.Hostname()
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			ctx, cancel := context.WithCancel(t.ctx)
			data, err := t.client.Dequeue(ctx, &pb.DequeueRequest{Topic: WTOPIC, ClientHost: hostName + "-builder"})
			cancel()
			if err != nil {
				if grpc1.ErrorDesc(err) == context.DeadlineExceeded.Error() {
					logrus.Warn(err.Error())
					continue
				}
				if grpc1.ErrorDesc(err) == "context canceled" {
					logrus.Warn("grpc dequeue context canceled")
					healthStatus["status"] = "unusual"
					healthStatus["info"] = "grpc dequeue context canceled"
					return
				}
				if grpc1.ErrorDesc(err) == "context timeout" {
					logrus.Warn(err.Error())
					continue
				}
				logrus.Error(err.Error())
				time.Sleep(time.Second * 2)
				continue
			}
			logrus.Debugf("Receive a task: %s", data.String())
			err = t.exec.AddTask(data)
			if err != nil {
				logrus.Error("add task error:", err.Error())
				//TODO:
				//速率控制
			}
		}
	}
}

//Stop 停止
func (t *TaskManager) Stop() error {
	logrus.Info("discover manager is stoping.")
	t.cancel()
	if t.client != nil {
		t.client.Close()
	}
	return nil
}

// 组件的健康检查
func HealthCheck() map[string]string {
	return healthStatus
}

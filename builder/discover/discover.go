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
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/goodrain/rainbond/builder/exector"
	"github.com/goodrain/rainbond/cmd/builder/option"
	"github.com/goodrain/rainbond/mq/api/grpc/pb"
	"github.com/goodrain/rainbond/mq/client"
	"github.com/sirupsen/logrus"
	grpc1 "google.golang.org/grpc"
)

//WTOPIC is builder
const WTOPIC string = "builder"

var healthStatus = make(map[string]string, 1)

//TaskManager task
type TaskManager struct {
	ctx, discoverCtx       context.Context
	cancel, discoverCancel context.CancelFunc
	config                 option.Config
	client                 client.MQClient
	exec                   exector.Manager
	callbackChan           chan *pb.TaskMessage
}

//NewTaskManager return *TaskManager
func NewTaskManager(c option.Config, client client.MQClient, exec exector.Manager) *TaskManager {
	ctx, cancel := context.WithCancel(context.Background())
	discoverCtx, discoverCancel := context.WithCancel(ctx)
	healthStatus["status"] = "health"
	healthStatus["info"] = "builder service health"
	callbackChan := make(chan *pb.TaskMessage, 100)
	taskManager := &TaskManager{
		discoverCtx:    discoverCtx,
		discoverCancel: discoverCancel,
		ctx:            ctx,
		cancel:         cancel,
		config:         c,
		client:         client,
		exec:           exec,
		callbackChan:   callbackChan,
	}
	exec.SetReturnTaskChan(taskManager.callback)
	return taskManager
}

//Start 启动
func (t *TaskManager) Start(errChan chan error) error {
	go t.Do(errChan)
	logrus.Info("start discover success.")
	return nil
}
func (t *TaskManager) callback(task *pb.TaskMessage) {
	ctx, cancel := context.WithCancel(t.ctx)
	defer cancel()
	_, err := t.client.Enqueue(ctx, &pb.EnqueueRequest{
		Topic:   client.BuilderTopic,
		Message: task,
	})
	if err != nil {
		logrus.Errorf("callback task to mq failure %s", err.Error())
	}
	logrus.Infof("The build controller returns an indigestible task(%s) to the messaging system", task.TaskId)
}

//Do do
func (t *TaskManager) Do(errChan chan error) {
	hostName, _ := os.Hostname()
	for {
		select {
		case <-t.discoverCtx.Done():
			return
		default:
			ctx, cancel := context.WithCancel(t.discoverCtx)
			data, err := t.client.Dequeue(ctx, &pb.DequeueRequest{Topic: t.config.Topic, ClientHost: hostName + "-builder"})
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
				if strings.Contains(err.Error(), "there is no connection available") {
					errChan <- fmt.Errorf("message dequeue failure %s", err.Error())
					return
				}
				logrus.Errorf("message dequeue failure %s, will retry", err.Error())
				time.Sleep(time.Second * 2)
				continue
			}
			err = t.exec.AddTask(data)
			if err != nil {
				t.callbackChan <- data
				logrus.Error("add task error:", err.Error())
			}
		}
	}
}

//Stop 停止
func (t *TaskManager) Stop() error {
	t.discoverCancel()
	if err := t.exec.Stop(); err != nil {
		logrus.Errorf("stop task exec manager failure %s", err.Error())
	}
	for len(t.callbackChan) > 0 {
		logrus.Infof("waiting callback chan empty")
		time.Sleep(time.Second * 2)
	}
	logrus.Info("discover manager is stoping.")
	t.cancel()
	if t.client != nil {
		t.client.Close()
	}
	return nil
}

// HealthCheck 组件的健康检查
func HealthCheck() map[string]string {
	return healthStatus
}

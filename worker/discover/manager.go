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
	"encoding/json"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/goodrain/rainbond/pkg/component/mq"
	"os"
	"sync"
	"time"

	"github.com/goodrain/rainbond/mq/api/grpc/pb"
	"github.com/goodrain/rainbond/mq/client"
	"github.com/goodrain/rainbond/worker/appm/controller"
	"github.com/goodrain/rainbond/worker/appm/store"
	"github.com/goodrain/rainbond/worker/discover/model"
	"github.com/goodrain/rainbond/worker/gc"
	"github.com/goodrain/rainbond/worker/handle"
	"github.com/sirupsen/logrus"
	grpc1 "google.golang.org/grpc"
)

var healthStatus = make(map[string]string, 1)
var healthStatusLock sync.Mutex

// TaskNum exec task number
var TaskNum float64

// TaskError exec error task number
var TaskError float64

// TaskManager task
type TaskManager struct {
	ctx           context.Context
	cancel        context.CancelFunc
	handleManager *handle.Manager
	k8sComponent  *k8s.Component
	serverConfig  *configs.ServerConfig
	client        client.MQClient
}

// NewTaskManager return *TaskManager
func NewTaskManager(store store.Storer,
	controllermanager *controller.Manager,
	garbageCollector *gc.GarbageCollector) *TaskManager {
	ctx, cancel := context.WithCancel(context.Background())
	handleManager := handle.NewManager(ctx, store, controllermanager, garbageCollector)
	healthStatusLock.Lock()
	healthStatus["status"] = "health"
	healthStatus["info"] = "worker service health"
	healthStatusLock.Unlock()
	return &TaskManager{
		ctx:           ctx,
		cancel:        cancel,
		handleManager: handleManager,
		k8sComponent:  k8s.Default(),
		serverConfig:  configs.Default().ServerConfig,
		client:        mq.Default().MqClient,
	}
}

// Start 启动
func (t *TaskManager) Start() error {
	go t.Do()
	logrus.Info("start discover success.")
	return nil
}

// Do do
func (t *TaskManager) Do() {
	logrus.Info("start receive task from mq")
	hostname, _ := os.Hostname()
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			data, err := t.client.Dequeue(t.ctx, &pb.DequeueRequest{Topic: client.WorkerTopic, ClientHost: hostname + "-worker"})
			if err != nil {
				if grpc1.ErrorDesc(err) == context.DeadlineExceeded.Error() {
					continue
				}
				if grpc1.ErrorDesc(err) == "context timeout" {
					continue
				}
				logrus.Error("receive task error.", err.Error())
				time.Sleep(time.Second * 2)
				continue
			}
			logrus.Debugf("receive a task: %v", data)
			transData, err := model.TransTask(data)
			if err != nil {
				logrus.Error("trans mq msg data error ", err.Error())
				continue
			}
			rc := t.handleManager.AnalystToExec(transData)
			if rc != nil && rc != handle.ErrCallback {
				logrus.Warningf("execute task: %v", rc)
				TaskError++
			} else if rc != nil && rc == handle.ErrCallback {
				logrus.Errorf("err callback; analyst to exet: %v", rc)
				ctx, cancel := context.WithCancel(t.ctx)
				reply, err := t.client.Enqueue(ctx, &pb.EnqueueRequest{
					Topic:   client.WorkerTopic,
					Message: data,
				})
				cancel()
				logrus.Debugf("retry send task to mq ,reply is %v", reply)
				if err != nil {
					logrus.Errorf("enqueue task %v to mq topic %v Error", data, client.WorkerTopic)
					continue
				}
				//if handle is waiting, sleep 3 second
				time.Sleep(time.Second * 3)
			} else {
				TaskNum++
			}
		}
	}
}

// Stop 停止
func (t *TaskManager) Stop() error {
	logrus.Info("discover manager is stoping")
	t.cancel()
	if t.client != nil {
		t.client.Close()
	}
	return nil
}

// HealthCheck health check
func HealthCheck() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	mqClient := mq.Default().MqClient
	if mqClient == nil {
		healthStatusLock.Lock()
		healthStatus["status"] = "un_health"
		healthStatus["info"] = "worker service un_health"
		result := make(map[string]string)
		for k, v := range healthStatus {
			result[k] = v
		}
		healthStatusLock.Unlock()
		return result
	}
	taskBody, _ := json.Marshal("health check")
	err := mqClient.SendBuilderTopic(client.TaskStruct{
		Topic:    client.WorkerHealth,
		TaskType: "check_worker_health",
		TaskBody: taskBody,
		Arch:     "test",
	})
	if err != nil {
		logrus.Errorf("worker check send worker topic failure: %v", err)
		healthStatusLock.Lock()
		healthStatus["status"] = "un_health"
		healthStatus["info"] = "worker service un_health"
		result := make(map[string]string)
		for k, v := range healthStatus {
			result[k] = v
		}
		healthStatusLock.Unlock()
		return result
	}
	// 等待一小段时间确保消息有时间被发送
	hostName, _ := os.Hostname()
	// 接收健康检测任务
	dequeueReq := &pb.DequeueRequest{
		Topic:      client.WorkerHealth,
		ClientHost: hostName + "health-worker",
	}
	for i := 0; i < 3; i++ {
		time.Sleep(2 * time.Second)
		msg, err := mqClient.Dequeue(ctx, dequeueReq)
		if err != nil {
			logrus.Errorf("failed to dequeue health check message: %v", err)
			continue
		}

		if msg == nil || len(msg.TaskBody) == 0 {
			continue
		}
		healthStatusLock.Lock()
		healthStatus["status"] = "health"
		healthStatus["info"] = "worker service health"
		result := make(map[string]string)
		for k, v := range healthStatus {
			result[k] = v
		}
		healthStatusLock.Unlock()
		return result
	}

	healthStatusLock.Lock()
	healthStatus["status"] = "un_health"
	healthStatus["info"] = "worker service un_health"
	result := make(map[string]string)
	for k, v := range healthStatus {
		result[k] = v
	}
	healthStatusLock.Unlock()
	return result
}

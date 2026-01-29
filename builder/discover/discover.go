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
	"github.com/goodrain/rainbond/pkg/component/mq"
	"os"
	"time"

	"github.com/goodrain/rainbond/builder/exector"
	"github.com/goodrain/rainbond/mq/api/grpc/pb"
	"github.com/goodrain/rainbond/mq/client"
	"github.com/sirupsen/logrus"
	grpc1 "google.golang.org/grpc"
)

// WTOPIC is builder
const WTOPIC string = "builder"

var healthStatus = make(map[string]string, 1)
var isReady = false

// TaskManager task
type TaskManager struct {
	ctx, discoverCtx       context.Context
	cancel, discoverCancel context.CancelFunc
	client                 client.MQClient
	exec                   exector.Manager
	callbackChan           chan *pb.TaskMessage
	ready                  bool
}

// NewChaosTaskManager return *TaskManager
func NewChaosTaskManager(exec exector.Manager) *TaskManager {
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
		client:         mq.Default().MqClient,
		exec:           exec,
		callbackChan:   callbackChan,
	}
	exec.SetReturnTaskChan(taskManager.callback)
	return taskManager
}

// Start 启动
func (t *TaskManager) Start(errChan chan error) error {
	go t.Do(errChan)

	// 等待消费循环启动并进入等待状态
	// 这个时间需要足够长，确保 Do() 循环已经执行到 Dequeue 的 Wait() 调用
	// 避免 lost wakeup 问题导致第一次任务丢失
	logrus.Info("waiting for consumer loop to start...")
	time.Sleep(time.Second * 3)

	// 发送一个测试任务并尝试接收，确保消费循环真正在运行
	// 这是一个"预热"过程
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	testBody := map[string]string{"test": "warmup"}
	testJSON, _ := json.Marshal(testBody)

	// 发送预热任务到实际使用的 topic
	topic := configs.Default().ChaosConfig.Topic
	_, err := t.client.Enqueue(ctx, &pb.EnqueueRequest{
		Topic: topic,
		Message: &pb.TaskMessage{
			TaskType:   "warmup",
			CreateTime: time.Now().Format(time.RFC3339),
			TaskBody:   testJSON,
			User:       "system",
		},
	})

	if err != nil {
		logrus.Warnf("warmup enqueue failed (non-critical): %v", err)
	} else {
		logrus.Info("warmup task sent, consumer loop should be active now")
		// 再等待一小段时间，让消费者处理预热任务
		time.Sleep(time.Millisecond * 500)
	}

	t.ready = true
	isReady = true
	logrus.Info("start discover success, chaos service is ready.")
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

// Do do
func (t *TaskManager) Do(errChan chan error) {
	hostName, _ := os.Hostname()
	for {
		select {
		case <-t.discoverCtx.Done():
			return
		default:
			ctx, cancel := context.WithCancel(t.discoverCtx)
			topic := configs.Default().ChaosConfig.Topic
			data, err := t.client.Dequeue(ctx, &pb.DequeueRequest{Topic: topic, ClientHost: hostName + "-builder"})
			cancel()
			if err != nil {
				if grpc1.ErrorDesc(err) == context.DeadlineExceeded.Error() {
					logrus.Debug("waiting for build task...")
					continue
				}
				if grpc1.ErrorDesc(err) == "context timeout" {
					logrus.Debug("waiting for build task...")
					continue
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

// Stop 停止
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

// IsReady 返回服务是否已经就绪
func IsReady() bool {
	return isReady
}

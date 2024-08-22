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

// 本文件实现了Rainbond平台中的任务管理器（TaskManager），负责从消息队列（MQ）中获取任务并进行处理。任务管理器的主要功能包括启动、执行和停止任务，并提供健康检查的接口。

// 文件中的主要内容包括：

// 1. `TaskManager` 结构体：
//    - 该结构体是任务管理器的核心，包含了上下文、配置、任务处理管理器、Kubernetes客户端和消息队列客户端等关键字段。
//    - `NewTaskManager` 函数用于初始化一个新的任务管理器实例，并返回该实例。

// 2. `Start` 方法：
//    - 启动任务管理器，与MQ建立连接，并开始从MQ中接收任务。

// 3. `Do` 方法：
//    - 这是任务管理器的主循环，持续从MQ中获取任务，并将任务交给任务处理管理器进行处理。
//    - 处理任务时，会根据不同的错误情况进行重试、记录错误计数等操作。

// 4. `Stop` 方法：
//    - 停止任务管理器，取消上下文并关闭MQ客户端连接。

// 5. 健康检查功能：
//    - 通过 `HealthCheck` 函数返回当前任务管理器的健康状态，健康状态包括服务是否正常及相关信息。

// 6. 错误和任务计数：
//    - 通过全局变量 `TaskNum` 和 `TaskError` 分别记录已执行的任务数量和发生错误的任务数量。

// 7. 消息队列的集成：
//    - 任务管理器通过 `client.MQClient` 与消息队列进行通信，接收和发送任务。

// 总体而言，本文件为Rainbond平台提供了一个可靠的任务调度和执行机制，确保在分布式环境中任务能够被有效地分配和处理。

package discover

import (
	"context"
	"fmt"
	"github.com/openkruise/kruise-api/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
	"sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/typed/apis/v1beta1"
	"time"

	"github.com/goodrain/rainbond/cmd/worker/option"
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

// TaskNum exec task number
var TaskNum float64

// TaskError exec error task number
var TaskError float64

// TaskManager task
type TaskManager struct {
	ctx           context.Context
	cancel        context.CancelFunc
	config        option.Config
	handleManager *handle.Manager
	client        client.MQClient
	kruiseClient  *versioned.Clientset
	gatewayClient *v1beta1.GatewayV1beta1Client
	restConfig    *rest.Config
	mapper        meta.RESTMapper
	clientset     *kubernetes.Clientset
}

// NewTaskManager return *TaskManager
func NewTaskManager(cfg option.Config,
	store store.Storer,
	controllermanager *controller.Manager,
	garbageCollector *gc.GarbageCollector,
	kruiseClient *versioned.Clientset,
	gatewayClient *v1beta1.GatewayV1beta1Client,
	restConfig *rest.Config,
	mapper meta.RESTMapper,
	clientset *kubernetes.Clientset) *TaskManager {

	ctx, cancel := context.WithCancel(context.Background())
	handleManager := handle.NewManager(ctx, cfg, store, controllermanager, garbageCollector, kruiseClient, gatewayClient, restConfig, mapper, clientset)
	healthStatus["status"] = "health"
	healthStatus["info"] = "worker service health"
	return &TaskManager{
		ctx:           ctx,
		cancel:        cancel,
		config:        cfg,
		handleManager: handleManager,
		restConfig:    restConfig,
		mapper:        mapper,
		clientset:     clientset,
	}
}

// Start 启动
func (t *TaskManager) Start() error {
	client, err := client.NewMqClient(t.config.MQAPI)
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
	return healthStatus
}

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

	grpc1 "google.golang.org/grpc"

	"github.com/Sirupsen/logrus"
	mysql "github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/builder/sources"
	"strings"
	imageclient "github.com/docker/engine-api/client"
	"fmt"
)

//WTOPIC is builder
const WTOPIC string = "builder"

//TaskManager task
type TaskManager struct {
	ctx    context.Context
	cancel context.CancelFunc
	config option.Config
	client *client.MQClient
	exec   exector.Manager
}

//NewTaskManager return *TaskManager
func NewTaskManager(c option.Config, exec exector.Manager) *TaskManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &TaskManager{
		ctx:    ctx,
		cancel: cancel,
		config: c,
		exec:   exec,
	}
}

//Start 启动
func (t *TaskManager) Start() error {
	client, err := client.NewMqClient(t.config.EtcdEndPoints, t.config.MQAPI)
	if err != nil {
		logrus.Errorf("new Mq client error, %v", err)
		return err
	}
	t.client = client
	go t.Do()
	go t.cleanVersion()
	logrus.Info("start discover success.")
	return nil
}

//清除三十天以前的应用构建版本数据
func (t *TaskManager) cleanVersion() {
	dc, _ := imageclient.NewEnvClient()
	now := time.Now()
	datetime := now.AddDate(0, -1, 0)
	serviceIdList := make([]string, 100)
	m := mysql.GetManager()
	timer := time.NewTimer(time.Hour * 24)
	defer timer.Stop()

	for {
		results, err := m.VersionInfoDao().SearchVersionInfo()
		if err != nil {
			fmt.Println("err", err)
		} else {
			fmt.Println("长度", len(results))
		}
		for _, v := range results {
			fmt.Println("serviceid", v.ServiceID)
			serviceIdList = append(serviceIdList, v.ServiceID)
		}
		fmt.Println(serviceIdList)
		fileResult, err := m.VersionInfoDao().GetVersionInfo(datetime, "slug", serviceIdList)
		if err != nil {
			logrus.Error(err)
			return
		}
		fmt.Println("源码个数", len(fileResult))
		for _, v := range fileResult {
			filePath := v.DeliveredPath
			if err := os.Remove(filePath); err != nil {
				if strings.Contains(err.Error(), "no such file or directory") {
					logrus.Error(err)
					if err := m.VersionInfoDao().DeleteVersionInfo(v); err != nil {
						logrus.Error(err)
						return
					}
					continue
				} else {
					logrus.Error(err)
					return
				}
			}

			os.Remove(filePath) //remove file
			logrus.Info("File deleted:", filePath)

		}

		imageResult, err := m.VersionInfoDao().GetVersionInfo(datetime, "image", serviceIdList)
		if err != nil {
			logrus.Error(err)
			return
		}
		fmt.Println("镜像个数", len(imageResult))
		for _, v := range imageResult {
			imagePath := v.DeliveredPath
			err := sources.ImageRemove(dc, imagePath) //remove image
			if err != nil && strings.Contains(err.Error(), "No such image") {
				logrus.Error(err)
				if err := m.VersionInfoDao().DeleteVersionInfo(v); err != nil {
					logrus.Error(err)
					return
				}
				continue
			}
			logrus.Info("Image deletion successful:", imagePath)

		}
		// deleted version information that failed thirty days ago
		m.VersionInfoDao().DeleteFailureVersionInfo(datetime, "failure", serviceIdList)
		select {
		case <-t.ctx.Done():
			return
		case <-timer.C:
			timer.Reset(time.Hour * 24)

		}
	}

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

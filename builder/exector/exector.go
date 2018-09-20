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

package exector

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/Sirupsen/logrus"
	//"github.com/docker/docker/client"
	"sync"

	"github.com/coreos/etcd/clientv3"
	"github.com/docker/engine-api/client"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/config"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/mq/api/grpc/pb"
	"github.com/goodrain/rainbond/util"
	"github.com/tidwall/gjson"
)

var TaskNum float64 = 0
var ErrorNum float64 = 0

//Manager 任务执行管理器
type Manager interface {
	AddTask(*pb.TaskMessage) error
	Start() error
	Stop() error
}

//NewManager new manager
func NewManager(conf config.Config) (Manager, error) {
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}
	etcdCli, err := clientv3.New(clientv3.Config{
		Endpoints:   conf.EtcdEndPoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	err = db.CreateManager(conf)
	if err != nil {
		return nil, err
	}
	//defer db.CloseManager()
	if err != nil {
		logrus.Errorf("create etcd client v3 in service check error, %v", err)
		return nil, err
	}
	return &exectorManager{
		DockerClient: dockerClient,
		EtcdCli:      etcdCli,
	}, nil
}

type exectorManager struct {
	DockerClient *client.Client
	EtcdCli      *clientv3.Client
	wg           sync.WaitGroup
}

//TaskWorker worker interface
type TaskWorker interface {
	Run(timeout time.Duration) error
	GetLogger() event.Logger
	Name() string
	Stop() error
	//ErrorCallBack if run error will callback
	ErrorCallBack(err error)
}

var workerCreaterList = make(map[string]func([]byte, *exectorManager) (TaskWorker, error))

//RegisterWorker register worker creater
func RegisterWorker(name string, fun func([]byte, *exectorManager) (TaskWorker, error)) {
	workerCreaterList[name] = fun
}

//TaskType:
//build_from_image build app from docker image
//build_from_source_code build app from source code
//build_from_market_slug build app from app market by download slug
//service_check check service source info
//plugin_image_build build plugin from image
//plugin_dockerfile_build build plugin from dockerfile
//share-slug share app with slug
//share-image share app with image
func (e *exectorManager) AddTask(task *pb.TaskMessage) error {
	e.wg.Add(1)
	TaskNum++
	switch task.TaskType {
	case "build_from_image":
		e.buildFromImage(task.TaskBody)
	case "build_from_source_code":
		e.buildFromSourceCode(task.TaskBody)
	case "build_from_market_slug":
		e.buildFromMarketSlug(task.TaskBody)
	case "service_check":
		go e.serviceCheck(task.TaskBody)
	case "plugin_image_build":
		e.pluginImageBuild(task.TaskBody)
	case "plugin_dockerfile_build":
		e.pluginDockerfileBuild(task.TaskBody)
	case "share-slug":
		e.slugShare(task.TaskBody)
	case "share-image":
		e.imageShare(task.TaskBody)
	default:
		return e.exec(task.TaskType, task.TaskBody)
	}

	return nil
}

func (e *exectorManager) exec(workerName string, in []byte) error {
	creater, ok := workerCreaterList[workerName]
	if !ok {
		return fmt.Errorf("`%s` tasktype can't support", workerName)
	}
	worker, err := creater(in, e)
	if err != nil {
		logrus.Errorf("create worker for builder error.%s", err)
		return err
	}
	go func() {
		defer e.wg.Done()
		defer event.GetManager().ReleaseLogger(worker.GetLogger())
		defer func() {
			if r := recover(); r != nil {
				fmt.Println(r)
				debug.PrintStack()
				worker.GetLogger().Error(util.Translation("Please try again or contact customer service"), map[string]string{"step": "callback", "status": "failure"})
				worker.ErrorCallBack(fmt.Errorf("%s", r))
			}
		}()
		if err := worker.Run(time.Minute * 10); err != nil {
			ErrorNum++
			worker.ErrorCallBack(err)
		}
	}()
	return nil
}

//buildFromImage build app from docker image
func (e *exectorManager) buildFromImage(in []byte) {
	i := NewImageBuildItem(in)
	i.DockerClient = e.DockerClient
	i.Logger.Info("从镜像构建应用任务开始执行", map[string]string{"step": "builder-exector", "status": "starting"})
	status := "success"
	go func() {
		start := time.Now()
		defer e.wg.Done()
		logrus.Debugf("start build from image worker")
		defer event.GetManager().ReleaseLogger(i.Logger)
		defer func() {
			if r := recover(); r != nil {
				fmt.Println(r)
				debug.PrintStack()
				i.Logger.Error("Back end service drift. Please check the rbd-chaos log", map[string]string{"step": "callback", "status": "failure"})
			}
		}()
		defer func() {
			logrus.Debugf("complete build from source code, consuming time %s", time.Now().Sub(start).String())
		}()
		for n := 0; n < 2; n++ {
			err := i.Run(time.Minute * 30)
			if err != nil {
				logrus.Errorf("build from image error: %s", err.Error())
				if n < 1 {
					i.Logger.Error("从镜像构建应用任务执行失败，开始重试", map[string]string{"step": "build-exector", "status": "failure"})
				} else {
					ErrorNum++
					i.Logger.Error("从镜像构建应用任务执行失败", map[string]string{"step": "callback", "status": "failure"})
					status = "failure"
				}
			} else {
				break
			}
		}
		if err := i.UpdateVersionInfo(status); err != nil {
			logrus.Debugf("update version Info error: %s", err.Error())
		}
	}()
}

//buildFromSourceCode build app from source code
//support git repository
func (e *exectorManager) buildFromSourceCode(in []byte) {
	i := NewSouceCodeBuildItem(in)
	i.DockerClient = e.DockerClient
	i.Logger.Info("Build app version from source code start", map[string]string{"step": "builder-exector", "status": "starting"})
	status := "success"
	go func() {
		start := time.Now()
		defer e.wg.Done()
		logrus.Debugf("start build from source code")
		defer event.GetManager().ReleaseLogger(i.Logger)
		defer func() {
			if r := recover(); r != nil {
				fmt.Println(r)
				debug.PrintStack()
				i.Logger.Error("Back end service drift. Please check the rbd-chaos log", map[string]string{"step": "callback", "status": "failure"})
			}
		}()
		defer func() {
			logrus.Debugf("Complete build from source code, consuming time %s", time.Now().Sub(start).String())
		}()
		err := i.Run(time.Minute * 30)
		if err != nil {
			logrus.Errorf("build from source code error: %s", err.Error())
			i.Logger.Error("Build app version from source code failure", map[string]string{"step": "callback", "status": "failure"})
			status = "failure"
		}
		if status == "failure" {
			vi := &dbmodel.VersionInfo{
				FinalStatus: status,
				EventID:     i.EventID,
				CodeVersion: i.commit.Hash,
				CommitMsg:   i.commit.Message,
				Author:      i.commit.Author,
			}
			if err := i.UpdateVersionInfo(vi); err != nil {
				logrus.Debugf("update version Info error: %s", err.Error())
			}
		}
	}()
}

//buildFromMarketSlug build app from market slug
func (e *exectorManager) buildFromMarketSlug(in []byte) {
	eventID := gjson.GetBytes(in, "event_id").String()
	logger := event.GetManager().GetLogger(eventID)
	logger.Info("云市应用代码包构建任务开始执行", map[string]string{"step": "builder-exector", "status": "starting"})
	i, err := NewMarketSlugItem(in)
	if err != nil {
		logrus.Error("create build from market slug task error.", err.Error())
		return
	}
	i.Logger.Info("开始构建应用", map[string]string{"step": "builder-exector", "status": "starting"})
	go func() {
		start := time.Now()
		defer e.wg.Done()
		defer event.GetManager().ReleaseLogger(i.Logger)
		defer func() {
			if r := recover(); r != nil {
				fmt.Println(r)
				debug.PrintStack()
				i.Logger.Error("Back end service drift. Please check the rbd-chaos log", map[string]string{"step": "callback", "status": "failure"})
			}
		}()
		defer func() {
			logrus.Debugf("complete build from market slug consuming time %s", time.Now().Sub(start).String())
		}()
		for n := 0; n < 2; n++ {
			err := i.Run()
			if err != nil {
				logrus.Errorf("image share error: %s", err.Error())
				if n < 1 {
					i.Logger.Error("应用构建失败，开始重试", map[string]string{"step": "builder-exector", "status": "failure"})
				} else {
					ErrorNum++
					i.Logger.Error("构建应用任务执行失败", map[string]string{"step": "callback", "status": "failure"})
				}
			} else {
				break
			}
		}
	}()

}

//slugShare share app of slug
func (e *exectorManager) slugShare(in []byte) {
	i, err := NewSlugShareItem(in, e.EtcdCli)
	if err != nil {
		logrus.Error("create share image task error.", err.Error())
		return
	}
	i.Logger.Info("开始分享应用", map[string]string{"step": "builder-exector", "status": "starting"})
	status := "success"
	go func() {
		defer e.wg.Done()
		defer event.GetManager().ReleaseLogger(i.Logger)
		defer func() {
			if r := recover(); r != nil {
				fmt.Println(r)
				debug.PrintStack()
				i.Logger.Error("后端服务开小差，请重试或联系客服", map[string]string{"step": "callback", "status": "failure"})
			}
		}()
		for n := 0; n < 2; n++ {
			err := i.ShareService()
			if err != nil {
				logrus.Errorf("image share error: %s", err.Error())
				if n < 1 {
					i.Logger.Error("应用分享失败，开始重试", map[string]string{"step": "builder-exector", "status": "failure"})
				} else {
					ErrorNum++
					i.Logger.Error("分享应用任务执行失败", map[string]string{"step": "builder-exector", "status": "failure"})
					status = "failure"
				}
			} else {
				status = "success"
				break
			}
		}
		if err := i.UpdateShareStatus(status); err != nil {
			logrus.Debugf("Add image share result error: %s", err.Error())
		}
	}()
}

//imageShare share app of docker image
func (e *exectorManager) imageShare(in []byte) {
	i, err := NewImageShareItem(in, e.DockerClient, e.EtcdCli)
	if err != nil {
		logrus.Error("create share image task error.", err.Error())
		return
	}
	i.Logger.Info("开始分享应用", map[string]string{"step": "builder-exector", "status": "starting"})
	status := "success"
	go func() {
		defer e.wg.Done()
		defer event.GetManager().ReleaseLogger(i.Logger)
		defer func() {
			if r := recover(); r != nil {
				fmt.Println(r)
				debug.PrintStack()
				i.Logger.Error("后端服务开小差，请重试或联系客服", map[string]string{"step": "callback", "status": "failure"})
			}
		}()
		for n := 0; n < 2; n++ {
			err := i.ShareService()
			if err != nil {
				logrus.Errorf("image share error: %s", err.Error())
				if n < 1 {
					i.Logger.Error("应用分享失败，开始重试", map[string]string{"step": "builder-exector", "status": "failure"})
				} else {
					ErrorNum++
					i.Logger.Error("分享应用任务执行失败", map[string]string{"step": "builder-exector", "status": "failure"})
					status = "failure"
				}
			} else {
				status = "success"
				break
			}
		}
		if err := i.UpdateShareStatus(status); err != nil {
			logrus.Debugf("Add image share result error: %s", err.Error())
		}
	}()
}

func (e *exectorManager) Start() error {
	return nil
}
func (e *exectorManager) Stop() error {
	logrus.Info("Waiting for all threads to exit.")
	e.wg.Wait()
	logrus.Info("All threads is exited.")
	return nil
}

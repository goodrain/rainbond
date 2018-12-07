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

	"sync"

	"github.com/coreos/etcd/clientv3"
	"github.com/docker/docker/client"
	"github.com/goodrain/rainbond/cmd/builder/option"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/mq/api/grpc/pb"
	mqclient "github.com/goodrain/rainbond/mq/client"
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
func NewManager(conf option.Config, mqc mqclient.MQClient) (Manager, error) {
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
	return &exectorManager{
		DockerClient: dockerClient,
		EtcdCli:      etcdCli,
		mqClient:     mqc,
		tasks:        make(map[*pb.TaskMessage][]byte),
	}, nil
}

type exectorManager struct {
	DockerClient *client.Client
	EtcdCli      *clientv3.Client
	tasks        map[*pb.TaskMessage][]byte
	taskLock     sync.RWMutex
	mqClient     mqclient.MQClient
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
	e.tasks[task] = task.TaskBody
	TaskNum++
	switch task.TaskType {
	case "build_from_image":
		e.buildFromImage(task)
	case "build_from_source_code":
		e.buildFromSourceCode(task)
	case "build_from_market_slug":
		e.buildFromMarketSlug(task)
	case "service_check":
		go e.serviceCheck(task)
	case "plugin_image_build":
		e.pluginImageBuild(task)
	case "plugin_dockerfile_build":
		e.pluginDockerfileBuild(task)
	case "share-slug":
		e.slugShare(task)
	case "share-image":
		e.imageShare(task)
	default:
		return e.exec(task)
	}

	return nil
}

func (e *exectorManager) exec(task *pb.TaskMessage) error {
	creater, ok := workerCreaterList[task.TaskType]
	if !ok {
		return fmt.Errorf("`%s` tasktype can't support", task.TaskType)
	}
	worker, err := creater(task.TaskBody, e)
	if err != nil {
		logrus.Errorf("create worker for builder error.%s", err)
		return err
	}
	go func() {
		defer e.removeTask(task)
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
func (e *exectorManager) buildFromImage(task *pb.TaskMessage) {
	i := NewImageBuildItem(task.TaskBody)
	i.DockerClient = e.DockerClient
	i.Logger.Info("Start with the image build application task", map[string]string{"step": "builder-exector", "status": "starting"})
	go func() {
		start := time.Now()
		defer e.removeTask(task)
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
					i.Logger.Error("The application task to build from the mirror failed to execute，will try", map[string]string{"step": "build-exector", "status": "failure"})
				} else {
					ErrorNum++
					i.Logger.Error("The application task to build from the image failed to execute", map[string]string{"step": "callback", "status": "failure"})
					if err := i.UpdateVersionInfo("failure"); err != nil {
						logrus.Debugf("update version Info error: %s", err.Error())
					}
				}
			} else {
				err = e.sendAction(i.TenantID, i.ServiceID, i.EventID, i.DeployVersion, i.Action, i.Logger)
				if err != nil {
					i.Logger.Error("Send upgrade action failed", map[string]string{"step": "callback", "status": "failure"})
				}
				break
			}
		}
	}()
}

//buildFromSourceCode build app from source code
//support git repository
func (e *exectorManager) buildFromSourceCode(task *pb.TaskMessage) {
	i := NewSouceCodeBuildItem(task.TaskBody)
	i.DockerClient = e.DockerClient
	i.Logger.Info("Build app version from source code start", map[string]string{"step": "builder-exector", "status": "starting"})
	go func() {
		start := time.Now()
		e.removeTask(task)
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
			vi := &dbmodel.VersionInfo{
				FinalStatus: "failure",
				EventID:     i.EventID,
				CodeVersion: i.commit.Hash,
				CommitMsg:   i.commit.Message,
				Author:      i.commit.Author,
			}
			if err := i.UpdateVersionInfo(vi); err != nil {
				logrus.Debugf("update version Info error: %s", err.Error())
			}
		} else {
			err = e.sendAction(i.TenantID, i.ServiceID, i.EventID, i.DeployVersion, i.Action, i.Logger)
			if err != nil {
				i.Logger.Error("Send upgrade action failed", map[string]string{"step": "callback", "status": "failure"})
			}
		}
	}()
}

//buildFromMarketSlug build app from market slug
func (e *exectorManager) buildFromMarketSlug(task *pb.TaskMessage) {
	eventID := gjson.GetBytes(task.TaskBody, "event_id").String()
	logger := event.GetManager().GetLogger(eventID)
	logger.Info("Build app version from market slug start", map[string]string{"step": "builder-exector", "status": "starting"})
	i, err := NewMarketSlugItem(task.TaskBody)
	if err != nil {
		logrus.Error("create build from market slug task error.", err.Error())
		return
	}
	go func() {
		start := time.Now()
		e.removeTask(task)
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
					i.Logger.Error("Build app version from market slug failure, will try", map[string]string{"step": "builder-exector", "status": "failure"})
				} else {
					ErrorNum++
					i.Logger.Error("Build app version from market slug failure", map[string]string{"step": "callback", "status": "failure"})
				}
			} else {
				err = e.sendAction(i.TenantID, i.ServiceID, i.EventID, i.DeployVersion, i.Action, i.Logger)
				if err != nil {
					i.Logger.Error("Send upgrade action failed", map[string]string{"step": "callback", "status": "failure"})
				}
				break
			}
		}
	}()

}

//rollingUpgradeTaskBody upgrade message body type
type rollingUpgradeTaskBody struct {
	TenantID  string   `json:"tenant_id"`
	ServiceID string   `json:"service_id"`
	EventID   string   `json:"event_id"`
	Strategy  []string `json:"strategy"`
}

func (e *exectorManager) sendAction(tenantID, serviceID, eventID, newVersion, actionType string, logger event.Logger) error {
	switch actionType {
	case "upgrade":
		if err := db.GetManager().TenantServiceDao().UpdateDeployVersion(serviceID, newVersion); err != nil {
			return fmt.Errorf("Update app service deploy version failure.Please try the upgrade again")
		}
		body := rollingUpgradeTaskBody{
			TenantID:  tenantID,
			ServiceID: serviceID,
			EventID:   eventID,
			Strategy:  []string{},
		}
		if err := e.mqClient.SendBuilderTopic(mqclient.TaskStruct{
			Topic:    mqclient.WorkerTopic,
			TaskType: "rolling_upgrade",
			TaskBody: body,
		}); err != nil {
			return err
		}
		logger.Info("Build success,start upgrade app service", map[string]string{"step": "builder", "status": "running"})
		return nil
	default:
		logger.Info("Build success,do not other action", map[string]string{"step": "last", "status": "success"})
	}
	return nil
}

//slugShare share app of slug
func (e *exectorManager) slugShare(task *pb.TaskMessage) {
	i, err := NewSlugShareItem(task.TaskBody, e.EtcdCli)
	if err != nil {
		logrus.Error("create share image task error.", err.Error())
		return
	}
	i.Logger.Info("开始分享应用", map[string]string{"step": "builder-exector", "status": "starting"})
	status := "success"
	go func() {
		defer e.removeTask(task)
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
func (e *exectorManager) imageShare(task *pb.TaskMessage) {
	i, err := NewImageShareItem(task.TaskBody, e.DockerClient, e.EtcdCli)
	if err != nil {
		logrus.Error("create share image task error.", err.Error())
		return
	}
	i.Logger.Info("开始分享应用", map[string]string{"step": "builder-exector", "status": "starting"})
	status := "success"
	go func() {
		e.removeTask(task)
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
	i := 0
	timer := time.NewTimer(time.Second * 2)
	defer timer.Stop()
	for {
		if i >= 15 {
			logrus.Errorf("There are %d tasks not completed", len(e.tasks))
			return fmt.Errorf("There are %d tasks not completed ", len(e.tasks))
		}
		if len(e.tasks) == 0 {
			break
		}
		select {
		case <-timer.C:
			i++
			timer.Reset(time.Second * 2)
		}
	}
	logrus.Info("All threads is exited.")
	return nil
}

func (e *exectorManager) removeTask(task *pb.TaskMessage) {
	e.taskLock.Lock()
	defer e.taskLock.Unlock()
	delete(e.tasks, task)
}

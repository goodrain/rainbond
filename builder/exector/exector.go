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
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/clientv3"
	"github.com/docker/docker/client"
	"github.com/goodrain/rainbond/cmd/builder/option"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/mq/api/grpc/pb"
	mqclient "github.com/goodrain/rainbond/mq/client"
	"github.com/goodrain/rainbond/util"
	workermodel "github.com/goodrain/rainbond/worker/discover/model"
	"github.com/tidwall/gjson"
)

//MetricTaskNum task number
var MetricTaskNum float64

//MetricErrorTaskNum error run task number
var MetricErrorTaskNum float64

//MetricBackTaskNum back task number
var MetricBackTaskNum float64

//Manager 任务执行管理器
type Manager interface {
	GetMaxConcurrentTask() float64
	GetCurrentConcurrentTask() float64
	AddTask(*pb.TaskMessage) error
	SetReturnTaskChan(func(*pb.TaskMessage))
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
	maxConcurrentTask := runtime.NumCPU() * 2
	ctx, cancel := context.WithCancel(context.Background())
	logrus.Infof("The maximum number of concurrent build tasks supported by the current node is %d", maxConcurrentTask)
	return &exectorManager{
		DockerClient:      dockerClient,
		EtcdCli:           etcdCli,
		mqClient:          mqc,
		tasks:             make(chan *pb.TaskMessage, maxConcurrentTask),
		maxConcurrentTask: maxConcurrentTask,
		ctx:               ctx,
		cancel:            cancel,
	}, nil
}

type exectorManager struct {
	DockerClient      *client.Client
	EtcdCli           *clientv3.Client
	tasks             chan *pb.TaskMessage
	callback          func(*pb.TaskMessage)
	maxConcurrentTask int
	mqClient          mqclient.MQClient
	ctx               context.Context
	cancel            context.CancelFunc
	runningTask       sync.Map
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

//ErrCallback do not handle this task
var ErrCallback = fmt.Errorf("callback task to mq")

func (e *exectorManager) SetReturnTaskChan(re func(*pb.TaskMessage)) {
	e.callback = re
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
	select {
	case e.tasks <- task:
		MetricTaskNum++
		e.RunTask(task)
		return nil
	default:
		logrus.Infof("The current number of parallel builds exceeds the maximum")
		if e.callback != nil {
			e.callback(task)
			//Wait a while
			//It's best to wait until the current controller can continue adding tasks
			for len(e.tasks) >= e.maxConcurrentTask {
				time.Sleep(time.Second * 2)
			}
			MetricBackTaskNum++
			return nil
		}
		return ErrCallback
	}
}
func (e *exectorManager) runTask(f func(task *pb.TaskMessage), task *pb.TaskMessage, concurrencyControl bool) {
	logrus.Infof("Build task %s in progress", task.TaskId)
	e.runningTask.LoadOrStore(task.TaskId, task)
	if !concurrencyControl {
		<-e.tasks
	} else {
		defer func() { <-e.tasks }()
	}
	f(task)
	e.runningTask.Delete(task.TaskId)
	logrus.Infof("Build task %s is completed", task.TaskId)
}
func (e *exectorManager) runTaskWithErr(f func(task *pb.TaskMessage) error, task *pb.TaskMessage, concurrencyControl bool) {
	logrus.Infof("Build task %s in progress", task.TaskId)
	e.runningTask.LoadOrStore(task.TaskId, task)
	//Remove a task that is being executed, not necessarily a task that is currently completed
	if !concurrencyControl {
		<-e.tasks
	} else {
		defer func() { <-e.tasks }()
	}
	if err := f(task); err != nil {
		logrus.Errorf("run builder task failure %s", err.Error())
	}
	e.runningTask.Delete(task.TaskId)
	logrus.Infof("Build task %s is completed", task.TaskId)
}
func (e *exectorManager) RunTask(task *pb.TaskMessage) {
	switch task.TaskType {
	case "build_from_image":
		go e.runTask(e.buildFromImage, task, false)
	case "build_from_source_code":
		go e.runTask(e.buildFromSourceCode, task, true)
	case "build_from_market_slug":
		//deprecated
		go e.runTask(e.buildFromMarketSlug, task, false)
	case "service_check":
		go e.runTask(e.serviceCheck, task, true)
	case "plugin_image_build":
		go e.runTask(e.pluginImageBuild, task, false)
	case "plugin_dockerfile_build":
		go e.runTask(e.pluginDockerfileBuild, task, true)
	case "share-slug":
		//deprecated
		go e.runTask(e.slugShare, task, false)
	case "share-image":
		go e.runTask(e.imageShare, task, false)
	default:
		go e.runTaskWithErr(e.exec, task, false)
	}
}

func (e *exectorManager) exec(task *pb.TaskMessage) error {
	creator, ok := workerCreaterList[task.TaskType]
	if !ok {
		return fmt.Errorf("`%s` tasktype can't support", task.TaskType)
	}
	worker, err := creator(task.TaskBody, e)
	if err != nil {
		logrus.Errorf("create worker for builder error.%s", err)
		return err
	}
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
		MetricErrorTaskNum++
		worker.ErrorCallBack(err)
	}
	return nil
}

//buildFromImage build app from docker image
func (e *exectorManager) buildFromImage(task *pb.TaskMessage) {
	i := NewImageBuildItem(task.TaskBody)
	i.DockerClient = e.DockerClient
	i.Logger.Info("Start with the image build application task", map[string]string{"step": "builder-exector", "status": "starting"})
	defer event.GetManager().ReleaseLogger(i.Logger)
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
			debug.PrintStack()
			i.Logger.Error("Back end service drift. Please check the rbd-chaos log", map[string]string{"step": "callback", "status": "failure"})
		}
	}()
	start := time.Now()
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
				MetricErrorTaskNum++
				i.Logger.Error("The application task to build from the image failed to execute", map[string]string{"step": "callback", "status": "failure"})
				if err := i.UpdateVersionInfo("failure"); err != nil {
					logrus.Debugf("update version Info error: %s", err.Error())
				}
			}
		} else {
			var configs = make(map[string]string, len(i.Configs))
			for k, v := range i.Configs {
				configs[k] = v.String()
			}
			err = e.sendAction(i.TenantID, i.ServiceID, i.EventID, i.DeployVersion, i.Action, configs, i.Logger)
			if err != nil {
				i.Logger.Error("Send upgrade action failed", map[string]string{"step": "callback", "status": "failure"})
			}
			break
		}
	}
}

//buildFromSourceCode build app from source code
//support git repository
func (e *exectorManager) buildFromSourceCode(task *pb.TaskMessage) {
	i := NewSouceCodeBuildItem(task.TaskBody)
	i.DockerClient = e.DockerClient
	i.Logger.Info("Build app version from source code start", map[string]string{"step": "builder-exector", "status": "starting"})
	start := time.Now()
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
			FinishTime:  time.Now(),
		}
		if err := i.UpdateVersionInfo(vi); err != nil {
			logrus.Errorf("update version Info error: %s", err.Error())
			// TODO(huangrh 20190816): use logger
		}
	} else {
		var configs = make(map[string]string, len(i.Configs))
		for k, v := range i.Configs {
			configs[k] = v.String()
		}
		err = e.sendAction(i.TenantID, i.ServiceID, i.EventID, i.DeployVersion, i.Action, configs, i.Logger)
		if err != nil {
			i.Logger.Error("Send upgrade action failed", map[string]string{"step": "callback", "status": "failure"})
		}
	}
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
					MetricErrorTaskNum++
					i.Logger.Error("Build app version from market slug failure", map[string]string{"step": "callback", "status": "failure"})
				}
			} else {
				err = e.sendAction(i.TenantID, i.ServiceID, i.EventID, i.DeployVersion, i.Action, i.Configs, i.Logger)
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

func (e *exectorManager) sendAction(tenantID, serviceID, eventID, newVersion, actionType string, configs map[string]string, logger event.Logger) error {
	// update build event complete status
	logger.Info("Build success", map[string]string{"step": "last", "status": "success"})
	switch actionType {
	case "upgrade":
		//add upgrade event
		event := &dbmodel.ServiceEvent{
			EventID:   util.NewUUID(),
			TenantID:  tenantID,
			ServiceID: serviceID,
			StartTime: time.Now().Format(time.RFC3339),
			OptType:   "upgrade",
			Target:    "service",
			TargetID:  serviceID,
			UserName:  "system",
			SynType:   dbmodel.ASYNEVENTTYPE,
		}
		if err := db.GetManager().ServiceEventDao().AddModel(event); err != nil {
			logrus.Errorf("create upgrade event failure %s, service %s do not auto upgrade", err.Error(), serviceID)
			return nil
		}
		if err := db.GetManager().TenantServiceDao().UpdateDeployVersion(serviceID, newVersion); err != nil {
			logrus.Errorf("Update app service deploy version failure %s, service %s do not auto upgrade", err.Error(), serviceID)
			return nil
		}
		body := workermodel.RollingUpgradeTaskBody{
			TenantID:         tenantID,
			ServiceID:        serviceID,
			NewDeployVersion: newVersion,
			EventID:          event.EventID,
			Configs:          configs,
		}
		if err := e.mqClient.SendBuilderTopic(mqclient.TaskStruct{
			Topic:    mqclient.WorkerTopic,
			TaskType: "rolling_upgrade", // TODO(huangrh 20190816): Separate from build
			TaskBody: body,
		}); err != nil {
			return err
		}
		return nil
	default:
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
					MetricErrorTaskNum++
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
				MetricErrorTaskNum++
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
}

func (e *exectorManager) Start() error {
	return nil
}
func (e *exectorManager) Stop() error {
	e.cancel()
	logrus.Info("Waiting for all threads to exit.")
	//Recycle all ongoing tasks
	e.runningTask.Range(func(k, v interface{}) bool {
		task := v.(*pb.TaskMessage)
		e.callback(task)
		return true
	})
	logrus.Info("All threads is exited.")
	return nil
}

func (e *exectorManager) GetMaxConcurrentTask() float64 {
	return float64(e.maxConcurrentTask)
}
func (e *exectorManager) GetCurrentConcurrentTask() float64 {
	return float64(len(e.tasks))
}

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
	"time"

	"github.com/Sirupsen/logrus"
	//"github.com/docker/docker/client"
	"github.com/coreos/etcd/clientv3"
	"github.com/docker/engine-api/client"
	"github.com/goodrain/rainbond/pkg/db"
	"github.com/goodrain/rainbond/pkg/db/config"
	dbmodel "github.com/goodrain/rainbond/pkg/db/model"
	"github.com/goodrain/rainbond/pkg/event"
	"github.com/goodrain/rainbond/pkg/mq/api/grpc/pb"
	"github.com/tidwall/gjson"
)

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
}

//TaskType:
//app_image 云市镜像构建
//app_slug 云市代码包构建
//image_manual 自定义镜像
//code_check 代码检测
//app_build 源码构建
func (e *exectorManager) AddTask(task *pb.TaskMessage) error {

	switch task.TaskType {
	case "app_image":
		e.appImage(task.TaskBody)
	case "build_from_image":
		e.buildFromImage(task.TaskBody)
	case "build_from_source_code":
		e.buildFromSourceCode(task.TaskBody)
	case "build_from_market_slug":
		e.buildFromMarketSlug(task.TaskBody)
	case "app_slug":
		e.appSlug(task.TaskBody)
	case "image_manual":
		e.imageManual(task.TaskBody)
	case "code_check":
		e.codeCheck(task.TaskBody)
	case "service_check":
		go e.serviceCheck(task.TaskBody)
	case "app_build":
		e.appBuild(task.TaskBody)
	case "plugin_image_build":
		e.pluginImageBuild(task.TaskBody)
	case "plugin_dockerfile_build":
		e.pluginDockerfileBuild(task.TaskBody)
	case "share-slug":
		e.slugShare(task.TaskBody)
	case "share-image":
		e.imageShare(task.TaskBody)
	default:
		return fmt.Errorf("`%s` tasktype can't support", task.TaskType)
	}
	return nil
}

const appImage = "plugins/app_image.pyc"
const appSlug = "plugins/app_slug.pyc"
const appBuild = "plugins/build_work.pyc"
const codeCheck = "plugins/code_check.pyc"
const imageManual = "plugins/image_manual.pyc"
const pluginImage = "plugins/plugin_image.pyc"
const pluginDockerfile = "plugins/plugin_dockerfile.pyc"

func (e *exectorManager) appImage(in []byte) {
	eventID := gjson.GetBytes(in, "event_id").String()
	//dest := gjson.GetBytes(in, "dest").String()
	//finalStatus:="failure"
	logger := event.GetManager().GetLogger(eventID)
	logger.Info("应用镜像构建任务开始执行", map[string]string{"step": "builder-exector", "status": "starting"})
	w := NewWorker(appImage, "", nil, in)
	go func() {
		logrus.Info("start exec app image worker")
		defer event.GetManager().ReleaseLogger(logger)
		for i := 0; i < 3; i++ {
			_, err := w.run(time.Minute * 30)
			if err != nil {
				logrus.Errorf("exec app image python shell error:%s", err.Error())
				if i < 2 {
					logger.Info("应用镜像构建任务执行失败,开始重试", map[string]string{"step": "builder-exector", "status": "failure"})
				} else {
					logger.Info("应用镜像构建任务执行失败", map[string]string{"step": "callback", "status": "failure"})
				}
			} else {
				//finalStatus="success"
				//updateBuildResult(eventID,finalStatus,dest)
				break
			}
		}
	}()
	//updateBuildResult(eventID,finalStatus,dest)
}
func (e *exectorManager) buildFromImage(in []byte) {
	i := NewImageBuildItem(in)
	i.DockerClient = e.DockerClient
	i.Logger.Info("从镜像构建应用任务开始执行", map[string]string{"step": "builder-exector", "status": "starting"})
	status := "success"
	go func() {
		logrus.Debugf("start build from image worker")
		defer event.GetManager().ReleaseLogger(i.Logger)
		for n := 0; n < 2; n++ {
			err := i.Run(time.Minute * 30)
			if err != nil {
				logrus.Errorf("build from image error: %s", err.Error())
				if n < 1 {
					i.Logger.Error("从镜像构建应用任务执行失败，开始重试", map[string]string{"step": "build-exector", "status": "failure"})
				} else {
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

func (e *exectorManager) buildFromSourceCode(in []byte) {
	i := NewSouceCodeBuildItem(in)
	i.DockerClient = e.DockerClient
	i.Logger.Info("从源码构建应用任务开始执行", map[string]string{"step": "builder-exector", "status": "starting"})
	status := "success"
	go func() {
		logrus.Debugf("start build from source code")
		defer event.GetManager().ReleaseLogger(i.Logger)
		for n := 0; n < 2; n++ {
			err := i.Run(time.Minute * 30)
			if err != nil {
				logrus.Errorf("build from source code error: %s", err.Error())
				if n < 1 {
					i.Logger.Error("从源码构建应用任务执行失败，开始重试", map[string]string{"step": "build-exector", "status": "failure"})
				} else {
					i.Logger.Error("从源码构建应用任务执行失败", map[string]string{"step": "callback", "status": "failure"})
					status = "failure"
				}
			} else {
				break
			}
		}
		vi := &dbmodel.VersionInfo{
			FinalStatus: status,
		}
		if err := i.UpdateVersionInfo(vi); err != nil {
			logrus.Debugf("update version Info error: %s", err.Error())
		}
	}()
}

//buildFromMarketSlug 从云市来源源码包构建应用
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
		defer event.GetManager().ReleaseLogger(i.Logger)
		for n := 0; n < 2; n++ {
			err := i.Run()
			if err != nil {
				logrus.Errorf("image share error: %s", err.Error())
				if n < 1 {
					i.Logger.Error("应用构建失败，开始重试", map[string]string{"step": "builder-exector", "status": "failure"})
				} else {
					i.Logger.Error("构建应用任务执行失败", map[string]string{"step": "callback", "status": "failure"})
				}
			} else {
				break
			}
		}
	}()

}

func (e *exectorManager) appSlug(in []byte) {
	eventID := gjson.GetBytes(in, "event_id").String()
	logger := event.GetManager().GetLogger(eventID)
	logger.Info("应用代码包构建任务开始执行", map[string]string{"step": "builder-exector", "status": "starting"})
	w := NewWorker(appSlug, "", nil, in)
	go func() {
		logrus.Info("start exec app slug worker")
		defer event.GetManager().ReleaseLogger(logger)
		for i := 0; i < 3; i++ {
			_, err := w.run(time.Minute * 30)
			if err != nil {
				logrus.Errorf("exec app slug python shell error:%s", err.Error())
				if i < 2 {
					logger.Info("应用代码包构建任务执行失败,开始重试", map[string]string{"step": "builder-exector", "status": "failure"})
				} else {
					logger.Info("应用代码包构建任务执行失败", map[string]string{"step": "callback", "status": "failure"})
				}
			} else {
				break
			}
		}
	}()
}
func (e *exectorManager) imageManual(in []byte) {
	eventID := gjson.GetBytes(in, "event_id").String()
	logger := event.GetManager().GetLogger(eventID)
	//dest := gjson.GetBytes(in, "dest").String()
	//finalStatus:="failure"

	logger.Info("应用镜像构建任务开始执行", map[string]string{"step": "builder-exector", "status": "starting"})
	w := NewWorker(imageManual, "", nil, in)
	go func() {
		defer event.GetManager().ReleaseLogger(logger)
		logrus.Info("start exec image manual worker")
		for i := 0; i < 3; i++ {
			_, err := w.run(time.Minute * 30)
			if err != nil {
				logrus.Errorf("exec image manual python shell error:%s", err.Error())
				if i < 2 {
					logger.Info("应用镜像构建任务执行失败,开始重试", map[string]string{"step": "builder-exector", "status": "failure"})
				} else {
					logger.Info("应用镜像构建任务执行失败", map[string]string{"step": "callback", "status": "failure"})
				}
			} else {
				//finalStatus="success"
				//updateBuildResult(eventID,finalStatus,dest)
				break
			}
		}
	}()
	//updateBuildResult(eventID,finalStatus,dest)
}
func (e *exectorManager) codeCheck(in []byte) {
	eventID := gjson.GetBytes(in, "event_id").String()
	logger := event.GetManager().GetLogger(eventID)
	logger.Info("应用代码检测任务开始执行", map[string]string{"step": "builder-exector", "status": "starting"})
	w := NewWorker(codeCheck, "", nil, in)
	go func() {
		logrus.Info("start exec code check worker")
		defer event.GetManager().ReleaseLogger(logger)
		for i := 0; i < 3; i++ {
			_, err := w.run(time.Minute * 30)
			if err != nil {
				logrus.Errorf("exec code check python shell error:%s", err.Error())
				if i < 2 {
					logger.Info("应用镜像构建任务执行失败,开始重试", map[string]string{"step": "builder-exector", "status": "failure"})
				} else {
					logger.Info("应用镜像构建任务执行失败", map[string]string{"step": "callback", "status": "failure"})
				}
			} else {
				break
			}
		}
	}()
}
func (e *exectorManager) appBuild(in []byte) {
	eventID := gjson.GetBytes(in, "event_id").String()
	//finalStatus:="failure"
	//dest := gjson.GetBytes(in, "dest").String()
	logger := event.GetManager().GetLogger(eventID)
	logger.Info("应用编译构建任务开始执行", map[string]string{"step": "builder-exector", "status": "starting"})

	w := NewWorker(appBuild, "", nil, in)
	go func() {
		logrus.Info("start exec build app worker")
		defer event.GetManager().ReleaseLogger(logger)
		for i := 0; i < 3; i++ {
			_, err := w.run(time.Minute * 30)
			if err != nil {
				logrus.Errorf("exec app build python shell error:%s", err.Error())
				if i < 2 {
					logger.Info("应用编译构建任务执行失败,开始重试", map[string]string{"step": "builder-exector", "status": "failure"})
				} else {
					logger.Info("应用编译构建任务执行失败", map[string]string{"step": "callback", "status": "failure"})
				}
			} else {
				//finalStatus="success"
				//updateBuildResult(eventID,finalStatus,dest)
				break
			}
		}
	}()
	//updateBuildResult(eventID,finalStatus,dest)
}

func (e *exectorManager) slugShare(in []byte) {
	i, err := NewSlugShareItem(in, e.EtcdCli)
	if err != nil {
		logrus.Error("create share image task error.", err.Error())
		return
	}
	i.Logger.Info("开始分享应用", map[string]string{"step": "builder-exector", "status": "starting"})
	status := "success"
	go func() {
		defer event.GetManager().ReleaseLogger(i.Logger)
		for n := 0; n < 2; n++ {
			err := i.ShareService()
			if err != nil {
				logrus.Errorf("image share error: %s", err.Error())
				if n < 1 {
					i.Logger.Error("应用分享失败，开始重试", map[string]string{"step": "builder-exector", "status": "failure"})
				} else {
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

func (e *exectorManager) imageShare(in []byte) {
	i, err := NewImageShareItem(in, e.DockerClient, e.EtcdCli)
	if err != nil {
		logrus.Error("create share image task error.", err.Error())
		return
	}
	i.Logger.Info("开始分享应用", map[string]string{"step": "builder-exector", "status": "starting"})
	status := "success"
	go func() {
		defer event.GetManager().ReleaseLogger(i.Logger)
		for n := 0; n < 2; n++ {
			err := i.ShareService()
			if err != nil {
				logrus.Errorf("image share error: %s", err.Error())
				if n < 1 {
					i.Logger.Error("应用分享失败，开始重试", map[string]string{"step": "builder-exector", "status": "failure"})
				} else {
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

func (e *exectorManager) pluginImageBuild1(in []byte) {
	eventID := gjson.GetBytes(in, "event_id").String()
	logger := event.GetManager().GetLogger(eventID)
	logger.Info("从镜像构建插件任务开始执行", map[string]string{"step": "builder-exector", "status": "starting"})

	w := NewWorker(pluginImage, "", nil, in)
	go func() {
		logrus.Info("start exec build plugin from image worker")
		defer event.GetManager().ReleaseLogger(logger)
		for i := 0; i < 3; i++ {
			_, err := w.run(time.Minute * 30)
			if err != nil {
				logrus.Errorf("exec plugin build from image python shell error:%s", err.Error())
				if i < 2 {
					logger.Info("镜像构建插件任务执行失败，开始重试", map[string]string{"step": "builder-exector", "status": "failure"})
				} else {
					logger.Info("镜像构建插件任务执行失败", map[string]string{"step": "callback", "status": "failure"})
				}
			} else {
				break
			}
		}
	}()
}

func (e *exectorManager) pluginDockerfileBuild1(in []byte) {
	eventID := gjson.GetBytes(in, "event_id").String()
	logger := event.GetManager().GetLogger(eventID)
	logger.Info("从dockerfile构建插件任务开始执行", map[string]string{"step": "builder-exector", "status": "starting"})

	w := NewWorker(pluginDockerfile, "", nil, in)
	go func() {
		logrus.Info("start exec build plugin from image worker")
		defer event.GetManager().ReleaseLogger(logger)
		for i := 0; i < 3; i++ {
			_, err := w.run(time.Minute * 30)
			if err != nil {
				logrus.Errorf("exec plugin build from image python shell error:%s", err.Error())
				if i < 2 {
					logger.Info("dockerfile构建插件任务执行失败，开始重试", map[string]string{"step": "builder-exector", "status": "failure"})
				} else {
					logger.Info("dockerfile构建插件任务执行失败", map[string]string{"step": "callback", "status": "failure"})
				}
			} else {
				break
			}
		}
	}()
}

func (e *exectorManager) Start() error {
	return nil
}
func (e *exectorManager) Stop() error {
	return nil
}

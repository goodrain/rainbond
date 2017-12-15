
// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.
 
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

	"github.com/goodrain/rainbond/pkg/db/config"
	"github.com/goodrain/rainbond/pkg/event"
	"github.com/goodrain/rainbond/pkg/mq/api/grpc/pb"
	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/client"
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
	return &exectorManager{
		DockerClient: dockerClient,
	}, nil
}

type exectorManager struct {
	DockerClient *client.Client
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
	case "app_slug":
		e.appSlug(task.TaskBody)
	case "image_manual":
		e.imageManual(task.TaskBody)
	case "code_check":
		e.codeCheck(task.TaskBody)
	case "app_build":
		e.appBuild(task.TaskBody)
	case "plugin_image_build":
		e.pluginImageBuild(task.TaskBody)
	case "plugin_dockerfile_build":
		e.pluginDockerfileBuild(task.TaskBody)
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
				if i < 3 {
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
func (e *exectorManager) appSlug(in []byte) {
	//eventID := gjson.GetBytes(in, "event_id").String()
	////dest := gjson.GetBytes(in, "dest").String()
	////finalStatus:="failure"
	//
	//logger := event.GetManager().GetLogger(eventID)
	//logger.Info("应用代码包构建任务开始执行", map[string]string{"step": "builder-exector", "status": "starting"})
	//w := NewWorker(appSlug, "", nil, in)
	//go func() {
	//	logrus.Info("start exec app slug worker")
	//	defer event.GetManager().ReleaseLogger(logger)
	//	for i := 0; i < 3; i++ {
	//		_, err := w.run(time.Minute * 30)
	//		if err != nil {
	//			logrus.Errorf("exec app slug python shell error:%s", err.Error())
	//			if i < 3 {
	//				logger.Info("应用代码包构建任务执行失败,开始重试", map[string]string{"step": "builder-exector", "status": "failure"})
	//			} else {
	//				logger.Info("应用代码包构建任务执行失败", map[string]string{"step": "callback", "status": "failure"})
	//
	//			}
	//		} else {
	//			//updateBuildResult(eventID,"success",dest)
	//			break
	//		}
	//	}
	//}()
	//updateBuildResult(eventID,"failure",dest)



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
				if i < 3 {
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
				if i < 3 {
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
				if i < 3 {
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
				if i < 3 {
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
//func updateBuildResult(eventID,finalStatus,dest string)  {
//	if dest == ""||!strings.Contains(dest,"y") {
//		v,err:=db.GetManager().VersionInfoDao().GetVersionByEventID(eventID)
//		if err != nil {
//			logrus.Errorf("error get version by eventID %s  from db,details %s",eventID,err.Error())
//			return
//		}
//		v.FinalStatus=finalStatus
//		db.GetManager().VersionInfoDao().UpdateModel(v)
//	}
//
//}
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
				if i < 3 {
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
				if i < 3 {
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

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
	"strings"

	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/sources"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/event"

	"github.com/pquerna/ffjson/ffjson"

	"github.com/goodrain/rainbond/builder/model"

	"github.com/Sirupsen/logrus"
)

func (e *exectorManager) pluginImageBuild(in []byte) {
	var tb model.BuildPluginTaskBody
	if err := ffjson.Unmarshal(in, &tb); err != nil {
		logrus.Errorf("unmarshal taskbody error, %v", err)
		return
	}
	eventID := tb.EventID
	logger := event.GetManager().GetLogger(eventID)
	logger.Info("从镜像构建插件任务开始执行", map[string]string{"step": "builder-exector", "status": "starting"})
	go func() {
		logrus.Info("start exec build plugin from image worker")
		defer event.GetManager().ReleaseLogger(logger)
		for retry := 0; retry < 2; retry++ {
			err := e.run(&tb, logger)
			if err != nil {
				logrus.Errorf("exec plugin build from image error:%s", err.Error())
				logger.Info("镜像构建插件任务执行失败，开始重试", map[string]string{"step": "builder-exector", "status": "failure"})
			} else {
				return
			}
		}
		version, err := db.GetManager().TenantPluginBuildVersionDao().GetBuildVersionByDeployVersion(tb.PluginID, tb.VersionID, tb.DeployVersion)
		if err != nil {
			logrus.Errorf("get version error, %v", err)
			return
		}
		version.Status = "failure"
		if err := db.GetManager().TenantPluginBuildVersionDao().UpdateModel(version); err != nil {
			logrus.Errorf("update version error, %v", err)
		}
		ErrorNum += 1
		logger.Info("镜像构建插件任务执行失败", map[string]string{"step": "callback", "status": "failure"})
	}()
}

func (e *exectorManager) run(t *model.BuildPluginTaskBody, logger event.Logger) error {

	if _, err := sources.ImagePull(e.DockerClient, t.ImageURL, t.ImageInfo.HubUser, t.ImageInfo.HubPassword, logger, 10); err != nil {
		logrus.Errorf("pull image %v error, %v", t.ImageURL, err)
		logger.Error("拉取镜像失败", map[string]string{"step": "builder-exector", "status": "failure"})
		return err
	}
	logger.Info("拉取镜像完成", map[string]string{"step": "build-exector", "status": "complete"})
	newTag := createPluginImageTag(t.ImageURL, t.PluginID, t.DeployVersion)
	err := sources.ImageTag(e.DockerClient, t.ImageURL, newTag, logger, 1)
	if err != nil {
		logrus.Errorf("set plugin image tag error, %v", err)
		logger.Error("修改镜像tag失败", map[string]string{"step": "builder-exector", "status": "failure"})
		return err
	}
	logger.Info("修改镜像Tag完成", map[string]string{"step": "build-exector", "status": "complete"})
	if err := sources.ImagePush(e.DockerClient, newTag, builder.REGISTRYUSER, builder.REGISTRYPASS, logger, 10); err != nil {
		logrus.Errorf("push image %s error, %v", newTag, err)
		logger.Error("推送镜像失败", map[string]string{"step": "builder-exector", "status": "failure"})
		return err
	}
	version, err := db.GetManager().TenantPluginBuildVersionDao().GetBuildVersionByDeployVersion(t.PluginID, t.VersionID, t.DeployVersion)
	if err != nil {
		logger.Error("更新插件版本信息错误", map[string]string{"step": "builder-exector", "status": "failure"})
		return err
	}
	version.BuildLocalImage = newTag
	version.Status = "complete"
	if err := db.GetManager().TenantPluginBuildVersionDao().UpdateModel(version); err != nil {
		logger.Error("更新插件版本信息错误", map[string]string{"step": "builder-exector", "status": "failure"})
		return err
	}
	logger.Info("从镜像构建插件完成", map[string]string{"step": "last", "status": "success"})
	return nil
}

func createPluginImageTag(image string, pluginid, version string) string {
	//alias is pluginID
	mm := strings.Split(image, "/")
	tag := "latest"
	iName := ""
	if strings.Contains(mm[len(mm)-1], ":") {
		nn := strings.Split(mm[len(mm)-1], ":")
		tag = nn[1]
		iName = nn[0]
	} else {
		iName = image
	}
	if strings.HasPrefix(iName, "plugin") {
		return fmt.Sprintf("%s/%s:%s_%s", builder.REGISTRYDOMAIN, iName, pluginid, version)
	}
	return fmt.Sprintf("%s/plugin_%s_%s:%s_%s", builder.REGISTRYDOMAIN, iName, pluginid, tag, version)
}

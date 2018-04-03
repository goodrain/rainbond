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

/*
Copyright 2017 The Goodrain Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/docker/engine-api/types"
	"github.com/goodrain/rainbond/pkg/builder/sources"

	"github.com/goodrain/rainbond/pkg/db"
	"github.com/goodrain/rainbond/pkg/event"

	"github.com/pquerna/ffjson/ffjson"

	"github.com/goodrain/rainbond/pkg/builder/model"

	"github.com/Sirupsen/logrus"
	"github.com/akkuman/parseConfig"
)

//const dockerBin = "docker"
const dockerBin = "sudo -P docker"
const configPath = "plugins/config.json"

func (e *exectorManager) pluginImageBuild(in []byte) {
	if err := checkConf(configPath); err != nil {
		logrus.Errorf("config check error, %v", err)
	}
	config := getConf(configPath)
	var tb model.BuildPluginTaskBody
	if err := ffjson.Unmarshal(in, &tb); err != nil {
		logrus.Errorf("unmarshal taskbody error, %v", err)
		return
	}
	eventID := tb.EventID
	logger := event.GetManager().GetLogger(eventID)
	logger.Info("从镜像构建插件任务开始执行", map[string]string{"step": "builder-exector", "status": "starting"})
	go func() {
		time.Sleep(buildingTimeout * time.Second)
		logrus.Debugf("building plugin time-out time is reach")
		version, err := db.GetManager().TenantPluginBuildVersionDao().GetBuildVersionByVersionID(tb.PluginID, tb.VersionID)
		if err != nil {
			logrus.Errorf("get version error, %v", err)
		}
		if version.Status != "complete" {
			version.Status = "timeout"
			if err := db.GetManager().TenantPluginBuildVersionDao().UpdateModel(version); err != nil {
				logrus.Errorf("update version error, %v", err)
			}
			logger.Info("插件构建超时，修改插件状态失败", map[string]string{"step": "callback", "status": "failure"})
		}
	}()
	go func() {
		logrus.Info("start exec build plugin from image worker")
		defer event.GetManager().ReleaseLogger(logger)
		for retry := 0; retry < 2; retry++ {
			err := e.run(&tb, config, logger)
			if err != nil {
				logrus.Errorf("exec plugin build from image error:%s", err.Error())
				logger.Info("镜像构建插件任务执行失败，开始重试", map[string]string{"step": "builder-exector", "status": "failure"})
			} else {
				return
			}
		}
		version, err := db.GetManager().TenantPluginBuildVersionDao().GetBuildVersionByVersionID(tb.PluginID, tb.VersionID)
		if err != nil {
			logrus.Errorf("get version error, %v", err)
		}
		version.Status = "failure"
		if err := db.GetManager().TenantPluginBuildVersionDao().UpdateModel(version); err != nil {
			logrus.Errorf("update version error, %v", err)
		}
		logger.Info("镜像构建插件任务执行失败", map[string]string{"step": "callback", "status": "failure"})
	}()
}

func checkConf(confPath string) error {
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		return fmt.Errorf("config.json is not exist")
	}
	return nil
}

func getConf(confPath string) parseConfig.Config {
	return parseConfig.New(confPath)
}

func (e *exectorManager) run(t *model.BuildPluginTaskBody, c parseConfig.Config, logger event.Logger) error {

	if _, err := sources.ImagePull(e.DockerClient, t.ImageURL, types.ImagePullOptions{}, logger, 5); err != nil {
		logrus.Errorf("pull image %v error, %v", t.ImageURL, err)
		logger.Error("拉取镜像失败", map[string]string{"step": "builder-exector", "status": "failure"})
		return err
	}

	logger.Info("拉取镜像完成", map[string]string{"step": "build-exector", "status": "complete"})
	newTag := createTag(t.ImageURL, t.PluginID)
	err := sources.ImageTag(e.DockerClient, t.ImageURL, newTag, logger, 1)
	if err != nil {
		logrus.Errorf("set plugin image tag error, %v", err)
		logger.Error("修改镜像tag失败", map[string]string{"step": "builder-exector", "status": "failure"})
		return err
	}
	logger.Info("修改镜像Tag完成", map[string]string{"step": "build-exector", "status": "complete"})
	if err := sources.ImagePush(e.DockerClient, newTag, "", "", logger, 5); err != nil {
		logrus.Errorf("push image %s error, %v", newTag, err)
		logger.Error("推送镜像失败", map[string]string{"step": "builder-exector", "status": "failure"})
		return err
	}

	version, err := db.GetManager().TenantPluginBuildVersionDao().GetBuildVersionByVersionID(t.PluginID, t.VersionID)
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

func createTag(image string, alias string) string {
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
	curImage := fmt.Sprintf("goodrain.me/%s:%s", iName, tag+"_"+alias)
	return curImage
}

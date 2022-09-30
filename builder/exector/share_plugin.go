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
	"time"

	"github.com/goodrain/rainbond/builder"

	"github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/event"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

//PluginShareItem PluginShareItem
type PluginShareItem struct {
	EventID        string `json:"event_id"`
	ImageName      string `json:"image_name"`
	LocalImageName string `json:"local_image_name"`
	ShareID        string `json:"share_id"`
	Logger         event.Logger
	ImageInfo      struct {
		HubURL      string `json:"hub_url"`
		HubUser     string `json:"hub_user"`
		HubPassword string `json:"hub_password"`
		Namespace   string `json:"namespace"`
		IsTrust     bool   `json:"is_trust,omitempty"`
	} `json:"image_info,omitempty"`
	ImageClient sources.ImageClient
	EtcdCli     *clientv3.Client
}

func init() {
	RegisterWorker("share-plugin", SharePluginItemCreater)
}

//SharePluginItemCreater create
func SharePluginItemCreater(in []byte, m *exectorManager) (TaskWorker, error) {
	eventID := gjson.GetBytes(in, "event_id").String()
	logger := event.GetManager().GetLogger(eventID)
	pluginShare := &PluginShareItem{
		Logger:  logger,
		EventID: eventID,

		ImageClient: m.imageClient,
		EtcdCli:     m.EtcdCli,
	}
	if err := ffjson.Unmarshal(in, &pluginShare); err != nil {
		return nil, err
	}
	return pluginShare, nil
}

//Run Run
func (i *PluginShareItem) Run(timeout time.Duration) error {
	_, err := i.ImageClient.ImagePull(i.LocalImageName, builder.REGISTRYUSER, builder.REGISTRYPASS, i.Logger, 10)
	if err != nil {
		logrus.Errorf("pull image %s error: %s", i.LocalImageName, err.Error())
		i.Logger.Error(fmt.Sprintf("拉取应用镜像: %s失败", i.LocalImageName), map[string]string{"step": "builder-exector", "status": "failure"})
		return err
	}
	if err := i.ImageClient.ImageTag(i.LocalImageName, i.ImageName, i.Logger, 1); err != nil {
		logrus.Errorf("change image tag error: %s", err.Error())
		i.Logger.Error(fmt.Sprintf("修改镜像tag: %s -> %s 失败", i.LocalImageName, i.ImageName), map[string]string{"step": "builder-exector", "status": "failure"})
		return err
	}
	user, pass := builder.GetImageUserInfoV2(i.ImageName, i.ImageInfo.HubUser, i.ImageInfo.HubPassword)
	if i.ImageInfo.IsTrust {
		err = i.ImageClient.TrustedImagePush(i.ImageName, user, pass, i.Logger, 10)
	} else {
		err = i.ImageClient.ImagePush(i.ImageName, user, pass, i.Logger, 10)
	}
	if err != nil {
		if err.Error() == "authentication required" {
			i.Logger.Error("镜像仓库授权失败", map[string]string{"step": "builder-exector", "status": "failure"})
			return err
		}
		logrus.Errorf("push image into registry error: %s", err.Error())
		i.Logger.Error("推送镜像至镜像仓库失败", map[string]string{"step": "builder-exector", "status": "failure"})
		return err
	}
	return i.updateShareStatus("success")
}

//Stop
func (i *PluginShareItem) Stop() error {
	return nil
}

//Name return worker name
func (i *PluginShareItem) Name() string {
	return "share-plugin"
}

//GetLogger GetLogger
func (i *PluginShareItem) GetLogger() event.Logger {
	return i.Logger
}

//ErrorCallBack if run error will callback
func (i *PluginShareItem) ErrorCallBack(err error) {
	i.updateShareStatus("failure")
}

//updateShareStatus update share task result
func (i *PluginShareItem) updateShareStatus(status string) error {
	var ss = ShareStatus{
		ShareID: i.ShareID,
		Status:  status,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := i.EtcdCli.Put(ctx, fmt.Sprintf("/rainbond/shareresult/%s", i.ShareID), ss.String())
	if err != nil {
		logrus.Errorf("put shareresult  %s into etcd error, %v", i.ShareID, err)
		i.Logger.Error("存储分享结果失败。", map[string]string{"step": "callback", "status": "failure"})
	}
	if status == "success" {
		i.Logger.Info("创建分享结果成功,分享成功", map[string]string{"step": "last", "status": "success"})
	} else {
		i.Logger.Info("创建分享结果成功,分享失败", map[string]string{"step": "callback", "status": "failure"})
	}
	return nil
}

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
	"github.com/goodrain/rainbond/builder"

	"github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/event"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/sirupsen/logrus"
)

//ImageShareItem ImageShareItem
type ImageShareItem struct {
	Namespace          string `json:"namespace"`
	TenantName         string `json:"tenant_name"`
	ServiceID          string `json:"service_id"`
	ServiceAlias       string `json:"service_alias"`
	ImageName          string `json:"image_name"`
	LocalImageName     string `json:"local_image_name"`
	LocalImageUsername string `json:"-"`
	LocalImagePassword string `json:"-"`
	ShareID            string `json:"share_id"`
	Logger             event.Logger
	ShareInfo          struct {
		ServiceKey string `json:"service_key" `
		AppVersion string `json:"app_version" `
		EventID    string `json:"event_id"`
		ShareUser  string `json:"share_user"`
		ShareScope string `json:"share_scope"`
		ImageInfo  struct {
			HubURL      string `json:"hub_url"`
			HubUser     string `json:"hub_user"`
			HubPassword string `json:"hub_password"`
			Namespace   string `json:"namespace"`
			IsTrust     bool   `json:"is_trust,omitempty"`
		} `json:"image_info,omitempty"`
	} `json:"share_info"`
	//DockerClient     *client.Client
	//ContainerdClient *containerd.Client
	ImageClient sources.ImageClient
	EtcdCli     *clientv3.Client
}

//NewImageShareItem 创建实体
func NewImageShareItem(in []byte, imageClient sources.ImageClient, EtcdCli *clientv3.Client) (*ImageShareItem, error) {
	var isi ImageShareItem
	if err := ffjson.Unmarshal(in, &isi); err != nil {
		return nil, err
	}
	isi.LocalImageUsername = builder.REGISTRYUSER
	isi.LocalImagePassword = builder.REGISTRYPASS
	eventID := isi.ShareInfo.EventID
	isi.Logger = event.GetManager().GetLogger(eventID)
	isi.ImageClient = imageClient
	isi.EtcdCli = EtcdCli
	return &isi, nil
}

//ShareService ShareService
func (i *ImageShareItem) ShareService() error {
	hubuser, hubpass := builder.GetImageUserInfoV2(i.LocalImageName, i.LocalImageUsername, i.LocalImagePassword)
	_, err := i.ImageClient.ImagePull(i.LocalImageName, hubuser, hubpass, i.Logger, 20)
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
	user, pass := builder.GetImageUserInfoV2(i.ImageName, i.ShareInfo.ImageInfo.HubUser, i.ShareInfo.ImageInfo.HubPassword)
	if i.ShareInfo.ImageInfo.IsTrust {
		err = i.ImageClient.TrustedImagePush(i.ImageName, user, pass, i.Logger, 30)
	} else {
		err = i.ImageClient.ImagePush(i.ImageName, user, pass, i.Logger, 30)
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
	return nil
}

//ShareStatus share status result
//ShareStatus share status result
type ShareStatus struct {
	ShareID string `json:"share_id,omitempty"`
	Status  string `json:"status,omitempty"`
}

func (s ShareStatus) String() string {
	b, _ := ffjson.Marshal(s)
	return string(b)
}

//UpdateShareStatus 更新任务执行结果
func (i *ImageShareItem) UpdateShareStatus(status string) error {
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


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
	"github.com/Sirupsen/logrus"
	"time"
	"fmt"
	"github.com/goodrain/rainbond/pkg/event"
	"github.com/tidwall/gjson"
	"github.com/docker/engine-api/client"
	"github.com/akkuman/parseConfig"
	"github.com/goodrain/rainbond/pkg/builder/sources"
	"github.com/docker/engine-api/types"
	dbmodel "github.com/goodrain/rainbond/pkg/db/model"
	"github.com/goodrain/rainbond/pkg/db"
)

//ImageShareItem ImageShareItem
type ImageShareItem struct {
	Namespace 		string `json:"namespace"`
	TenantName 		string `json:"tenant_name"`
	ServiceAlias 	string `json:"service_alias"`
	Image 			string `json:"image"`
	DestImage 		string `json:"dest_image"`
	Logger 			event.Logger `json:"logger"`
	EventID	 		string `json:"event_id"`
	DockerClient    *client.Client	
	Config          parseConfig.Config
	TenantID        string
	ServiceID 		string
	DeployVersion   string
	ShareID 		string
	ServiceKey      string
	AppVersion      string
	HubImage 		string
	ImageConf       ImageConf
}

//ImageConf ImageConf
type ImageConf struct {
	OuterRegistry string
}


//NewImageShareItem 创建实体
func NewImageShareItem(in []byte) *ImageShareItem {
	eventID := gjson.GetBytes(in, "event_id").String()
	logger := event.GetManager().GetLogger(eventID)
	ic := ImageConf {
		OuterRegistry: gjson.GetBytes(in, "share_conf.outer_registry").String(),
	}
	return &ImageShareItem{
		Namespace: gjson.GetBytes(in, "namespace").String(),
		TenantName:  gjson.GetBytes(in, "tenant_name").String(),
		ServiceAlias: gjson.GetBytes(in, "service_alias").String(),
		Image: gjson.GetBytes(in, "image").String(),
		DeployVersion: gjson.GetBytes(in, "deploy_version").String(),
		ServiceKey: gjson.GetBytes(in, "service_key").String(),
		AppVersion: gjson.GetBytes(in, "app_version").String(),	
		Logger: logger,
		EventID: eventID,
		ShareID: gjson.GetBytes(in, "share_id").String(),
		Config: GetBuilderConfig(),
		ImageConf: ic,
	}
}

//Run Run
func (i *ImageShareItem) Run(timeout time.Duration) error {
	if err := i.ShareToYS(); err != nil {
		return err
	}
	return nil
}

//ShareToYS ShareToYS
func (i *ImageShareItem) ShareToYS() error {
	_, err := sources.ImagePull(i.DockerClient, i.Image, types.ImagePullOptions{}, i.Logger, 3)
	if err != nil {
		logrus.Errorf("pull image %s error: %s", i.Image, err.Error())
		i.Logger.Error(fmt.Sprintf("拉取镜像: %s失败， %s", i.Image, err.Error()), map[string]string{"step": "builder-exector", "status":"failure"})
		return err
	}
	hubImage := i.RenameImage(i.Image)
	if err := sources.ImageTag(i.DockerClient, i.Image, hubImage, i.Logger, 1); err != nil {
		logrus.Errorf("change image tag error: %s", err.Error())
		i.Logger.Error(fmt.Sprintf("修改镜像tag: %s -> %s 失败", i.Image, hubImage), map[string]string{"step": "builder-exector", "status": "failure"})
		return err
	}
	i.HubImage = hubImage
	err = sources.ImagePush(i.DockerClient, hubImage, types.ImagePushOptions{}, i.Logger, 2)
	if err != nil {
		logrus.Errorf("push image into registry error: %s", err.Error())
		i.Logger.Error("推送镜像至镜像仓库失败", map[string]string{"step": "builder-exector", "status":"failure"})
		return err
	}
	return nil
}

//RenameImage RenameImage
func (i *ImageShareItem)RenameImage(image string) string {
	im := sources.ImageNameHandle(image)
	hubImage := fmt.Sprintf("%s/%s:%s", i.ImageConf.OuterRegistry, im.Name, im.Tag)
	logrus.Debugf("hub image is %s", hubImage)
	return hubImage
}

//UpdateShareStatus 更新任务执行结果
func (i *ImageShareItem) UpdateShareStatus(status string) error {
	result := &dbmodel.AppPublish{
		ServiceKey: i.ServiceKey,
		AppVersion: i.AppVersion,
		Image: i.HubImage,
		Status: status,
	}
	if err := db.GetManager().AppPublishDao().AddModel(result); err != nil {
		return err
	}
	return nil
} 
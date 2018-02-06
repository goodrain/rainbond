
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
	"time"
	"github.com/goodrain/rainbond/pkg/event"
	"github.com/tidwall/gjson"
	"github.com/docker/docker/client"
	"github.com/akkuman/parseConfig"
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
	Dest  			string
	ShareID 		string
	ImageConf       ImageConf
}

//ImageConf ImageConf
type ImageConf struct {
	InnerRegistry string
	OuterRegistry string
}


//NewImageShareItem 创建实体
func NewImageShareItem(in []byte) *ImageShareItem {
	eventID := gjson.GetBytes(in, "event_id").String()
	logger := event.GetManager().GetLogger(eventID)
	ic := ImageConf {
		InnerRegistry: gjson.GetBytes(in, "share_conf.inner_registry").String(),
		OuterRegistry: gjson.GetBytes(in, "share_conf.outer_registry").String(),
	}
	return &ImageShareItem{
		Namespace: gjson.GetBytes(in, "namespace").String(),
		TenantName:  gjson.GetBytes(in, "tenant_name").String(),
		ServiceAlias: gjson.GetBytes(in, "service_alias").String(),
		Image: gjson.GetBytes(in, "image").String(),
		Dest: gjson.GetBytes(in, "dest").String(),
		DeployVersion: gjson.GetBytes(in, "deploy_version").String(),
		Logger: logger,
		EventID: eventID,
		ShareID: gjson.GetBytes(in, "share_id").String(),
		Config: GetBuilderConfig(),
		ImageConf: ic,
	}
}

//Run Run
func (i *ImageShareItem) Run(timeout time.Duration) error {
	switch i.Dest {
	case "ys":
		if err := i.ShareToYS(); err != nil {
			return err
		}
	case "yb":
		if err := i.ShareToYB(); err != nil {
			return err
		}
	default:
		if err := i.ShareToYS(); err != nil {
			return err
		}
	}
	return nil
}

//ShareToYS ShareToYS
func (i *ImageShareItem) ShareToYS() error {
	return nil
}

//ShareToYB ShareToYB
func (i *ImageShareItem) ShareToYB() error {
	return nil
}


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

package share

import (
	"fmt"

	"github.com/goodrain/rainbond/mq/client"

	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/builder/exector"
	"github.com/goodrain/rainbond/db"
	"github.com/google/uuid"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/sirupsen/logrus"
)

// PluginShareHandle plugin share
type PluginShareHandle struct {
	MQClient client.MQClient
}

// PluginResult share plugin api return
type PluginResult struct {
	EventID   string `json:"event_id"`
	ShareID   string `json:"share_id"`
	ImageName string `json:"image_name"`
}

// PluginShare PluginShare
type PluginShare struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	TenantID   string
	// in: path
	// required: true
	PluginID string `json:"plugin_id"`
	//in: body
	Body struct {
		//in: body
		//应用分享Key
		PluginKey     string `json:"plugin_key" validate:"plugin_key|required"`
		PluginVersion string `json:"plugin_version" validate:"plugin_version|required"`
		EventID       string `json:"event_id"`
		ShareUser     string `json:"share_user"`
		ShareScope    string `json:"share_scope"`
		ImageInfo     struct {
			HubURL      string `json:"hub_url"`
			HubUser     string `json:"hub_user"`
			HubPassword string `json:"hub_password"`
			Namespace   string `json:"namespace"`
			IsTrust     bool   `json:"is_trust,omitempty" validate:"is_trust"`
		} `json:"image_info,omitempty"`
	}
}

// Share share app
func (s *PluginShareHandle) Share(ss PluginShare) (*PluginResult, *util.APIHandleError) {
	_, err := db.GetManager().TenantPluginDao().GetPluginByID(ss.PluginID, ss.TenantID)
	if err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("query plugin error", err)
	}
	//query new build version
	version, err := db.GetManager().TenantPluginBuildVersionDao().GetLastBuildVersionByVersionID(ss.PluginID, ss.Body.PluginVersion)
	if err != nil {
		logrus.Error("query service deploy version error", err.Error())
		return nil, util.CreateAPIHandleErrorFromDBError("query plugin version error", err)
	}
	shareID := uuid.New().String()
	shareImageName, err := version.CreateShareImage(ss.Body.ImageInfo.HubURL, ss.Body.ImageInfo.Namespace)
	if err != nil {
		return nil, util.CreateAPIHandleErrorf(500, "create share image name error:%s", err.Error())
	}
	info := map[string]interface{}{
		"image_info":       ss.Body.ImageInfo,
		"event_id":         ss.Body.EventID,
		"tenant_name":      ss.TenantName,
		"image_name":       shareImageName,
		"share_id":         shareID,
		"local_image_name": version.BuildLocalImage,
	}
	err = s.MQClient.SendBuilderTopic(client.TaskStruct{
		TaskType: "share-plugin",
		TaskBody: info,
		Topic:    client.BuilderTopic,
	})
	if err != nil {
		logrus.Errorf("equque mq error, %v", err)
		return nil, util.CreateAPIHandleError(502, err)
	}
	return &PluginResult{EventID: ss.Body.EventID, ShareID: shareID, ImageName: shareImageName}, nil
}

// ShareResult 分享应用结果查询
func (s *PluginShareHandle) ShareResult(shareID string) (i exector.ShareStatus, e *util.APIHandleError) {
	res, err := db.GetManager().KeyValueDao().Get(fmt.Sprintf("/rainbond/shareresult/%s", shareID))
	if err != nil {
		return exector.ShareStatus{}, nil
	}
	if err != nil {
		e = util.CreateAPIHandleError(500, err)
	} else {
		if res == nil {
			i.ShareID = shareID
		} else {
			if err := ffjson.Unmarshal([]byte(res.V), &i); err != nil {
				return i, util.CreateAPIHandleError(500, err)
			}
		}
	}
	return
}

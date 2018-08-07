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
	"context"
	"fmt"

	"github.com/goodrain/rainbond/builder/exector"

	"github.com/twinj/uuid"

	"github.com/pquerna/ffjson/ffjson"

	"github.com/goodrain/rainbond/db"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/clientv3"
	api_db "github.com/goodrain/rainbond/api/db"
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/mq/api/grpc/pb"
)

//ServiceShareHandle service share
type ServiceShareHandle struct {
	MQClient pb.TaskQueueClient
	EtcdCli  *clientv3.Client
}

//APIResult 分享接口返回
type APIResult struct {
	EventID   string `json:"event_id"`
	ShareID   string `json:"share_id"`
	ImageName string `json:"image_name,omitempty"`
	SlugPath  string `json:"slug_path,omitempty"`
}

//Share 分享应用
func (s *ServiceShareHandle) Share(serviceID string, ss api_model.ServiceShare) (*APIResult, *util.APIHandleError) {
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("查询应用出错", err)
	}
	//查询部署版本
	version, err := db.GetManager().VersionInfoDao().GetVersionByDeployVersion(service.DeployVersion, serviceID)
	if err != nil {
		logrus.Error("query service deploy version error", err.Error())
	}
	shareID := uuid.NewV4().String()
	var slugPath, shareImageName string
	var bs api_db.BuildTaskStruct
	if version.DeliveredType == "slug" {
		shareSlugInfo := ss.Body.SlugInfo
		slugPath = service.CreateShareSlug(ss.Body.ServiceKey, shareSlugInfo.Namespace, ss.Body.AppVersion)
		if ss.Body.SlugInfo.FTPHost == "" {
			slugPath = fmt.Sprintf("/grdata/build/tenant/app_publish/%s", slugPath)
		}
		info := map[string]interface{}{
			"service_alias": ss.ServiceAlias,
			"service_id":    serviceID,
			"tenant_name":   ss.TenantName,
			"share_info":    ss.Body,
			"slug_path":     slugPath,
			"share_id":      shareID,
		}
		if version != nil && version.DeliveredPath != "" {
			info["local_slug_path"] = version.DeliveredPath
		} else {
			info["local_slug_path"] = fmt.Sprintf("/grdata/build/tenant/%s/slug/%s/%s.tgz", service.TenantID, service.ServiceID, service.DeployVersion)
		}
		body, _ := ffjson.Marshal(info)
		bs.TaskType = "share-slug"
		bs.TaskBody = body
		bs.User = ss.Body.ShareUser
	} else {
		shareImageInfo := ss.Body.ImageInfo
		shareImageName, err = service.CreateShareImage(shareImageInfo.HubURL, shareImageInfo.Namespace, ss.Body.AppVersion)
		if err != nil {
			return nil, util.CreateAPIHandleError(500, err)
		}
		info := map[string]interface{}{
			"share_info":       ss.Body,
			"service_alias":    ss.ServiceAlias,
			"service_id":       serviceID,
			"tenant_name":      ss.TenantName,
			"image_name":       shareImageName,
			"share_id":         shareID,
			"local_image_name": service.ImageName,
		}
		if version != nil && version.DeliveredPath != "" {
			info["local_image_name"] = version.DeliveredPath
		}
		body, _ := ffjson.Marshal(info)
		bs.TaskType = "share-image"
		bs.TaskBody = body
		bs.User = ss.Body.ShareUser
	}
	eq, _ := api_db.BuildTaskBuild(&bs)
	ctx, cancel := context.WithCancel(context.Background())
	_, err = s.MQClient.Enqueue(ctx, eq)
	cancel()
	if err != nil {
		logrus.Errorf("equque mq error, %v", err)
		return nil, util.CreateAPIHandleError(502, err)
	}
	return &APIResult{EventID: ss.Body.EventID, ShareID: shareID, ImageName: shareImageName, SlugPath: slugPath}, nil
}

//ShareResult 分享应用结果查询
func (s *ServiceShareHandle) ShareResult(shareID string) (i exector.ShareStatus, e *util.APIHandleError) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	res, err := s.EtcdCli.Get(ctx, fmt.Sprintf("/rainbond/shareresult/%s", shareID))
	if err != nil {
		e = util.CreateAPIHandleError(500, err)
	} else {
		if res.Count == 0 {
			i.ShareID = shareID
		} else {
			if err := ffjson.Unmarshal(res.Kvs[0].Value, &i); err != nil {
				return i, util.CreateAPIHandleError(500, err)
			}
		}
	}
	return
}

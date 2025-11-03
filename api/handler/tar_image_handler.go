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

package handler

import (
	"encoding/json"
	"fmt"

	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/mq/client"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// TarImageHandle tar包镜像处理
type TarImageHandle struct {
	MQClient client.MQClient
}

var tarImageHandle *TarImageHandle

// GetTarImageHandle 获取tar镜像处理handler
func GetTarImageHandle() *TarImageHandle {
	if tarImageHandle == nil {
		logrus.Error("TarImageHandle is not initialized")
	}
	return tarImageHandle
}

// CreateTarImageHandle 创建tar镜像处理handler
func CreateTarImageHandle(mqClient client.MQClient) {
	tarImageHandle = &TarImageHandle{
		MQClient: mqClient,
	}
}

// LoadTarImage 开始异步解析tar包镜像
func (t *TarImageHandle) LoadTarImage(tenantID string, req model.LoadTarImageReq) (*model.LoadTarImageResp, *util.APIHandleError) {
	loadID := uuid.New().String()

	logrus.Infof("[LoadTarImage] Starting, load_id: %s, tenant_id: %s, event_id: %s, tar_file_path: %s",
		loadID, tenantID, req.EventID, req.TarFilePath)

	// 构建任务发送到MQ
	task := client.TaskStruct{
		Topic:    "builder",
		TaskType: "load-tar-image",
		TaskBody: map[string]interface{}{
			"event_id":      req.EventID,
			"tar_file_path": req.TarFilePath,
			"load_id":       loadID,
			"tenant_id":     tenantID,
		},
	}

	logrus.Infof("[LoadTarImage] Sending task to MQ, topic: %s, task_type: %s, task_body: %+v",
		task.Topic, task.TaskType, task.TaskBody)

	// 发送任务到消息队列
	err := t.MQClient.SendBuilderTopic(task)
	if err != nil {
		logrus.Errorf("[LoadTarImage] Failed to send task to MQ: %v", err)
		return nil, util.CreateAPIHandleError(500, fmt.Errorf("启动解析任务失败"))
	}

	logrus.Infof("[LoadTarImage] Task sent successfully to MQ, load_id: %s, event_id: %s", loadID, req.EventID)

	return &model.LoadTarImageResp{
		LoadID: loadID,
		Status: "loading",
	}, nil
}

// GetTarLoadResult 查询tar包解析结果
func (t *TarImageHandle) GetTarLoadResult(loadID string) (*model.TarLoadResult, *util.APIHandleError) {
	// 从etcd查询结果
	key := fmt.Sprintf("/rainbond/tarload/%s", loadID)
	logrus.Infof("[GetTarLoadResult] Querying etcd for load_id: %s, key: %s", loadID, key)

	res, err := db.GetManager().KeyValueDao().Get(key)
	if err != nil || res == nil {
		if err != nil {
			logrus.Infof("[GetTarLoadResult] Key not found in etcd (task may still be processing): load_id: %s, error: %v", loadID, err)
		} else {
			logrus.Infof("[GetTarLoadResult] Result is nil for load_id: %s (task may still be processing)", loadID)
		}
		// 如果没找到结果，可能还在处理中
		return &model.TarLoadResult{
			LoadID:  loadID,
			Status:  "loading",
			Message: "正在解析中...",
		}, nil
	}

	logrus.Infof("[GetTarLoadResult] Found result in etcd for load_id: %s, value length: %d bytes", loadID, len(res.V))

	// 解析结果
	var result model.TarLoadResult
	if err := json.Unmarshal([]byte(res.V), &result); err != nil {
		logrus.Errorf("[GetTarLoadResult] Failed to decode result for load_id: %s, error: %v, raw_value: %s", loadID, err, res.V)
		return nil, util.CreateAPIHandleError(500, fmt.Errorf("解析结果格式错误"))
	}

	logrus.Infof("[GetTarLoadResult] Successfully decoded result for load_id: %s, status: %s", loadID, result.Status)

	return &result, nil
}

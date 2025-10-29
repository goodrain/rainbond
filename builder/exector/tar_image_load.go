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
	"encoding/json"
	"fmt"

	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/event"
	pb "github.com/goodrain/rainbond/mq/api/grpc/pb"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/sirupsen/logrus"
)

// TarImageLoadTaskBody tar包镜像加载任务体
type TarImageLoadTaskBody struct {
	EventID     string `json:"event_id"`
	TarFilePath string `json:"tar_file_path"`
	LoadID      string `json:"load_id"`
	TenantID    string `json:"tenant_id"`
}

// loadTarImage 执行tar包镜像加载任务
func (e *exectorManager) loadTarImage(task *pb.TaskMessage) {
	logrus.Infof("[LoadTarImage] Received task from MQ, task_id: %s, task_body_length: %d", task.TaskId, len(task.TaskBody))

	// 1. 解析任务体
	var taskBody TarImageLoadTaskBody
	if err := ffjson.Unmarshal(task.TaskBody, &taskBody); err != nil {
		logrus.Errorf("[LoadTarImage] Failed to unmarshal task body: %v, raw_body: %s", err, string(task.TaskBody))
		return
	}

	logrus.Infof("[LoadTarImage] Parsed task body: load_id=%s, event_id=%s, tenant_id=%s, tar_file_path=%s",
		taskBody.LoadID, taskBody.EventID, taskBody.TenantID, taskBody.TarFilePath)

	// 2. 获取event logger
	logger := event.GetManager().GetLogger(taskBody.EventID)
	logger.Info("开始解析tar包镜像", map[string]string{"step": "builder-exector", "status": "starting"})

	defer event.GetManager().ReleaseLogger(logger)

	var status string
	var images []string
	var message string
	var metadata map[string]model.ImageMetadata

	// 3. 执行镜像加载
	logger.Info("正在从tar包加载镜像...", map[string]string{"step": "load-tar", "status": "loading"})
	logrus.Infof("[LoadTarImage] Starting to load images from tar file: %s", taskBody.TarFilePath)

	imageNames, err := e.imageClient.ImageLoad(taskBody.TarFilePath, logger)
	if err != nil {
		logrus.Errorf("[LoadTarImage] Failed to load images: %v", err)
		status = "failure"
		message = fmt.Sprintf("加载镜像失败: %v", err)
		logger.Error("tar包镜像加载失败", map[string]string{"step": "load-tar", "status": "failure"})
	} else {
		logrus.Infof("[LoadTarImage] Successfully loaded %d images: %v", len(imageNames), imageNames)
		status = "success"
		images = imageNames
		message = fmt.Sprintf("成功加载%d个镜像", len(imageNames))
		metadata = make(map[string]model.ImageMetadata)

		// 获取镜像元数据
		for _, imageName := range imageNames {
			// 检查镜像是否存在并获取信息
			if imageRef, exists, err := e.imageClient.CheckIfImageExists(imageName); exists && err == nil {
				metadata[imageName] = model.ImageMetadata{
					Name:       imageName,
					RepoDigest: imageRef,
					// Size和CreatedAt可以通过其他API获取,这里简化处理
				}
			}
		}

		logger.Info(fmt.Sprintf("成功加载%d个镜像", len(imageNames)), map[string]string{"step": "load-tar", "status": "success"})
		for _, img := range imageNames {
			logger.Info(fmt.Sprintf("- %s", img), map[string]string{"step": "load-tar"})
		}
	}

	// 4. 保存结果到etcd
	result := model.TarLoadResult{
		LoadID:   taskBody.LoadID,
		Status:   status,
		Message:  message,
		Images:   images,
		Metadata: metadata,
	}

	resultJSON, _ := json.Marshal(result)
	key := fmt.Sprintf("/rainbond/tarload/%s", taskBody.LoadID)
	logrus.Infof("[LoadTarImage] Saving result to etcd, key: %s, status: %s, images_count: %d", key, status, len(images))

	if err := db.GetManager().KeyValueDao().Put(key, string(resultJSON)); err != nil {
		logrus.Errorf("[LoadTarImage] Failed to save result to etcd: %v", err)
		logger.Error("保存解析结果失败", map[string]string{"step": "save-result", "status": "failure"})
	} else {
		logrus.Infof("[LoadTarImage] Result saved successfully to etcd, key: %s", key)
		logger.Info("解析结果已保存", map[string]string{"step": "save-result", "status": "success"})
	}

	logrus.Infof("[LoadTarImage] Task completed, load_id: %s, final_status: %s", taskBody.LoadID, status)
	logger.Info("tar包镜像解析任务完成", map[string]string{"step": "last", "status": status})
}

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
	"strings"

	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/parser"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/mq/client"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// TarImageHandle tar包镜像处理
type TarImageHandle struct {
	MQClient    client.MQClient
	ImageClient sources.ImageClient
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
func CreateTarImageHandle(mqClient client.MQClient, imageClient sources.ImageClient) {
	tarImageHandle = &TarImageHandle{
		MQClient:    mqClient,
		ImageClient: imageClient,
	}
}

// LoadTarImage 开始异步解析tar包镜像
func (t *TarImageHandle) LoadTarImage(tenantID string, req model.LoadTarImageReq) (*model.LoadTarImageResp, *util.APIHandleError) {
	loadID := uuid.New().String()

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

	// 发送任务到消息队列
	err := t.MQClient.SendBuilderTopic(task)
	if err != nil {
		logrus.Errorf("send load tar image task to mq error: %v", err)
		return nil, util.CreateAPIHandleError(500, fmt.Errorf("启动解析任务失败"))
	}

	logrus.Infof("started load tar image task, load_id: %s, event_id: %s", loadID, req.EventID)

	return &model.LoadTarImageResp{
		LoadID: loadID,
		Status: "loading",
	}, nil
}

// GetTarLoadResult 查询tar包解析结果
func (t *TarImageHandle) GetTarLoadResult(loadID string) (*model.TarLoadResult, *util.APIHandleError) {
	// 从etcd查询结果
	key := fmt.Sprintf("/rainbond/tarload/%s", loadID)
	res, err := db.GetManager().KeyValueDao().Get(key)
	if err != nil {
		logrus.Errorf("get tar load result from etcd error: %v", err)
		// 如果没找到结果，可能还在处理中
		return &model.TarLoadResult{
			LoadID:  loadID,
			Status:  "loading",
			Message: "正在解析中...",
		}, nil
	}

	// 解析结果
	var result model.TarLoadResult
	if err := json.Unmarshal([]byte(res.V), &result); err != nil {
		logrus.Errorf("decode tar load result error: %v", err)
		return nil, util.CreateAPIHandleError(500, fmt.Errorf("解析结果格式错误"))
	}

	return &result, nil
}

// ImportTarImages 确认导入镜像到镜像仓库(同步执行)
func (t *TarImageHandle) ImportTarImages(tenantID, tenantName string, req model.ImportTarImagesReq) (*model.ImportTarImagesResp, *util.APIHandleError) {
	// 检查 ImageClient 是否可用
	if t.ImageClient == nil {
		return nil, util.CreateAPIHandleError(503, fmt.Errorf("image client is not available, cannot perform synchronous import"))
	}

	// 1. 获取解析结果
	loadResult, errH := t.GetTarLoadResult(req.LoadID)
	if errH != nil {
		return nil, errH
	}

	if loadResult.Status != "success" {
		return nil, util.CreateAPIHandleError(400, fmt.Errorf("tar包解析未完成或失败"))
	}

	// 2. 确定命名空间
	namespace := req.Namespace
	if namespace == "" {
		namespace = tenantName
	}

	// 3. 构建目标镜像名并执行导入
	var importedImages []model.ImportedImage
	var failedImages []model.FailedImage

	registryDomain := builder.REGISTRYDOMAIN
	registryUser := builder.REGISTRYUSER
	registryPass := builder.REGISTRYPASS

	for _, sourceImage := range req.Images {
		// 验证镜像是否在解析结果中
		found := false
		for _, img := range loadResult.Images {
			if img == sourceImage {
				found = true
				break
			}
		}
		if !found {
			failedImages = append(failedImages, model.FailedImage{
				SourceImage: sourceImage,
				Error:       "镜像不在解析结果中",
			})
			continue
		}

		// 构建目标镜像名
		targetImage := fmt.Sprintf("%s/%s/%s", registryDomain, namespace, getImageNameWithoutRegistry(sourceImage))

		// Tag镜像
		err := t.ImageClient.ImageTag(sourceImage, targetImage, nil, 3)
		if err != nil {
			logrus.Errorf("tag image %s to %s error: %v", sourceImage, targetImage, err)
			failedImages = append(failedImages, model.FailedImage{
				SourceImage: sourceImage,
				Error:       fmt.Sprintf("Tag镜像失败: %v", err),
			})
			continue
		}

		// Push镜像到仓库
		err = t.ImageClient.ImagePush(targetImage, registryUser, registryPass, nil, 30)
		if err != nil {
			logrus.Errorf("push image %s error: %v", targetImage, err)
			failedImages = append(failedImages, model.FailedImage{
				SourceImage: sourceImage,
				Error:       fmt.Sprintf("Push镜像失败: %v", err),
			})
			continue
		}

		importedImages = append(importedImages, model.ImportedImage{
			SourceImage: sourceImage,
			TargetImage: targetImage,
		})

		logrus.Infof("successfully imported tar image: %s -> %s", sourceImage, targetImage)
	}

	message := fmt.Sprintf("导入完成: 成功%d个, 失败%d个", len(importedImages), len(failedImages))

	return &model.ImportTarImagesResp{
		ImportedImages: importedImages,
		FailedImages:   failedImages,
		Message:        message,
	}, nil
}

// getImageNameWithoutRegistry 从完整镜像名中提取不包含registry的部分
// 例如: docker.io/library/nginx:latest -> library/nginx:latest
//      nginx:latest -> nginx:latest
func getImageNameWithoutRegistry(fullImageName string) string {
	image := parser.ParseImageName(fullImageName)

	// 获取repository路径(不包含registry)
	repo := image.GetRepostory()

	// 添加tag
	if image.Tag != "" {
		return repo + ":" + image.Tag
	}

	// 如果没有tag,检查原始镜像名是否包含@digest
	if strings.Contains(fullImageName, "@") {
		parts := strings.Split(fullImageName, "@")
		if len(parts) == 2 {
			// 保留digest
			return repo + "@" + parts[1]
		}
	}

	return repo
}

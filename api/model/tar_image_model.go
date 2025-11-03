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

package model

// LoadTarImageReq 加载tar包镜像请求
type LoadTarImageReq struct {
	EventID     string `json:"event_id" validate:"required"`
	TarFilePath string `json:"tar_file_path" validate:"required"`
}

// LoadTarImageResp 加载tar包镜像响应
type LoadTarImageResp struct {
	LoadID string `json:"load_id"`
	Status string `json:"status"` // loading
}

// TarLoadResult tar包解析结果
type TarLoadResult struct {
	LoadID        string                   `json:"load_id"`
	Status        string                   `json:"status"`         // loading, success, failure
	Message       string                   `json:"message"`        // 错误信息或提示
	Images        []string                 `json:"images,omitempty"`         // 原始镜像列表
	TargetImages  map[string]string        `json:"target_images,omitempty"`  // 目标镜像映射: 原始镜像名 -> 目标镜像名
	Metadata      map[string]ImageMetadata `json:"metadata,omitempty"`       // 镜像元数据
}

// ImageMetadata 镜像元数据
type ImageMetadata struct {
	Name       string `json:"name"`        // 镜像名称
	Size       int64  `json:"size"`        // 镜像大小(字节)
	CreatedAt  string `json:"created_at"`  // 创建时间
	RepoDigest string `json:"repo_digest"` // 镜像digest
}

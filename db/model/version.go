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

import (
	"fmt"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/sirupsen/logrus"
)

//VersionInfo version info struct
type VersionInfo struct {
	Model
	BuildVersion string `gorm:"column:build_version;size:40" json:"build_version"` //唯一
	EventID      string `gorm:"column:event_id;size:40;index:event_id" json:"event_id"`
	ServiceID    string `gorm:"column:service_id;size:40;index:service_id" json:"service_id"`
	Kind         string `gorm:"column:kind;size:40" json:"kind"` //kind
	//DeliveredType app version delivered type
	//image: this is a docker image
	//slug: this is a source code tar file
	DeliveredType string `gorm:"column:delivered_type;size:40" json:"delivered_type"`  //kind
	DeliveredPath string `gorm:"column:delivered_path;size:250" json:"delivered_path"` //交付物path
	ImageName     string `gorm:"column:image_name;size:250" json:"image_name"`         //运行镜像名称
	Cmd           string `gorm:"column:cmd;size:2048" json:"cmd"`                      //启动命令
	RepoURL       string `gorm:"column:repo_url;size:2047" json:"repo_url"`
	CodeVersion   string `gorm:"column:code_version;size:40" json:"code_version"`
	CodeBranch    string `gorm:"column:code_branch;size:40" json:"code_branch"`
	CommitMsg     string `gorm:"column:code_commit_msg;size:1024" json:"code_commit_msg"`
	Author        string `gorm:"column:code_commit_author;size:40" json:"code_commit_author"`
	//FinalStatus app version status
	//success: version available
	//failure: build failure
	//lost: there is no delivered
	FinalStatus string    `gorm:"column:final_status;size:40" json:"final_status"`
	FinishTime  time.Time `gorm:"column:finish_time;" json:"finish_time"`
	PlanVersion string  `gorm:"column:plan_version;size:250" json:"plan_version"`
}

//TableName 表名
func (t *VersionInfo) TableName() string {
	return "tenant_service_version"
}

//CreateShareImage create share image name
func (t *VersionInfo) CreateShareImage(hubURL, namespace, appVersion string) (string, error) {
	_, err := reference.ParseAnyReference(t.DeliveredPath)
	if err != nil {
		logrus.Errorf("reference image error: %s", err.Error())
		return "", err
	}
	image := ParseImage(t.DeliveredPath)
	if hubURL != "" {
		image.Host = hubURL
	}
	if namespace != "" {
		image.Namespace = namespace
	}
	image.Name = fmt.Sprintf("%s:%s", t.ServiceID, t.BuildVersion)
	return image.String(), nil
}

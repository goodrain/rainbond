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
	"fmt"
	"github.com/goodrain/rainbond/pkg/component/storage"
	"os"

	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/event"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/sirupsen/logrus"

	//"github.com/docker/docker/api/types"

	//"github.com/docker/docker/client"

	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
)

// MarketSlugItem MarketSlugItem
type MarketSlugItem struct {
	TenantName    string       `json:"tenant_name"`
	ServiceAlias  string       `json:"service_alias"`
	Logger        event.Logger `json:"logger"`
	EventID       string       `json:"event_id"`
	Operator      string       `json:"operator"`
	DeployVersion string       `json:"deploy_version"`
	TenantID      string       `json:"tenant_id"`
	ServiceID     string       `json:"service_id"`
	Action        string       `json:"action"`
	TGZPath       string
	Configs       map[string]string `json:"configs"`
	SlugInfo      struct {
		SlugPath    string `json:"slug_path"`
		FTPHost     string `json:"ftp_host"`
		FTPPort     string `json:"ftp_port"`
		FTPUser     string `json:"ftp_username"`
		FTPPassword string `json:"ftp_password"`
	} `json:"slug_info"`
}

// NewMarketSlugItem 创建实体
func NewMarketSlugItem(in []byte) (*MarketSlugItem, error) {
	var msi MarketSlugItem
	if err := ffjson.Unmarshal(in, &msi); err != nil {
		return nil, err
	}
	msi.Logger = event.GetManager().GetLogger(msi.EventID)
	msi.TGZPath = fmt.Sprintf("/grdata/build/tenant/%s/slug/%s/%s.tgz", msi.TenantID, msi.ServiceID, msi.DeployVersion)
	return &msi, nil
}

// Run Run
func (i *MarketSlugItem) Run() error {
	if i.SlugInfo.FTPHost != "" && i.SlugInfo.FTPPort != "" {
		sFTPClient, err := sources.NewSFTPClient(i.SlugInfo.FTPUser, i.SlugInfo.FTPPassword, i.SlugInfo.FTPHost, i.SlugInfo.FTPPort)
		if err != nil {
			i.Logger.Error("创建FTP客户端失败", map[string]string{"step": "slug-share", "status": "failure"})
			return err
		}
		defer sFTPClient.Close()
		if err := sFTPClient.DownloadFile(i.SlugInfo.SlugPath, i.TGZPath, i.Logger); err != nil {
			i.Logger.Error("源码包远程FTP获取失败，安装失败", map[string]string{"step": "slug-share", "status": "failure"})
			logrus.Errorf("copy slug file error when build service, %s", err.Error())
			return nil
		}
	} else {
		if err := storage.Default().StorageCli.CopyFileWithProgress(i.SlugInfo.SlugPath, i.TGZPath, i.Logger); err != nil {
			i.Logger.Error("源码包本地获取失败，安装失败", map[string]string{"step": "slug-share", "status": "failure"})
			logrus.Errorf("copy slug file error when build service, %s", err.Error())
			return nil
		}
	}
	if err := os.Chown(i.TGZPath, 200, 200); err != nil {
		os.Remove(i.TGZPath)
		i.Logger.Error("源码包本地获取失败，安装失败", map[string]string{"step": "slug-share", "status": "failure"})
		logrus.Errorf("chown slug file error when build service, %s", err.Error())
		return nil
	}
	i.Logger.Info("应用构建完成", map[string]string{"step": "build-code", "status": "success"})
	vi := &dbmodel.VersionInfo{
		DeliveredType: "slug",
		DeliveredPath: i.TGZPath,
		EventID:       i.EventID,
		FinalStatus:   "success",
	}
	if err := i.UpdateVersionInfo(vi); err != nil {
		logrus.Errorf("update version info error: %s", err.Error())
		i.Logger.Error("更新应用版本信息失败", map[string]string{"step": "slug-share", "status": "failure"})
		return err
	}
	return nil
}

// UpdateVersionInfo 更新任务执行结果
func (i *MarketSlugItem) UpdateVersionInfo(vi *dbmodel.VersionInfo) error {
	version, err := db.GetManager().VersionInfoDao().GetVersionByDeployVersion(i.DeployVersion, i.ServiceID)
	if err != nil {
		return err
	}
	if vi.DeliveredType != "" {
		version.DeliveredType = vi.DeliveredType
	}
	if vi.DeliveredPath != "" {
		version.DeliveredPath = vi.DeliveredPath
	}
	if vi.EventID != "" {
		version.EventID = vi.EventID
	}
	if vi.FinalStatus != "" {
		version.FinalStatus = vi.FinalStatus
	}
	if vi.DeliveredType == "slug" {
		version.ImageName = builder.RUNNERIMAGENAME
	}
	if err := db.GetManager().VersionInfoDao().UpdateModel(version); err != nil {
		return err
	}
	return nil
}

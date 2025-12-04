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
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/pkg/component/storage"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/sirupsen/logrus"
)

// SlugShareItem SlugShareItem
type SlugShareItem struct {
	Namespace     string `json:"namespace"`
	TenantName    string `json:"tenant_name"`
	ServiceID     string `json:"service_id"`
	ServiceAlias  string `json:"service_alias"`
	SlugPath      string `json:"slug_path"`
	LocalSlugPath string `json:"local_slug_path"`
	ShareID       string `json:"share_id"`
	Logger        event.Logger
	ShareInfo     struct {
		ServiceKey string `json:"service_key" `
		AppVersion string `json:"app_version" `
		EventID    string `json:"event_id"`
		ShareUser  string `json:"share_user"`
		ShareScope string `json:"share_scope"`
		SlugInfo   struct {
			Namespace   string `json:"namespace"`
			FTPHost     string `json:"ftp_host"`
			FTPPort     string `json:"ftp_port"`
			FTPUser     string `json:"ftp_username"`
			FTPPassword string `json:"ftp_password"`
		} `json:"slug_info,omitempty"`
	} `json:"share_info"`
	PackageName string
}

// NewSlugShareItem 创建实体
func NewSlugShareItem(in []byte) (*SlugShareItem, error) {
	var ssi SlugShareItem
	if err := ffjson.Unmarshal(in, &ssi); err != nil {
		return nil, err
	}
	eventID := ssi.ShareInfo.EventID
	ssi.Logger = event.GetManager().GetLogger(eventID)
	return &ssi, nil
}

// ShareService  Run
func (i *SlugShareItem) ShareService() error {
	logrus.Debugf("share app local slug path: %s ,target path: %s", i.LocalSlugPath, i.SlugPath)
	if _, err := os.Stat(i.LocalSlugPath); err != nil {
		i.Logger.Error(util.Translation("Slug package not exist, please build first"), map[string]string{"step": "slug-share", "status": "failure"})
		return err
	}
	if i.ShareInfo.SlugInfo.FTPHost != "" && i.ShareInfo.SlugInfo.FTPPort != "" {
		if err := i.ShareToFTP(); err != nil {
			return err
		}
	} else {
		if err := i.ShareToLocal(); err != nil {
			return err
		}
	}
	return nil
}

func createMD5(packageName string) (string, error) {
	md5Path := packageName + ".md5"
	_, err := os.Stat(md5Path)
	if err == nil {
		//md5 file exist
		return md5Path, nil
	}
	f, err := exec.Command("md5sum", packageName).Output()
	if err != nil {
		return "", err
	}
	md5In := strings.Split(string(f), "")
	if err := ioutil.WriteFile(md5Path, []byte(md5In[0]), 0644); err != nil {
		return "", err
	}
	return md5Path, nil
}

// ShareToFTP - ShareToFTP
func (i *SlugShareItem) ShareToFTP() error {
	i.Logger.Info("开始上传应用介质到FTP服务器", map[string]string{"step": "slug-share"})
	sFTPClient, err := sources.NewSFTPClient(i.ShareInfo.SlugInfo.FTPUser, i.ShareInfo.SlugInfo.FTPPassword, i.ShareInfo.SlugInfo.FTPHost, i.ShareInfo.SlugInfo.FTPPort)
	if err != nil {
		i.Logger.Error(util.Translation("Create FTP client failed"), map[string]string{"step": "slug-share", "status": "failure"})
		return err
	}
	defer sFTPClient.Close()
	if err := sFTPClient.PushFile(i.LocalSlugPath, i.SlugPath, i.Logger); err != nil {
		i.Logger.Error(util.Translation("Upload slug package failed"), map[string]string{"step": "slug-share", "status": "failure"})
		return err
	}
	i.Logger.Info("分享云市远程服务器完成", map[string]string{"step": "slug-share", "status": "success"})
	return nil
}

// ShareToLocal - ShareToLocal
func (i *SlugShareItem) ShareToLocal() error {
	file := i.LocalSlugPath
	i.Logger.Info("开始分享应用到本地目录", map[string]string{"step": "slug-share"})
	md5, err := createMD5(file)
	if err != nil {
		i.Logger.Error(util.Translation("Generate MD5 failed"), map[string]string{"step": "slug-share", "status": "failure"})
		return err
	}
	if err := storage.Default().StorageCli.UploadFileToFile(i.LocalSlugPath, i.SlugPath, i.Logger); err != nil {
		os.Remove(i.SlugPath)
		logrus.Errorf("copy file to share path error: %s", err.Error())
		i.Logger.Error(util.Translation("Copy file failed"), map[string]string{"step": "slug-share", "status": "failure"})
		return err
	}
	if err := storage.Default().StorageCli.UploadFileToFile(md5, i.SlugPath+".md5", i.Logger); err != nil {
		os.Remove(i.SlugPath)
		os.Remove(i.SlugPath + ".md5")
		logrus.Errorf("copy file to share path error: %s", err.Error())
		i.Logger.Error(util.Translation("Copy MD5 file failed"), map[string]string{"step": "slug-share", "status": "failure"})
		return err
	}
	i.Logger.Info("分享数据中心本地完成", map[string]string{"step": "slug-share", "status": "success"})
	return nil
}

// UpdateShareStatus 更新任务执行结果
func (i *SlugShareItem) UpdateShareStatus(status string) error {
	var ss = ShareStatus{
		ShareID: i.ShareID,
		Status:  status,
	}
	err := db.GetManager().KeyValueDao().Put(fmt.Sprintf("/rainbond/shareresult/%s", i.ShareID), ss.String())
	if err != nil {
		logrus.Errorf("put shareresult  %s into etcd error, %v", i.ShareID, err)
		i.Logger.Error(util.Translation("Save share result failed"), map[string]string{"step": "callback", "status": "failure"})
	}
	if status == "success" {
		i.Logger.Info("创建分享结果成功,分享成功", map[string]string{"step": "last", "status": "success"})
	} else {
		i.Logger.Info("创建分享结果成功,分享失败", map[string]string{"step": "callback", "status": "failure"})
	}
	return nil
}

// CheckMD5FileExist CheckMD5FileExist
func (i *SlugShareItem) CheckMD5FileExist(md5path, packageName string) bool {
	return false
}

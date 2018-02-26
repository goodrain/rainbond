
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
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/pkg/builder/sources"
	"time"
	"fmt"
	"os"
	"crypto/md5"
	"io/ioutil"
	"github.com/goodrain/rainbond/pkg/event"
	"github.com/tidwall/gjson"
	"github.com/akkuman/parseConfig"
	"github.com/goodrain/rainbond/pkg/db"
	dbmodel "github.com/goodrain/rainbond/pkg/db/model"
)


//SlugShareItem SlugShareItem
type SlugShareItem struct {
	Namespace 		string `json:"namespace"`
	TenantName 		string 
	Action			string 
	Logger 			event.Logger 
	SourceDir		string 
	ServiceKey      string
	AppVersion      string
	ServiceID       string
	DeployVersion   string
	TenantID        string 
	ShareID 		string
	EventID	 		string
	IsOuter 		string
	Config          parseConfig.Config
	FTPConf 		SlugFTPConf
	PackageName     string
}

//SlugFTPConf SlugFTPConf
type SlugFTPConf struct {
	Username 		string
	Password		string
	Host			string
	Port 			int
	FTPNamespace    string
}

//NewSlugShareItem 创建实体
func NewSlugShareItem(in []byte) *SlugShareItem {
	eventID := gjson.GetBytes(in, "event_id").String()
	logger := event.GetManager().GetLogger(eventID)
	sf := SlugFTPConf {
		Username: gjson.GetBytes(in, "share_conf.ftp_username").String(),
		Password: gjson.GetBytes(in, "share_conf.ftp_password").String(),
		Host: gjson.GetBytes(in, "share_conf.ftp_host").String(),
		Port: int(gjson.GetBytes(in, "share_conf.ftp_port").Int()),
		FTPNamespace: gjson.GetBytes(in, "share_conf.ftp_namespace").String(),
	}
	return &SlugShareItem{
		Namespace: gjson.GetBytes(in, "tenant_id").String(),
		TenantName:  gjson.GetBytes(in, "tenant_name").String(),
		TenantID: gjson.GetBytes(in, "tenant_id").String(),
		ServiceID: gjson.GetBytes(in, "service_id").String(),
		Action: gjson.GetBytes(in, "action").String(),
		ServiceKey: gjson.GetBytes(in, "service_key").String(),
		AppVersion: gjson.GetBytes(in, "app_version").String(),	
		DeployVersion: gjson.GetBytes(in, "deploy_version").String(),
		ShareID: gjson.GetBytes(in, "share_id").String(),
		Logger: logger,
		EventID: eventID,
		Config: GetBuilderConfig(),
		FTPConf: sf,
	}
}

//Run Run
func (i *SlugShareItem) Run(timeout time.Duration) error {
	packageName := fmt.Sprintf("/grdata/build/tenant/%s/slug/%s/%s.tgz",
		i.TenantID, i.ServiceID, i.DeployVersion)
	i.PackageName = packageName
	i.Logger.Debug(fmt.Sprintf("数据中心文件路径: %s", packageName), map[string]string{"step":"slug-share"})
	if _, err := os.Stat(packageName); err != nil {
		i.Logger.Error(fmt.Sprintf("数据中心文件不存在: %s", packageName), map[string]string{"step":"slug-share", "status":"failure"})
		return err
	}
	if err := i.ShareToYS(packageName); err != nil {
		return err
	}
	return nil
}

func createMD5(packageName string) (string, error) {
	file, err := os.Open(packageName)
    if err != nil {
        return "", err
	}
	defer file.Close()
	body, err := ioutil.ReadAll(file)
    if err != nil {
        return "", err
    }
    value := md5.Sum(body)
	if err := ioutil.WriteFile(packageName+".md5", value[:], 0644); err != nil {
		return "", err
	}
	return packageName+".md5", nil
}

//ShareToYS ShareToYS
func (i *SlugShareItem)ShareToYS(file string)error {
	i.Logger.Info("开始分享云市", map[string]string{"step":"slug-share"})
	md5, err := createMD5(file)
	if err != nil {
		i.Logger.Error("生成md5失败", map[string]string{"step":"slug-share", "status":"success"})
	}
	logrus.Debugf("md5 path is %s", md5)
	ftpUpPath := fmt.Sprintf("%s%s", i.FTPConf.FTPNamespace, i.ServiceKey)
	if err := i.UploadFtp(ftpUpPath, file, md5); err != nil {
		logrus.Errorf("upload file to ftp error: %s", err.Error())
		return err
	}
	i.Logger.Info("分享云市完成", map[string]string{"step":"slug-share", "status":"success"})
	return nil
}

//UploadFtp UploadFt
func (i *SlugShareItem)UploadFtp(path, file, md5 string) error {
	i.Logger.Info(fmt.Sprintf("开始上传代码包: %s", file), map[string]string{"step":"slug-share"})
	ftp  := sources.NewFTPManager(i.FTPConf.Username, i.FTPConf.Password, i.FTPConf.Host, i.FTPConf.Port)
	sc, err := ftp.LoginFTP(i.Logger)
	if err != nil {
		return err
	}
	defer ftp.LogoutFTP(sc, i.Logger)
	bl, err := ftp.CheckMd5FileName(sc, path, md5)
	if err != nil {
		return err
	}
	if bl {
		i.Logger.Info(fmt.Sprintf("文件(%s)已上传", file), map[string]string{"step":"slug-share", "status":"success"})
		return nil
	}
	if err := ftp.UploadFile(sc, path, file, i.Logger); err != nil {
		i.Logger.Error(fmt.Sprintf("上传代码包%s失败", file), map[string]string{"step":"slug-share", "status":"failure"})
		return err
	}
	if err := ftp.UploadFile(sc, path, md5, i.Logger); err != nil {
		i.Logger.Error("上传md5文件失败", map[string]string{"step":"slug-share", "status":"failure"})
		return err
	}
	i.Logger.Info("代码包上传完成", map[string]string{"step":"slug-share", "status":"success"})
	return nil
}

//UpdateShareStatus 更新任务执行结果
func (i *SlugShareItem) UpdateShareStatus(status string) error {
	result := &dbmodel.AppPublish{
		ServiceKey: i.ServiceKey,
		AppVersion: i.AppVersion,
		Slug: i.PackageName,
		Status: status,
	}
	if err := db.GetManager().AppPublishDao().AddModel(result); err != nil {
		return err
	}
	return nil
} 
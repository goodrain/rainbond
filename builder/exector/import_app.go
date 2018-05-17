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
	"fmt"
	"github.com/Sirupsen/logrus"
	"io/ioutil"
	"os/exec"
	"strings"
	"path/filepath"
	"github.com/goodrain/rainbond/builder/sources"
)

func init() {
	RegisterWorker("import_app", NewExportApp)
}

// 组目录命名规则，将组名中unicode转为中文，并去掉空格，"JAVA-ETCD\\u5206\\u4eab\\u7ec4" -> "JAVA-ETCD分享组"
func (i *ExportApp) importApp() error {
	// 解压tar包
	if err := i.unzip(); err != nil {
		return err
	}

	// 上传镜像和源码包到仓库中
	if err := i.loadApps(); err != nil {
		return err
	}

	// 更橷应用状态
	if err := i.updateStatus("success"); err != nil {
		return err
	}

	return nil
}

func (i *ExportApp) unzip() error {
	cmd := fmt.Sprintf("cd %s && rm -rf %s && tar -xf %s.tar", filepath.Dir(i.SourceDir), i.SourceDir, i.SourceDir)
	err := exec.Command("sh", "-c", cmd).Run()
	if err != nil {
		logrus.Error("Failed to unzip for import app: ", i.SourceDir, ".tar")
		return err
	}

	logrus.Debug("Failed to unzip for import app: ", i.SourceDir, ".tar")
	return err
}

func (i *ExportApp) loadApps() error {
	apps, err := i.parseApps()
	if err != nil {
		return err
	}

	for _, app := range apps {
		// 获取该组件资源文件
		serviceName := app.Get("service_cname").String()
		serviceName = unicode2zh(serviceName)
		serviceDir := fmt.Sprintf("%s/%s", i.SourceDir, serviceName)
		files, err := ioutil.ReadDir(serviceDir)
		if err != nil || len(files) < 1 {
			logrus.Error("Failed to list in service directory: ", serviceDir)
			return err
		}

		fileName := filepath.Join(serviceDir, files[0].Name())
		logrus.Debug("Parse the source file for service: ", fileName)

		// 判断该用应资源是什么类型
		// 如果是镜像，则加载到本地，并上传到仓库
		// 如果slug文件，则上传到ftp服务器
		if strings.HasSuffix(fileName, ".image.tar") {
			// 加载到本地
			if err := sources.ImageLoad(i.DockerClient, fileName, i.Logger); err != nil {
				logrus.Error("Failed to load image for service: ", serviceName)
				return err
			}

			// 上传到仓库
			image := app.Get("image").String()
			user := app.Get("service_image.hub_user").String()
			pass := app.Get("service_image.hub_password").String()
			if err := sources.ImagePush(i.DockerClient, image, user, pass, i.Logger, 15); err != nil {
				logrus.Error("Failed to load image for service: ", serviceName)
				return err
			}

			logrus.Debug("Successful load and push the image ", image)
		} else if strings.HasSuffix(fileName, ".tgz") {
			// 将slug包上传到ftp服务器

			// 提取tfp服务器信息
			shareSlugPath := app.Get("share_slug_path").String()
			ftpHost := app.Get("service_slug.ftp_host").String()
			ftpPort := app.Get("service_slug.ftp_port").String()
			ftpUsername := app.Get("service_slug.ftp_username").String()
			ftpPassword := app.Get("service_slug.ftp_password").String()

			ftpClient, err := sources.NewSFTPClient(ftpUsername, ftpPassword, ftpHost, ftpPort)
			if err != nil {
				logrus.Error("Failed to create ftp client: ", err)
				return err
			}

			// 开始上传文件
			i.Logger.Info(fmt.Sprintf("获取应用源码：%s", serviceName),
				map[string]string{"step": "get-slug", "status": "failure"})

			err = ftpClient.PushFile(fileName, shareSlugPath, i.Logger)
			ftpClient.Close()
			if err != nil {
				logrus.Errorf("Failed to upload slug file for group %s: %v", i.SourceDir, err)
				return err
			}
			logrus.Debug("Successful upload slug file: ", fileName)

		}

	}

	logrus.Debug("Successful load apps for group: ", i.SourceDir)
	return nil
}

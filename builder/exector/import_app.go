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
	"errors"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/bitly/go-simplejson"
	"github.com/docker/engine-api/client"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/tidwall/gjson"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"github.com/goodrain/rainbond/api/model"
)

func init() {
	RegisterWorker("import_app", NewImportApp)
}

//ExportApp Export app to specified format(rainbond-app or dockercompose)
type ImportApp struct {
	EventID      string `json:"event_id"`
	Format       string `json:"format"`
	SourceDir    string `json:"source_dir"`
	ServiceImage model.ServiceImage
	ServiceSlug  model.ServiceSlug
	Logger       event.Logger
	DockerClient *client.Client
}

//NewExportApp create
func NewImportApp(in []byte) TaskWorker {
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		logrus.Error("Failed to create task for export app: ", err)
		return nil
	}

	eventID := gjson.GetBytes(in, "event_id").String()
	serviceImage := gjson.GetBytes(in, "service_image").Value().(model.ServiceImage)
	serviceSlug := gjson.GetBytes(in, "service_image").Value().(model.ServiceSlug)
	logger := event.GetManager().GetLogger(eventID)

	return &ImportApp{
		Format:       gjson.GetBytes(in, "format").String(),
		SourceDir:    gjson.GetBytes(in, "source_dir").String(),
		ServiceImage: serviceImage,
		ServiceSlug:  serviceSlug,
		Logger:       logger,
		EventID:      eventID,
		DockerClient: dockerClient,
	}
}

//Stop stop
func (i *ImportApp) Stop() error {
	return nil
}

//Name return worker name
func (i *ImportApp) Name() string {
	return "export_app"
}

//GetLogger GetLogger
func (i *ImportApp) GetLogger() event.Logger {
	return i.Logger
}

//Run Run
func (i *ImportApp) Run(timeout time.Duration) error {
	if i.Format == "rainbond-app" {
		err := i.importApp()
		if err != nil {
			i.updateStatus("failed")
		}
		return err
	} else {
		return errors.New("Unsupported the format: " + i.Format)
	}
	return nil
}

// 组目录命名规则，将组名中unicode转为中文，并去掉空格，"JAVA-ETCD\\u5206\\u4eab\\u7ec4" -> "JAVA-ETCD分享组"
func (i *ImportApp) importApp() error {
	// 解压tar包
	if err := i.unzip(); err != nil {
		return err
	}

	// 如果目录下有多个子目录，则认为每个子目录是一个应用组，循环导入之
	dirs, err := ioutil.ReadDir(i.SourceDir)
	if err != nil {
		logrus.Error("Failed to read dir for import app: ", i.SourceDir)
		return err
	}

	oldSourceDir := i.SourceDir
	var errMsg string
	for _, dir := range dirs {
		if dir.IsDir() {
			continue
		}

		i.SourceDir = filepath.Join(i.SourceDir, dir.Name())

		// 修改json元数据中的镜像和源码包仓库地址为指定地址
		if err := i.replaceRepo(); err != nil {
			logrus.Errorf("Failed to change repo address in metadata json for %s: %s", i.SourceDir, err)
			errMsg = fmt.Sprintf("%s; %s", errMsg, err.Error())
			continue
		}

		// 上传镜像和源码包到仓库中
		if err := i.loadApps(); err != nil {
			logrus.Errorf("Failed to load apps for %s: %s", i.SourceDir, err)
			errMsg = fmt.Sprintf("%s; %s", errMsg, err.Error())
			continue
		}
	}
	i.SourceDir = oldSourceDir
	if errMsg != "" {
		return errors.New(errMsg)
	}

	// 更新应用状态
	if err := i.updateStatus("success"); err != nil {
		return err
	}

	return nil
}

// i.SourceDir = "/grdata/app/import/n7brv4/web-app"
func (i *ImportApp) unzip() error {
	cmd := fmt.Sprintf("cd %s && tar -xf *.tar", i.SourceDir)
	err := exec.Command("sh", "-c", cmd).Run()
	if err != nil {
		logrus.Error("Failed to unzip for import app: ", i.SourceDir, ".tar")
		return err
	}

	logrus.Debug("Failed to unzip for import app: ", i.SourceDir, ".tar")
	return err
}

// 替换元数据中的镜像和源码包的仓库地址
func (i *ImportApp) replaceRepo() error {
	metaFile := fmt.Sprintf("%s/metadata.json", i.SourceDir)
	logrus.Debug("Change image and slug repo address in: ", metaFile)

	data, err := ioutil.ReadFile(metaFile)
	logrus.Debug("old json: ", string(data))
	meta, err := simplejson.NewJson(data)
	if err != nil {
		return err
	}

	apps, err := meta.Get("apps").Array()
	if err != nil {
		return err
	}

	for index := range apps {
		app := meta.Get("apps").GetIndex(index)
		if _, ok := app.CheckGet("service_image"); ok {
			app.Set("service_image", i.ServiceImage)
		}

		if _, ok := app.CheckGet("service_slug"); ok {
			app.Set("service_slug", i.ServiceSlug)
		}
		apps[index] = app
	}

	meta.Set("apps", apps)
	data, err = meta.Bytes()
	if err != nil {
		return err
	}
	logrus.Debug("new json: ", string(data))

	return ioutil.WriteFile(metaFile, data, 0644)
}

//parseApps get apps array from metadata.json
func (i *ImportApp) parseApps() ([]gjson.Result, error) {
	i.Logger.Info("解析应用信息", map[string]string{"step": "export-app", "status": "success"})

	data, err := ioutil.ReadFile(fmt.Sprintf("%s/metadata.json", i.SourceDir))
	if err != nil {
		i.Logger.Error("导出应用失败，没有找到应用信息", map[string]string{"step": "read-metadata", "status": "failure"})
		logrus.Error("Failed to read metadata file: ", err)
		return nil, err
	}

	arr := gjson.GetBytes(data, "apps").Array()
	if len(arr) < 1 {
		i.Logger.Error("解析应用列表信息失败", map[string]string{"step": "parse-apps", "status": "failure"})
		err := errors.New("not found apps in the metadata")
		logrus.Error("Failed to get apps from json: ", err)
		return nil, err
	}
	logrus.Debug("Successful parse apps array from metadata, count: ", len(arr))

	return arr, nil
}

func (i *ImportApp) loadApps() error {
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

func (i *ImportApp) updateStatus(status string) error {
	logrus.Debug("Update app status in database to: ", status)
	// 从数据库中获取该应用的状态信息
	res, err := db.GetManager().AppDao().GetByEventId(i.EventID)
	if err != nil {
		err = errors.New(fmt.Sprintf("Failed to get app %s from db: %v", i.EventID, err))
		logrus.Error(err)
		return err
	}

	data, err := ioutil.ReadFile(fmt.Sprintf("%s/metadata.json", i.SourceDir))
	if err != nil {
		logrus.Error("Failed to read metadata file for update status: ", err)
		return err
	}

	// 在数据库中更新该应用的状态信息
	app := res.(*dbmodel.AppStatus)
	app.Status = status
	app.Metadata = string(data)

	if err := db.GetManager().AppDao().UpdateModel(app); err != nil {
		err = errors.New(fmt.Sprintf("Failed to update app %s: %v", i.EventID, err))
		logrus.Error(err)
		return err
	}

	return nil
}

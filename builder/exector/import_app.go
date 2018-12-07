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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/bitly/go-simplejson"
	"github.com/docker/docker/client"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"github.com/tidwall/gjson"
)

func init() {
	RegisterWorker("import_app", NewImportApp)
}

//ImportApp Export app to specified format(rainbond-app or dockercompose)
type ImportApp struct {
	EventID       string   `json:"event_id"`
	Format        string   `json:"format"`
	SourceDir     string   `json:"source_dir"`
	Apps          []string `json:"apps"`
	ServiceImage  model.ServiceImage
	ServiceSlug   model.ServiceSlug
	Logger        event.Logger
	DockerClient  *client.Client
	oldAPPPath    map[string]string
	oldPluginPath map[string]string
}

//NewImportApp create
func NewImportApp(in []byte, m *exectorManager) (TaskWorker, error) {
	eventID := gjson.GetBytes(in, "event_id").String()
	var serviceImage model.ServiceImage
	if err := json.Unmarshal([]byte(gjson.GetBytes(in, "service_image").String()), &serviceImage); err != nil {
		logrus.Error("Failed to unmarshal service_image for import: ", err)
		return nil, err
	}

	var serviceSlug model.ServiceSlug
	if err := json.Unmarshal([]byte(gjson.GetBytes(in, "service_slug").String()), &serviceSlug); err != nil {
		logrus.Error("Failed to unmarshal service_slug for import app: ", err)
		return nil, err
	}

	apps := make([]string, 0, 10)
	for _, r := range gjson.GetBytes(in, "apps").Array() {
		apps = append(apps, r.String())
	}

	logger := event.GetManager().GetLogger(eventID)

	return &ImportApp{
		Format:        gjson.GetBytes(in, "format").String(),
		SourceDir:     gjson.GetBytes(in, "source_dir").String(),
		Apps:          apps,
		ServiceImage:  serviceImage,
		ServiceSlug:   serviceSlug,
		Logger:        logger,
		EventID:       eventID,
		DockerClient:  m.DockerClient,
		oldAPPPath:    make(map[string]string),
		oldPluginPath: make(map[string]string),
	}, nil
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

//ErrorCallBack if run error will callback
func (i *ImportApp) ErrorCallBack(err error) {
	i.updateStatus("failed", "")
}

//Run Run
func (i *ImportApp) Run(timeout time.Duration) error {
	if i.Format == "rainbond-app" {
		err := i.importApp()
		if err != nil {
			return err
		}
		return nil
	}
	return errors.New("Unsupported the format: " + i.Format)
}

// importApp import app
// support batch import
func (i *ImportApp) importApp() error {
	oldSourceDir := i.SourceDir
	var datas = "["
	tmpDir := oldSourceDir + "/TmpUnzipDir"
	for _, app := range i.Apps {
		if err := i.updateStatusForApp(app, "importing"); err != nil {
			logrus.Errorf("Failed to update status to importing for app %s: %v", app, err)
		}
		appFile := filepath.Join(oldSourceDir, app)
		os.MkdirAll(tmpDir, 0755)
		err := util.Unzip(appFile, tmpDir)
		if err != nil {
			logrus.Errorf("Failed to unzip app file %s : %s", appFile, err.Error())
			i.updateStatusForApp(app, "failed")
			continue
		}
		files, _ := ioutil.ReadDir(tmpDir)
		if len(files) < 1 {
			logrus.Errorf("Failed to read files in tmp dir %s: %v", appFile, err)
			continue
		}
		i.SourceDir = fmt.Sprintf("%s/%s", oldSourceDir, files[0].Name())
		if _, err := os.Stat(i.SourceDir); err == nil {
			os.RemoveAll(i.SourceDir)
		}
		err = os.Rename(fmt.Sprintf("%s/%s", tmpDir, files[0].Name()), i.SourceDir)
		if err != nil {
			logrus.Errorf("Failed to mv source dir to %s: %v", i.SourceDir, err)
			continue
		}
		os.RemoveAll(tmpDir)
		// push image and slug file

		// 修改json元数据中的镜像和源码包仓库地址为指定地址
		metaFile := fmt.Sprintf("%s/metadata.json", i.SourceDir)
		logrus.Debug("Change image and slug repo address in: ", metaFile)

		data, err := ioutil.ReadFile(metaFile)
		meta, err := simplejson.NewJson(data)
		if err != nil {
			logrus.Errorf("Failed to new json for load app %s: %v", appFile, err)
			i.updateStatusForApp(app, "failed")
			continue
		}
		apps, err := meta.Get("apps").Array()
		if err != nil {
			logrus.Errorf("Failed to get apps from meta for load app %s: %v", appFile, err)
			i.updateStatusForApp(app, "failed")
			continue
		}
		for index := range apps {
			app := meta.Get("apps").GetIndex(index)
			if _, ok := app.CheckGet("service_image"); ok {
				app.Set("service_image", i.ServiceImage)
			}
			if _, ok := app.CheckGet("service_slug"); ok {
				app.Set("service_slug", i.ServiceSlug)
			}
			getAppImage := func() string {
				oldname, _ := app.Get("share_image").String()
				oldImageName := sources.ImageNameWithNamespaceHandle(oldname)
				var image string
				if i.ServiceImage.NameSpace == "" {
					image = fmt.Sprintf("%s/%s:%s", i.ServiceImage.HubUrl, oldImageName.Name, oldImageName.Tag)
				} else {
					image = fmt.Sprintf("%s/%s/%s:%s", i.ServiceImage.HubUrl, i.ServiceImage.NameSpace, oldImageName.Name, oldImageName.Tag)
				}
				return image
			}
			getAppSlugPath := func() string {
				shareSlugPath, _ := app.Get("share_slug_path").String()
				if strings.HasPrefix(shareSlugPath, "/grdata/build/tenant/") {
					shareSlugPath = strings.Replace(shareSlugPath, "/grdata/build/tenant/", "", 1)
				}
				if i.ServiceSlug.FtpHost == "" {
					shareSlugPath = fmt.Sprintf("/grdata/build/tenant/%s", shareSlugPath)
				} else {
					info := strings.Split(shareSlugPath, "/")
					shareSlugPath = fmt.Sprintf("%s/%s", i.ServiceSlug.NameSpace, strings.Join(info[1:], "/"))
				}
				return strings.Replace(shareSlugPath, "//", "/", -1)
			}

			if oldimage, ok := app.CheckGet("share_image"); ok {
				appKey, _ := app.Get("service_key").String()
				i.oldAPPPath[appKey], _ = oldimage.String()
				app.Set("share_image", getAppImage())
			}
			if oldslug, ok := app.CheckGet("share_slug_path"); ok {
				appKey, _ := app.Get("service_key").String()
				i.oldAPPPath[appKey], _ = oldslug.String()
				app.Set("share_slug_path", getAppSlugPath())
			}
			apps[index] = app
		}
		meta.Set("apps", apps)
		data, err = meta.MarshalJSON()
		if err != nil {
			logrus.Errorf("Failed to marshal json for app %s: %v", appFile, err)
			i.updateStatusForApp(app, "failed")
			continue
		}
		err = ioutil.WriteFile(metaFile, data, 0644)
		if err != nil {
			logrus.Errorf("Failed to write metadata load app %s: %v", appFile, err)
			i.updateStatusForApp(app, "failed")
			continue
		}
		// load all service version attachment in app
		if err := i.loadApps(); err != nil {
			logrus.Errorf("Failed to load app %s: %v", appFile, err)
			i.updateStatusForApp(app, "failed")
			continue
		}
		// load all plugins
		if err := i.importPlugins(); err != nil {
			logrus.Errorf("Failed to load app plugin %s: %v", appFile, err)
			i.updateStatusForApp(app, "failed")
			continue
		}
		if err := i.updateStatusForApp(app, "success"); err != nil {
			logrus.Errorf("Failed to update status to success for app %s: %v", app, err)
			continue
		}
		if datas == "[" {
			datas += string(data)
		} else {
			datas += ", " + string(data)
		}
		os.Rename(appFile, appFile+".success")
		logrus.Debug("Successful import app: ", appFile)
	}
	datas += "]"
	i.SourceDir = oldSourceDir

	metadatasFile := fmt.Sprintf("%s/metadatas.json", i.SourceDir)
	if err := ioutil.WriteFile(metadatasFile, []byte(datas), 0644); err != nil {
		logrus.Errorf("Failed to load apps %s: %v", i.SourceDir, err)
		return err
	}
	// 更新应用状态
	if err := i.updateStatus("success", datas); err != nil {
		logrus.Errorf("Failed to load apps %s: %v", i.SourceDir, err)
		return err
	}
	return nil
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

func (i *ImportApp) importPlugins() error {
	i.Logger.Info("解析插件信息", map[string]string{"step": "import-plugins", "status": "success"})

	data, err := ioutil.ReadFile(fmt.Sprintf("%s/metadata.json", i.SourceDir))
	if err != nil {
		i.Logger.Error("导出插件失败，没有找到应用信息", map[string]string{"step": "read-metadata", "status": "failure"})
		logrus.Error("Failed to read metadata file: ", err)
		return err
	}

	// 先修改json数据中的插件镜像服务器地址为新环境中的服务器地址
	meta, err := simplejson.NewJson(data)
	if err != nil {
		logrus.Errorf("Failed to new json for load app %s: %v", i.SourceDir, err)
		return err
	}

	oldPlugins, err := meta.Get("plugins").Array()
	if err != nil {
		logrus.Errorf("Failed to get plugins from meta for load app %s: %v", i.SourceDir, err)
		return nil
	}

	for index := range oldPlugins {
		plugin := meta.Get("plugins").GetIndex(index)
		if _, ok := plugin.CheckGet("plugin_image"); ok {
			plugin.Set("plugin_image", i.ServiceImage)
		}
		getImageImage := func() string {
			oldname, _ := plugin.Get("share_image").String()
			oldImageName := sources.ImageNameWithNamespaceHandle(oldname)
			var image string
			if i.ServiceImage.NameSpace == "" {
				image = fmt.Sprintf("%s/%s:%s", i.ServiceImage.HubUrl, oldImageName.Name, oldImageName.Tag)
			} else {
				image = fmt.Sprintf("%s/%s/%s:%s", i.ServiceImage.HubUrl, i.ServiceImage.NameSpace, oldImageName.Name, oldImageName.Tag)
			}
			return image
		}
		if oldimage, ok := plugin.CheckGet("share_image"); ok {
			appKey, _ := plugin.Get("service_key").String()
			i.oldPluginPath[appKey], _ = oldimage.String()
			plugin.Set("share_image", getImageImage())
		}
		oldPlugins[index] = plugin
	}

	meta.Set("plugins", oldPlugins)
	data, err = meta.MarshalJSON()
	if err != nil {
		logrus.Errorf("Failed to marshal json for app %s: %v", i.SourceDir, err)
		return err
	}

	// 修改完毕后写回文件中
	err = ioutil.WriteFile(fmt.Sprintf("%s/metadata.json", i.SourceDir), data, 0644)
	if err != nil {
		logrus.Errorf("Failed to write metadata load app %s: %v", i.SourceDir, err)
		return err
	}

	plugins := gjson.GetBytes(data, "plugins").Array()

	for _, plugin := range plugins {
		pluginName := plugin.Get("plugin_name").String()
		pluginName = unicode2zh(pluginName)
		pluginDir := fmt.Sprintf("%s/%s", i.SourceDir, pluginName)

		files, err := ioutil.ReadDir(pluginDir)
		if err != nil || len(files) < 1 {
			logrus.Error("Failed to list in service directory: ", pluginDir)
			continue
		}

		fileName := filepath.Join(pluginDir, files[0].Name())
		logrus.Debug("Parse the source file for service: ", fileName)

		// 将镜像加载到本地，并上传到仓库
		if strings.HasSuffix(fileName, ".image.tar") {
			// 加载到本地
			if err := sources.ImageLoad(i.DockerClient, fileName, i.Logger); err != nil {
				logrus.Error("Failed to load image for service: ", pluginName)
				return err
			}
			// 上传到仓库
			user := plugin.Get("plugin_image.hub_user").String()
			pass := plugin.Get("plugin_image.hub_password").String()
			// 上传之前先要根据新的仓库地址修改镜像名
			image := i.oldPluginPath[plugin.Get("service_key").String()]
			imageName := sources.ImageNameWithNamespaceHandle(image)
			saveImageName := fmt.Sprintf("%s/%s:%s", "goodrain.me", imageName.Name, imageName.Tag)
			newImageName := plugin.Get("share_image").String()
			if err := sources.ImageTag(i.DockerClient, saveImageName, newImageName, i.Logger, 2); err != nil {
				return fmt.Errorf("change plugin image tag(%s => %s) error %s", saveImageName, newImageName, err.Error())
			}
			// 开始上传
			if err := sources.ImagePush(i.DockerClient, saveImageName, user, pass, i.Logger, 15); err != nil {
				return fmt.Errorf("push plugin image %s error %s", image, err.Error())
			}
			logrus.Debug("Successful load and push the plugin image ", image)
		}
	}

	return nil
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
			continue
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
			oldImage := i.oldAPPPath[app.Get("service_key").String()]
			oldImageName := sources.ImageNameWithNamespaceHandle(oldImage)
			user := app.Get("service_image.hub_user").String()
			pass := app.Get("service_image.hub_password").String()
			// 上传之前先要根据新的仓库地址修改镜像名
			image := app.Get("share_image").String()
			if err := sources.ImageTag(i.DockerClient, fmt.Sprintf("%s/%s:%s", "goodrain.me", oldImageName.Name, oldImageName.Tag), image, i.Logger, 15); err != nil {
				return fmt.Errorf("change image tag(%s => %s) error %s", fmt.Sprintf("%s/%s:%s", i.ServiceImage.HubUrl, oldImageName.Name, oldImageName.Tag), image, err.Error())
			}
			// 开始上传
			if err := sources.ImagePush(i.DockerClient, image, user, pass, i.Logger, 15); err != nil {
				return fmt.Errorf("push  image %s error %s", image, err.Error())
			}
			logrus.Debug("Successful load and push the image ", image)
		} else if strings.HasSuffix(fileName, ".tgz") {
			shareSlugPath := app.Get("share_slug_path").String()
			ftpHost := app.Get("service_slug.ftp_host").String()
			ftpPort := app.Get("service_slug.ftp_port").String()
			ftpUsername := app.Get("service_slug.ftp_username").String()
			ftpPassword := app.Get("service_slug.ftp_password").String()
			// if sftp available
			if ftpHost != "" && ftpPort != "" {
				ftpClient, err := sources.NewSFTPClient(ftpUsername, ftpPassword, ftpHost, ftpPort)
				if err != nil {
					return err
				}
				err = ftpClient.PushFile(fileName, shareSlugPath, i.Logger)
				ftpClient.Close()
				if err != nil {
					logrus.Errorf("Failed to upload slug file for group %s: %v", i.SourceDir, err)
					return err
				}
			} else {
				if err := util.CheckAndCreateDir(filepath.Dir(shareSlugPath)); err != nil {
					logrus.Error("Failed create slug file directory %s error %s ", filepath.Dir(shareSlugPath), err.Error())
					return err
				}
				err := util.CopyFile(fileName, shareSlugPath)
				if err != nil {
					logrus.Error("Failed to copy slug file to local directory: ", shareSlugPath)
					return err
				}
			}
			logrus.Debug("Successful upload slug file: ", fileName)
		}
	}
	logrus.Debug("Successful load apps for group: ", i.SourceDir)
	return nil
}

func (i *ImportApp) updateStatus(status, data string) error {
	logrus.Debug("Update app status in database to: ", status)
	// 从数据库中获取该应用的状态信息
	res, err := db.GetManager().AppDao().GetByEventId(i.EventID)
	if err != nil {
		err = fmt.Errorf("failed to get app %s from db: %s", i.EventID, err.Error())
		logrus.Error(err)
		return err
	}

	// 在数据库中更新该应用的状态信息
	res.Status = status

	if err := db.GetManager().AppDao().UpdateModel(res); err != nil {
		err = fmt.Errorf("failed to update app %s: %s", i.EventID, err.Error())
		logrus.Error(err)
		return err
	}

	return nil
}

func (i *ImportApp) updateStatusForApp(app, status string) error {
	logrus.Debugf("Update status in database for app %s to: %s", app, status)
	// 从数据库中获取该应用的状态信息
	res, err := db.GetManager().AppDao().GetByEventId(i.EventID)
	if err != nil {
		err = fmt.Errorf("Failed to get app %s from db: %s", i.EventID, err.Error())
		logrus.Error(err)
		return err
	}

	// 在数据库中更新该应用的状态信息
	appsMap := str2map(res.Apps)
	appsMap[app] = status
	res.Apps = map2str(appsMap)

	if err := db.GetManager().AppDao().UpdateModel(res); err != nil {
		err = fmt.Errorf("Failed to update app %s: %s", i.EventID, err.Error())
		logrus.Error(err)
		return err
	}

	return nil
}

func str2map(str string) map[string]string {
	result := make(map[string]string, 10)

	for _, app := range strings.Split(str, ",") {
		appMap := strings.Split(app, ":")
		result[appMap[0]] = appMap[1]
	}

	return result
}

func map2str(m map[string]string) string {
	var result string

	for k, v := range m {
		kv := k + ":" + v

		if result == "" {
			result += kv
		} else {
			result += "," + kv
		}
	}

	return result
}

// 只保留"/"后面的部分，并去掉不合法字符，一般用于把导出的镜像文件还原为镜像名
func buildFromLinuxFileName(fileName string) string {
	if fileName == "" {
		return fileName
	}

	arr := strings.Split(fileName, "/")

	if str := arr[len(arr)-1]; str == "" {
		fileName = strings.Replace(fileName, "---", "/", -1)
	} else {
		fileName = str
	}

	fileName = strings.Replace(fileName, "--", ":", -1)
	fileName = strings.TrimSpace(fileName)

	return fileName
}

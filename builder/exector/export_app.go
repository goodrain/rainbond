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
	"time"

	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/event"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
)

//ExportApp Export app to specified format(rainbond-app or dockercompose)
type ExportApp struct {
	EventID      string `json:"event_id"`
	GroupKey     string `json:"service_key"`
	Version      string `json:"version"`
	Format       string `json:"format"`
	SourceDir    string `json:"source_dir"`
	Logger       event.Logger
	DockerClient *client.Client
}

func init() {
	RegisterWorker("export_app", NewExportApp)
}

//NewExportApp create
func NewExportApp(in []byte) TaskWorker {
	eventID := gjson.GetBytes(in, "event_id").String()
	logger := event.GetManager().GetLogger(eventID)
	return &ExportApp{
		GroupKey:  gjson.GetBytes(in, "group_key").String(),
		Format:    gjson.GetBytes(in, "format").String(),
		SourceDir: gjson.GetBytes(in, "source_dir").String(),
		Logger:    logger,
		EventID:   eventID,
	}
}

//Run Run
func (i *ExportApp) Run(timeout time.Duration) error {
	if i.Format == "rainbond-app" {
		err := i.exportRainbondAPP()
		if err != nil {
			i.updateStatus("failed")
		}
		return err
	} else if i.Format == "docker-compose" {
		err := i.exportDockerCompose()
		if err != nil {
			i.updateStatus("failed")
		}
		return err
	}
	return nil
}

// 组目录命名规则，将组名中unicode转为中文，并去掉空格，"JAVA-ETCD\\u5206\\u4eab\\u7ec4" -> "JAVA-ETCD分享组"
func (i *ExportApp) exportRainbondAPP() error {
	// 保存用应镜像和slug包
	if err := i.saveApps(); err != nil {
		return err
	}

	// 打包整个目录为tar包
	if err := i.generateTarFile(); err != nil {
		return err
	}

	// 更新应用状态
	if err := i.updateStatus("success"); err != nil {
		return err
	}

	return nil
}

// 组目录命名规则，将组名中unicode转为中文，并去掉空格，"JAVA-ETCD\\u5206\\u4eab\\u7ec4" -> "JAVA-ETCD分享组"
func (i *ExportApp) exportDockerCompose() error {
	// 保存用应镜像和slug包
	if err := i.saveApps(); err != nil {
		return err
	}

	// 当导出格式为docker-compose时，需要导出runner镜像
	if err := i.exportRunnerImage(); err != nil {
		return err
	}

	// 在主目录中生成文件：docker-compose.yaml
	if err := i.generateDockerComposeYaml(); err != nil {
		return err
	}

	// 生成应用启动脚本
	if err := i.generateStartScript(); err != nil {
		return err
	}

	// 打包整个目录为tar包
	if err := i.generateTarFile(); err != nil {
		return err
	}

	// 更新应用状态
	if err := i.updateStatus("success"); err != nil {
		return err
	}

	return nil
}

//Stop stop
func (i *ExportApp) Stop() error {
	return nil
}

//Name return worker name
func (i *ExportApp) Name() string {
	return "export_app"
}

//GetLogger GetLogger
func (i *ExportApp) GetLogger() event.Logger {
	return i.Logger
}

//parseApps get apps array from metadata.json
func (i *ExportApp) parseApps() ([]gjson.Result, error) {
	i.Logger.Info("解析应用信息", map[string]string{"step": "export-app", "status": "success"})

	data, err := ioutil.ReadFile(fmt.Sprintf("%s/metadata.json", i.SourceDir))
	if err != nil {
		i.Logger.Error("导出应用失败，没有找到应用信息", map[string]string{"step": "read-metadata", "status": "failure"})
		logrus.Error("Failed to export rainbond app:", err)
		return nil, err
	}

	arr := gjson.GetBytes(data, "apps").Array()
	if len(arr) < 1 {
		i.Logger.Error("解析应用列表信息失败", map[string]string{"step": "parse-apps", "status": "failure"})
		err := errors.New("Not found app in the metadata.")
		logrus.Error("Failed to get apps from json:", err)
		return nil, err
	}

	return arr, nil
}

func (i *ExportApp) replaceMetadata(old, new string) error {
	fileName := fmt.Sprintf("%s/metadata.json", i.SourceDir)

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}

	context := strings.Replace(string(data), old, new, -1)

	err = ioutil.WriteFile(fileName, []byte(context), 0644)

	return err
}

func (i *ExportApp) exportImage(app gjson.Result) error {
	serviceName := app.Get("service_cname").String()
	serviceName = unicode2zh(serviceName)

	serviceDir := fmt.Sprintf("%s/%s", i.SourceDir, serviceName)
	os.MkdirAll(serviceDir, 0755)

	// 处理掉文件名中冒号等不合法字符
	image := app.Get("image").String()
	tarFileName := buildToLinuxFileName(image)

	// 如果是runner镜像则跳过
	if checkIsRunner(image) {
		return nil
	}

	// docker pull image-name
	_, err := sources.ImagePull(i.DockerClient, image, types.ImagePullOptions{}, i.Logger, 15)
	if err != nil {
		i.Logger.Error(fmt.Sprintf("拉取镜像失败：%s", image),
			map[string]string{"step": "pull-image", "status": "failure"})
		logrus.Error("Failed to pull image:", err)
	}

	// save image to tar file
	err = sources.ImageSave(i.DockerClient, image, fmt.Sprintf("%s/%s.image.tar", serviceDir, tarFileName), i.Logger)
	if err != nil {
		i.Logger.Error(fmt.Sprintf("保存镜像失败：%s", image),
			map[string]string{"step": "save-image", "status": "failure"})
		logrus.Error("Failed to save image:", err)
		return err
	}

	return nil
}

// 下载组件相的镜像，如果该组件是源码方式部署，则下载相应slug文件
// 组件目录命名规则：将组件名中unicode转为中文，并去掉空格，"2048\\u5e94\\u7528" -> "2048应用"
// 镜像包命名规则: goodrain.me/percona-mysql:5.5_latest -> percona-mysqlTAG5.5_latest.image.tar
// slug包命名规则: /app_publish/vzrd9po6/9d2635a7c59d4974bb4dc62f04/v1.0_20180207165207.tgz -> v1.0_20180207165207.tgz
func (i *ExportApp) saveApps() error {
	apps, err := i.parseApps()
	if err != nil {
		return err
	}

	i.Logger.Info("开始打包应用", map[string]string{"step": "export-app", "status": "success"})

	for _, app := range apps {
		serviceName := app.Get("service_cname").String()
		serviceName = unicode2zh(serviceName)

		serviceDir := fmt.Sprintf("%s/%s", i.SourceDir, serviceName)
		os.MkdirAll(serviceDir, 0755)

		// 如果该slug文件存在于本地，则直接复制，然后修改json中的share_slug_path字段
		shareSlugPath := app.Get("share_slug_path").String()
		tarFileName := buildToLinuxFileName(shareSlugPath)
		_, err := os.Stat(shareSlugPath)
		if os.IsExist(err) {
			err = exec.Command(fmt.Sprintf("cp %s %s/%s", shareSlugPath, serviceDir, tarFileName)).Run()
			if err == nil {
				// 当导出格式为rainbond-app时，要修改json文件中share_slug_path字段，以便在导入时将每个组件与slug文件对应起来
				i.replaceMetadata(shareSlugPath, tarFileName)
				return nil
			}
		}

		// 如果这个字段存在于该app中，则认为该app是源码部署方式，并从ftp下载相应slug文件
		// 否则认为该app是镜像方式部署，然后下载相应镜像即可
		ftpHost := app.Get("service_slug.ftp_host").String()
		if ftpHost == "" {
			logrus.Infof("Not found fields ftp_host for service key %s", serviceName)

			// 下载镜像到应用导出目录
			if err := i.exportImage(app); err != nil {
				return err
			}

			return nil
		}

		i.Logger.Info(fmt.Sprintf("解析应用源码信息：%s", serviceName),
			map[string]string{"step": "parse-slug", "status": "failure"})

		// 提取tfp服务器信息
		ftpPort := app.Get("service_slug.ftp_port").String()
		ftpUsername := app.Get("service_slug.ftp_username").String()
		ftpPassword := app.Get("service_slug.ftp_password").String()

		ftpClient, err := sources.NewSFTPClient(ftpUsername, ftpPassword, ftpPort, ftpHost)
		if err != nil {
			logrus.Error("Failed to create ftp client:", err)
			return err
		}

		// 开始下载文件
		i.Logger.Info(fmt.Sprintf("获取应用源码：%s", serviceName),
			map[string]string{"step": "get-slug", "status": "failure"})

		err = ftpClient.DownloadFile(shareSlugPath, fmt.Sprintf("%s/%s", serviceDir, tarFileName), i.Logger)
		ftpClient.Close()
		if err != nil {
			logrus.Errorf("Failed to download slug file for group key %s: %v", i.GroupKey, err)
			return err
		}

		i.replaceMetadata(shareSlugPath, tarFileName)
	}
	return nil
}

// unicode2zh 将unicode转为中文，并去掉空格
func unicode2zh(uText string) (context string) {
	for i, char := range strings.Split(uText, `\\u`) {
		if i < 1 {
			context = char
			continue
		}

		length := len(char)
		if length > 3 {
			pre := char[:4]
			zh, err := strconv.ParseInt(pre, 16, 32)
			if err != nil {
				context += char
				continue
			}

			context += fmt.Sprintf("%c", zh)

			if length > 4 {
				context += char[4:]
			}
		}

	}

	context = strings.TrimSpace(context)

	return context
}

func checkIsRunner(image string) bool {
	return strings.Contains(image, "/runner")
}

func (i *ExportApp) exportRunnerImage() error {
	isExist := false
	var image string

	apps, err := i.parseApps()
	if err != nil {
		return err
	}

	for _, app := range apps {
		image = app.Get("image").String()
		if checkIsRunner(image) {
			isExist = true
			break
		}
	}

	if isExist {
		_, err := sources.ImagePull(i.DockerClient, image, types.ImagePullOptions{}, i.Logger, 15)
		if err != nil {
			i.Logger.Error(fmt.Sprintf("拉取镜像失败：%s", image),
				map[string]string{"step": "pull-image", "status": "failure"})
			logrus.Error("Failed to pull image:", err)
		}

		err = sources.ImageSave(i.DockerClient, image, buildToLinuxFileName(image), i.Logger)
		if err != nil {
			i.Logger.Error(fmt.Sprintf("保存镜像失败：%s", image),
				map[string]string{"step": "save-image", "status": "failure"})
			logrus.Error("Failed to save image:", err)
			return err
		}
	}

	return nil
}

type Service struct {
	Image         string            `yaml:"image"`
	ContainerName string            `yaml:"container_name"`
	NetworkMode   string            `yaml:"network_mode"`
	Restart       string            `yaml:"restart"`
	Volumes       []string          `yaml:"volumes"`
	Command       []string          `yaml:"command"`
	Environment   map[string]string `yaml:"environment"`
	Loggin        struct {
		Driver  string `yaml:"driver"`
		Options struct {
			MaxSize string `yaml:"max-size"`
			MaxFile string `yaml:"max-file"`
		}
	} `yaml:"options"`
}

type DockerComposeYaml struct {
	Version  string              `yaml:"version"`
	Volumes  []string            `yaml:"volumes"`
	Services map[string]*Service `yaml:"services"`
}

func (i *ExportApp) generateDockerComposeYaml() error {
	// 因为在保存apps的步骤中更新了json文件，所以要重新加载
	apps, err := i.parseApps()
	if err != nil {
		return err
	}

	y := &DockerComposeYaml{
		Version:  "2.1",
		Volumes:  make([]string, 0, 3),
		Services: make(map[string]*Service, 5),
	}

	i.Logger.Info("开始生成YAML文件", map[string]string{"step": "build-yaml", "status": "failure"})

	for _, app := range apps {
		appName := app.Get("service_cname").String()
		volumes := make([]string, 0, 3)
		envs := make(map[string]string, 10)

		for _, itme := range app.Get("service_volume_map_list").Array() {
			volumeName := itme.Get("volume_name").String() + ":"
			volumePath := itme.Get("volume_path").String()

			y.Volumes = append(y.Volumes, volumeName)
			volumes = append(volumes, volumeName+volumePath)
		}

		for k, v := range app.Get("service_env_map_list").Map() {
			envs[k] = v.String()
		}

		image := app.Get("image").String()

		// 如果该组件是源码方式部署，则挂载slug文件到runner容器内
		if checkIsRunner(image) {
			volume := fmt.Sprintf("%s:/tmp/slug/slug.tgz", app.Get("share_slug_path"))
			volumes = append(volumes, volume)
		}

		service := &Service{
			Image:         image,
			ContainerName: appName,
			NetworkMode:   "host",
			Restart:       "always",
			Volumes:       volumes,
			Command:       []string{app.Get("cmd").String()},
			Environment:   envs,
		}
		service.Loggin.Driver = "json-file"
		service.Loggin.Options.MaxSize = "5m"
		service.Loggin.Options.MaxFile = "2"

		y.Services[appName] = service
	}

	content, err := yaml.Marshal(y)
	if err != nil {
		i.Logger.Error(fmt.Sprintf("生成YAML文件失败：%v", err), map[string]string{"step": "build-yaml", "status": "failure"})
		logrus.Error("Failed to build yaml file:", err)
		return err
	}

	err = ioutil.WriteFile(fmt.Sprintf("%s/docker-compose.yaml", i.SourceDir), content, 0644)
	if err != nil {
		i.Logger.Error(fmt.Sprintf("创建YAML文件失败：%v", err), map[string]string{"step": "create-yaml", "status": "failure"})
		logrus.Error("Failed to create yaml file:", err)
		return err
	}

	return nil
}

func (i *ExportApp) generateStartScript() error {
	return exec.Command(fmt.Sprintf("cp /src/export-app/run.sh %s/", i.SourceDir)).Run()
}

func (i *ExportApp) generateTarFile() error {
	// /grdata/export-app/myapp-1.0 -> /grdata/export-app
	dirName := path.Dir(i.SourceDir)
	// /grdata/export-app/myapp-1.0 -> myapp-1.0
	baseName := path.Base(i.SourceDir)
	// 打包整个目录为tar包
	err := exec.Command(fmt.Sprintf("sh", "-c", "cd %s ; rm -rf %s.tar ; tar -cf %s.tar %s", dirName, baseName, baseName, baseName)).Run()
	if err != nil {
		i.Logger.Error("打包应用失败", map[string]string{"step": "export-app", "status": "failure"})
		logrus.Errorf("Failed to create tar file for group key %s: %v", i.GroupKey, err)
		return err
	}

	i.Logger.Info("打包应用成功", map[string]string{"step": "export-app", "status": "success"})
	logrus.Infof("Success to export app by group key:", i.GroupKey)
	return nil
}

func (i *ExportApp) updateStatus(status string) error {
	res, err := db.GetManager().AppDao().Get(i.GroupKey, i.Version)
	if err != nil {
		err = errors.New(fmt.Sprintf("Failed to get app %s from db: %v", i.GroupKey, err))
		return err
	}

	app := res.(*model.AppStatus)
	app.Status = status
	app.TimeStamp = time.Now().Nanosecond()

	if db.GetManager().AppDao().UpdateModel(app); err != nil {
		err = errors.New(fmt.Sprintf("Failed to update app %s: %v", i.GroupKey, err))
		return err
	}

	return nil
}

// 只保留"/"后面的部分，并去掉不合法字符
func buildToLinuxFileName(fileName string) string {
	if fileName == "" {
		return fileName
	}

	arr := strings.Split(fileName, "/")
	fileName = arr[len(arr)-1]

	fileName = strings.Replace(fileName, ":", "-", -1)
	fileName = strings.TrimSpace(fileName)

	return fileName
}

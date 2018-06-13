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
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v2"
	"regexp"
)

var re = regexp.MustCompile(`\s`)

//ExportApp Export app to specified format(rainbond-app or dockercompose)
type ExportApp struct {
	EventID      string `json:"event_id"`
	Format       string `json:"format"`
	SourceDir    string `json:"source_dir"`
	Logger       event.Logger
	DockerClient *client.Client
}

func init() {
	RegisterWorker("export_app", NewExportApp)
}

//NewExportApp create
func NewExportApp(in []byte, m *exectorManager) (TaskWorker, error) {
	eventID := gjson.GetBytes(in, "event_id").String()
	logger := event.GetManager().GetLogger(eventID)
	return &ExportApp{
		Format:       gjson.GetBytes(in, "format").String(),
		SourceDir:    gjson.GetBytes(in, "source_dir").String(),
		Logger:       logger,
		EventID:      eventID,
		DockerClient: m.DockerClient,
	}, nil
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
	return errors.New("Unsupported the format: " + i.Format)
}

// 组目录命名规则，将组名中unicode转为中文，并去掉空格，"JAVA-ETCD\\u5206\\u4eab\\u7ec4" -> "JAVA-ETCD分享组"
func (i *ExportApp) exportRainbondAPP() error {
	// 如果该应用已经打包过且是最新版，则跳过打包并返回成功
	if ok := i.isLatest(); ok {
		i.updateStatus("success")
		return nil
	}

	// 删除旧应用组目录，然后重新生成该应用包
	if err := i.CleanSourceDir(); err != nil {
		return err
	}

	// 保存用应镜像和slug包
	if err := i.saveApps(); err != nil {
		return err
	}

	// 打包整个目录为tar包
	if err := i.zip(); err != nil {
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
	// 如果该应用已经打包过且是最新版，则跳过打包并返回成功
	if ok := i.isLatest(); ok {
		i.updateStatus("success")
		return nil
	}

	// 删除旧应用组目录，然后重新生成该应用包
	if err := i.CleanSourceDir(); err != nil {
		return err
	}

	// 保存用应镜像和slug包
	if err := i.saveApps(); err != nil {
		return err
	}

	// 当导出格式为docker-compose时，需要导出runner镜像
	if err := i.exportRunnerImage(); err != nil {
		return err
	}

	// 在主目录中生成文件：docker-compose.yaml
	if err := i.buildDockerComposeYaml(); err != nil {
		return err
	}

	// 生成应用启动脚本
	if err := i.buildStartScript(); err != nil {
		return err
	}

	// 打包整个目录为tar包
	if err := i.zip(); err != nil {
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

// isLatest 如果该应用已经打包过且是最新版则返回true
func (i *ExportApp) isLatest() bool {
	md5File := fmt.Sprintf("%s/metadata.json.md5", i.SourceDir)

	if _, err := os.Stat(md5File); os.IsNotExist(err) {
		logrus.Debug("The export app md5 file is not found: ", md5File)
		return false
	}

	err := exec.Command("md5sum", "-c", md5File).Run()
	if err != nil {
		logrus.Debug("The export app tar file is not latest.")
		return false
	}

	logrus.Debug("The export app tar file is latest.")
	return true
}

func (i *ExportApp) CleanSourceDir() error {
	logrus.Debug("Ready clean the source directory.")
	metaFile := fmt.Sprintf("%s/metadata.json", i.SourceDir)

	data, err := ioutil.ReadFile(metaFile)
	if err != nil {
		logrus.Error("Failed to read metadata file: ", err)
		return err
	}

	os.RemoveAll(i.SourceDir)
	os.MkdirAll(i.SourceDir, 0755)

	if err := ioutil.WriteFile(metaFile, data, 0644); err != nil {
		logrus.Error("Failed to write metadata file: ", err)
		return err
	}

	return nil
}

//parseApps get apps array from metadata.json
func (i *ExportApp) parseApps() ([]gjson.Result, error) {
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
		err := errors.New("Not found app in the metadata.")
		logrus.Error("Failed to get apps from json: ", err)
		return nil, err
	}
	logrus.Debug("Successful parse apps array from metadata, count: ", len(arr))

	return arr, nil
}

func (i *ExportApp) exportImage(app gjson.Result) error {
	serviceName := app.Get("service_cname").String()
	serviceName = unicode2zh(serviceName)

	serviceDir := fmt.Sprintf("%s/%s", i.SourceDir, serviceName)
	os.MkdirAll(serviceDir, 0755)

	// 处理掉文件名中冒号等不合法字符
	image := app.Get("share_image").String()
	tarFileName := buildToLinuxFileName(image)

	// 如果是runner镜像则跳过
	if checkIsRunner(image) {
		logrus.Debug("Skip the runner image: ", image)
		return nil
	}

	// docker pull image-name
	_, err := sources.ImagePull(i.DockerClient, image, "", "", i.Logger, 15)
	if err != nil {
		// 处理掉文件名中冒号等不合法字符
		image = app.Get("image").String()
		tarFileName = buildToLinuxFileName(image)

		// docker pull image-name
		_, err := sources.ImagePull(i.DockerClient, image, "", "", i.Logger, 15)
		if err != nil {
			i.Logger.Error(fmt.Sprintf("拉取镜像失败：%s", image),
				map[string]string{"step": "pull-image", "status": "failure"})
			logrus.Error("Failed to pull image: ", err)
		}
	}

	// save image to tar file
	err = sources.ImageSave(i.DockerClient, image, fmt.Sprintf("%s/%s.image.tar", serviceDir, tarFileName), i.Logger)
	if err != nil {
		i.Logger.Error(fmt.Sprintf("保存镜像失败：%s", image),
			map[string]string{"step": "save-image", "status": "failure"})
		logrus.Error("Failed to save image: ", err)
		return err
	}
	logrus.Debug("Successful save image file: ", image)

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

		logrus.Debug("Create directory for service: ", serviceDir)

		// 如果该slug文件存在于本地，则直接复制，然后修改json中的share_slug_path字段
		shareSlugPath := app.Get("share_slug_path").String()
		tarFileName := buildToLinuxFileName(shareSlugPath)
		_, err := os.Stat(shareSlugPath)
		if shareSlugPath != "" && err == nil {
			logrus.Debug("The slug file was exist already, direct copy to service dir: ", shareSlugPath)
			err = exec.Command("cp", shareSlugPath, fmt.Sprintf("%s/%s", serviceDir, tarFileName)).Run()
			if err == nil {
				continue
			}
			// 如果copy失败则忽略，在下一步中下载该slug包
			logrus.Debugf("Failed to copy the slug file to service dir %s: %v", shareSlugPath, err)
		}

		// 如果这个字段存在于该app中，则认为该app是源码部署方式，并从ftp下载相应slug文件
		// 否则认为该app是镜像方式部署，然后下载相应镜像即可
		if shareSlugPath == "" {
			logrus.Infof("The service is image model deploy: %s", serviceName)
			// 下载镜像到应用导出目录
			if err := i.exportImage(app); err != nil {
				return err
			}

			continue
		}

		i.Logger.Info(fmt.Sprintf("解析应用源码信息：%s", serviceName),
			map[string]string{"step": "parse-slug", "status": "failure"})
		logrus.Debug("Ready download slug file: ", shareSlugPath)

		// 提取tfp服务器信息
		ftpHost := app.Get("service_slug.ftp_host").String()
		ftpPort := app.Get("service_slug.ftp_port").String()
		ftpUsername := app.Get("service_slug.ftp_username").String()
		ftpPassword := app.Get("service_slug.ftp_password").String()

		ftpClient, err := sources.NewSFTPClient(ftpUsername, ftpPassword, ftpHost, ftpPort)
		if err != nil {
			logrus.Error("Failed to create ftp client: ", err)
			return err
		}

		// 开始下载文件
		i.Logger.Info(fmt.Sprintf("获取应用源码：%s", serviceName),
			map[string]string{"step": "get-slug", "status": "failure"})

		err = ftpClient.DownloadFile(shareSlugPath, fmt.Sprintf("%s/%s", serviceDir, tarFileName), i.Logger)
		ftpClient.Close()
		if err != nil {
			logrus.Errorf("Failed to download slug file for group %s: %v", i.SourceDir, err)
			return err
		}
		logrus.Debug("Successful download slug file: ", shareSlugPath)

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
	var image, tarFileName string

	logrus.Debug("Ready export runner image")
	apps, err := i.parseApps()
	if err != nil {
		return err
	}

	for _, app := range apps {
		image = app.Get("image").String()
		tarFileName = buildToLinuxFileName(image)
		if checkIsRunner(image) {
			logrus.Debug("Discovered runner image at service: ", app.Get("service_cname"))
			isExist = true
			break
		}
	}

	if !isExist {
		logrus.Debug("Not discovered runner image in any service.")
		return nil
	}

	_, err = sources.ImagePull(i.DockerClient, image, "", "", i.Logger, 10)
	if err != nil {
		i.Logger.Error(fmt.Sprintf("拉取镜像失败：%s", image),
			map[string]string{"step": "pull-image", "status": "failure"})
		logrus.Error("Failed to pull image: ", err)
	}

	err = sources.ImageSave(i.DockerClient, image, fmt.Sprintf("%s/%s.image.tar", i.SourceDir, tarFileName), i.Logger)
	if err != nil {
		i.Logger.Error(fmt.Sprintf("保存镜像失败：%s", image),
			map[string]string{"step": "save-image", "status": "failure"})
		logrus.Error("Failed to save image: ", err)
		return err
	}

	logrus.Debug("Successful download runner image: ", image)

	return nil
}

type DockerComposeYaml struct {
	Version  string              `yaml:"version"`
	Volumes  map[string]string   `yaml:"volumes,omitempty"`
	Services map[string]*Service `yaml:"services,omitempty"`
}

type Service struct {
	Image         string            `yaml:"image"`
	ContainerName string            `yaml:"container_name,omitempty"`
	Restart       string            `yaml:"restart,omitempty"`
	NetworkMode   string            `yaml:"network_mode,omitempty"`
	Volumes       []string          `yaml:"volumes,omitempty"`
	Command       string            `yaml:"command,omitempty"`
	Environment   map[string]string `yaml:"environment,omitempty"`
	Loggin        struct {
		Driver  string `yaml:"driver,omitempty"`
		Options struct {
			MaxSize string `yaml:"max-size,omitempty"`
			MaxFile string `yaml:"max-file,omitempty"`
		}
	} `yaml:"logging,omitempty"`
}

func (i *ExportApp) buildDockerComposeYaml() error {
	// 因为在保存apps的步骤中更新了json文件，所以要重新加载
	apps, err := i.parseApps()
	if err != nil {
		return err
	}

	y := &DockerComposeYaml{
		Version:  "2.1",
		Volumes:  make(map[string]string, 5),
		Services: make(map[string]*Service, 5),
	}

	i.Logger.Info("开始生成YAML文件", map[string]string{"step": "build-yaml", "status": "failure"})
	logrus.Debug("Build docker compose yaml file in directory: ", i.SourceDir)

	for _, app := range apps {
		image := app.Get("image").String()
		appName := app.Get("service_cname").String()
		appName = unicode2zh(appName)
		volumes := make([]string, 0, 3)
		envs := make(map[string]string, 10)

		// 如果该组件是镜像方式部署，需要做两件事
		// 1. 在.volumes中创建一个volume
		// 2. 在.services.volumes中做映射
		for _, item := range app.Get("service_volume_map_list").Array() {
			volumeName := item.Get("volume_name").String()
			volumeName = buildToLinuxFileName(volumeName)
			volumePath := item.Get("volume_path").String()

			y.Volumes[volumeName] = ""
			volumes = append(volumes, fmt.Sprintf("%s:%s", volumeName, volumePath))
		}

		// 如果该组件是源码方式部署，则挂载slug文件到runner容器内
		if checkIsRunner(image) {
			shareSlugPath := app.Get("share_slug_path").String()
			tarFileName := buildToLinuxFileName(shareSlugPath)
			volume := fmt.Sprintf("__GROUP_DIR__/%s/%s:/tmp/slug/slug.tgz", appName, tarFileName)
			volumes = append(volumes, volume)
			logrus.Debug("Mount the slug file to runner image: ", volume)
		}

		// 处理环境变量
		for k, v := range app.Get("service_env_map_list").Map() {
			envs[k] = v.String()
		}

		for _, item := range app.Get("service_connect_info_map_list").Array() {
			key := item.Get("attr_name").String()
			value := item.Get("attr_value").String()
			envs[key] = value
		}

		// 如果该app依赖了另了个app-b，则把app-b中所有公开环境变量注入到该app
		for _, item := range app.Get("dep_service_map_list").Array() {
			serviceKey := item.Get("dep_service_key").String()
			depEnvs := i.getPublicEnvByKey(serviceKey, &apps)
			for k, v := range depEnvs {
				envs[k] = v
			}
		}

		service := &Service{
			Image:         image,
			ContainerName: appName,
			Restart:       "always",
			NetworkMode:   "host",
			Volumes:       volumes,
			Command:       app.Get("cmd").String(),
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
		logrus.Error("Failed to build yaml file: ", err)
		return err
	}

	err = ioutil.WriteFile(fmt.Sprintf("%s/docker-compose.yaml", i.SourceDir), content, 0644)
	if err != nil {
		i.Logger.Error(fmt.Sprintf("创建YAML文件失败：%v", err), map[string]string{"step": "create-yaml", "status": "failure"})
		logrus.Error("Failed to create yaml file: ", err)
		return err
	}

	return nil
}

func (i *ExportApp) getPublicEnvByKey(serviceKey string, apps *[]gjson.Result) map[string]string {
	envs := make(map[string]string, 5)
	for _, app := range *apps {
		appKey := app.Get("service_key").String()
		if appKey == serviceKey {
			for _, item := range app.Get("service_connect_info_map_list").Array() {
				key := item.Get("attr_name").String()
				value := item.Get("attr_value").String()
				envs[key] = value
			}
			break
		}
	}

	return envs
}

func (i *ExportApp) buildStartScript() error {
	if err := exec.Command("cp", "/src/export-app/run.sh", i.SourceDir).Run(); err != nil {
		err = errors.New("Failed to generate start script to: " + i.SourceDir)
		logrus.Error(err)
		return err
	}

	logrus.Debug("Successful generate start script to: ", i.SourceDir)
	return nil
}

//ErrorCallBack if run error will callback
func (i *ExportApp) ErrorCallBack(err error) {

}

func (i *ExportApp) zip() error {
	err := util.Zip(i.SourceDir, i.SourceDir+".tar")
	if err != nil {
		i.Logger.Error("打包应用失败", map[string]string{"step": "export-app", "status": "failure"})
		logrus.Errorf("Failed to create tar file for group %s: %v", i.SourceDir, err)
		return err
	}

	// 生成MD5值并写入到文件，以便在下次收到该请求时决定是否该重新打包该应用
	metadataFile := fmt.Sprintf("%s/metadata.json", i.SourceDir)
	if err := exec.Command("sh", "-c", fmt.Sprintf("md5sum %s > %s.md5", metadataFile, metadataFile)).Run(); err != nil {
		err = errors.New(fmt.Sprintf("Failed to create md5 file: %v", err))
		logrus.Error(err)
		return err
	}

	i.Logger.Info("打包应用成功", map[string]string{"step": "export-app", "status": "success"})
	logrus.Info("Successful export app by event id: ", i.EventID)
	return nil
}

func (i *ExportApp) updateStatus(status string) error {
	logrus.Debug("Update app status in database to: ", status)
	// 从数据库中获取该应用的状态信息
	res, err := db.GetManager().AppDao().GetByEventId(i.EventID)
	if err != nil {
		err = errors.New(fmt.Sprintf("Failed to get app %s from db: %v", i.EventID, err))
		logrus.Error(err)
		return err
	}

	// 在数据库中更新该应用的状态信息
	res.Status = status

	if err := db.GetManager().AppDao().UpdateModel(res); err != nil {
		err = errors.New(fmt.Sprintf("Failed to update app %s: %v", i.EventID, err))
		logrus.Error(err)
		return err
	}

	return nil
}

// 只保留"/"后面的部分，并去掉不合法字符，一般用于把镜像名变为将要导出的文件名
func buildToLinuxFileName(fileName string) string {
	if fileName == "" {
		return fileName
	}

	arr := strings.Split(fileName, "/")

	if str := arr[len(arr)-1]; str == "" {
		fileName = strings.Replace(fileName, "/", "---", -1)
	} else {
		fileName = str
	}

	fileName = strings.Replace(fileName, ":", "--", -1)
	fileName = re.ReplaceAllString(fileName, "")

	return fileName
}

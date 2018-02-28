
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
	"github.com/pquerna/ffjson/ffjson"
	"github.com/Sirupsen/logrus"
	"time"
	"fmt"
	"path"
	"os"
	"regexp"
	"strings"
	"github.com/goodrain/rainbond/pkg/event"
	"github.com/tidwall/gjson"
	//"github.com/docker/docker/api/types"
	"github.com/docker/engine-api/types"
	//"github.com/docker/docker/client"
	"github.com/docker/engine-api/client"
	"github.com/goodrain/rainbond/pkg/builder/sources"
	"github.com/akkuman/parseConfig"
	"github.com/goodrain/rainbond/pkg/db"
	dbmodel "github.com/goodrain/rainbond/pkg/db/model"
	"github.com/goodrain/rainbond/pkg/worker/discover/model"
	"github.com/goodrain/rainbond/pkg/builder/apiHandler"
)

//REGISTRYDOMAIN REGISTRY_DOMAIN
var REGISTRYDOMAIN = "goodrain.me"

//SourceCodeBuildItem SouceCodeBuildItem
type SourceCodeBuildItem struct {
	Namespace 		string `json:"namespace"`
	TenantName 		string `json:"tenant_name"`
	ServiceAlias 	string `json:"service_alias"`
	Action			string `json:"action"`
	DestImage 		string `json:"dest_image"`
	Logger 			event.Logger `json:"logger"`
	EventID	 		string `json:"event_id"`
	CacheDir		string `json:"cache_dir"`
	SourceDir		string `json:"source_dir"`
	DockerClient    *client.Client	
	Config          parseConfig.Config
	TenantID        string
	ServiceID 		string
	DeployVersion   string
	Lang 			string
	Runtime			string
	BuildEnvs		map[string]string
	CodeSouceInfo   sources.CodeSourceInfo
}

//NewSouceCodeBuildItem 创建实体
func NewSouceCodeBuildItem(in []byte) *SourceCodeBuildItem {
	eventID := gjson.GetBytes(in, "event_id").String()
	logger := event.GetManager().GetLogger(eventID)
	csi := sources.CodeSourceInfo{
		ServerType : gjson.GetBytes(in, "server_type").String(),
		RepositoryURL: gjson.GetBytes(in, "repo_url").String(),
		Branch: gjson.GetBytes(in, "branch").String(),
		//TODO: user password api发出任务时判断是否存在，不存在则套用define
		User: gjson.GetBytes(in, "user").String(),
		Password: gjson.GetBytes(in, "password").String(),
		TenantID:  gjson.GetBytes(in, "tenant_id").String(),
	}
	envs := gjson.GetBytes(in, "envs").String()
	be := make(map[string]string)
	if err := ffjson.Unmarshal([]byte(envs), &be); err != nil {
		logrus.Errorf("unmarshal build envs error: %s", err.Error())
	}
	return &SourceCodeBuildItem{
		Namespace: gjson.GetBytes(in, "tenant_id").String(),
		TenantName:  gjson.GetBytes(in, "tenant_name").String(),
		ServiceAlias: gjson.GetBytes(in, "service_alias").String(),
		TenantID: gjson.GetBytes(in, "tenant_id").String(),
		ServiceID: gjson.GetBytes(in, "service_id").String(),
		Action: gjson.GetBytes(in, "action").String(),
		DeployVersion: gjson.GetBytes(in, "deploy_version").String(),
		Logger: logger,
		EventID: eventID,
		Config: GetBuilderConfig(),
		CodeSouceInfo: csi,
		Lang: gjson.GetBytes(in, "lang").String(),
		Runtime: gjson.GetBytes(in, "runtime").String(),
		BuildEnvs: be,
	}
}

//Run Run
func (i *SourceCodeBuildItem) Run(timeout time.Duration) error {
	//TODO:
	// 1.clone
	// 2.check dockerfile/ source_code
	// 3.build
	// 4.upload image /upload slug
	i.CacheDir = i.CodeSouceInfo.GetCodeCacheDir()
	i.SourceDir = i.CodeSouceInfo.GetCodeSourceDir()
	_, err := sources.GitClone(i.CodeSouceInfo, i.SourceDir, i.Logger, 3)
	if err != nil {
		logrus.Errorf("pull git code error: %s", err.Error())
		i.Logger.Error(fmt.Sprintf("拉取代码失败, %s", err.Error()), map[string]string{"step": "builder-exector", "status":"failure"})
		return err
	}
	if i.IsDockerfile() {
		i.Logger.Info("代码识别出Dockerfile,直接构建镜像。", map[string]string{"step": "builder-exector"})
		if err := i.buildImage(); err != nil {
			logrus.Errorf("build from dockerfile error: %s", err.Error())
			i.Logger.Error("解析Dockerfile发生异常", map[string]string{"step":"builder-exector", "status":"failure"})
			return err
		}
	}else {
		i.Logger.Info("开始代码构建", map[string]string{"step": "builder-exector"})
		if err := i.buildCode(); err != nil {
			logrus.Errorf("build from source code error: %s", err.Error())
			i.Logger.Error("编译代码包过程遇到异常", map[string]string{"step":"builder-exector", "status":"failure"})	
			return err
		}
	
	}
	i.Logger.Info("应用同步完成，开始启动应用", map[string]string{"step": "build-exector"})
	if err := apiHandler.UpgradeService(i.TenantName, i.ServiceAlias, i.CreateUpgradeTaskBody()); err != nil {
		i.Logger.Error("启动应用失败，请手动启动", map[string]string{"step": "callback", "status": "failure"})
		logrus.Errorf("rolling update service error, %s", err.Error())
	}
	i.Logger.Info("应用启动成功", map[string]string{"step":"build-exector"})
	return nil
}

//IsDockerfile CheckDockerfile
func (i *SourceCodeBuildItem) IsDockerfile() bool {
	filepath := path.Join(i.SourceDir, "Dockerfile")
	_, err := os.Stat(filepath)
	if err != nil {
		return false
	}
	return true
}

func (i *SourceCodeBuildItem) buildImage() error {
	filepath := path.Join(i.SourceDir, "Dockerfile")
	i.Logger.Info("开始解析Dockerfile", map[string]string{"step":"builder-exector"})
	_, err := sources.ParseFile(filepath)
	if err != nil {
		return err
	}
	reg := regexp.MustCompile(`.*(?:\:|\/)([\w\-\.]+)/([\w\-\.]+)\.git`)
	rc := reg.FindSubmatch([]byte(i.CodeSouceInfo.RepositoryURL))
	logrus.Debugf("reg git url piece is %s", rc)
	pieceID := func(s string) string {
		mm := []byte(s)
		return string(mm[12:])
	}(i.ServiceID)
	if len(rc) != 3 {
		return fmt.Errorf("git—url识别错误")
	}
	name := fmt.Sprintf("%s_%s_%s", pieceID, string(rc[1]), string(rc[2]))
	tag := i.DeployVersion
	buildImageName := strings.ToLower(fmt.Sprintf("%s/%s_%s", REGISTRYDOMAIN, name, tag))
	i.Logger.Info(fmt.Sprintf("构建镜像名称为: %s",buildImageName), map[string]string{"step":"builder-exector"})
	buildOptions := types.ImageBuildOptions{
		Tags:   	[]string{buildImageName},
		Dockerfile: filepath,
		Remove: 	true,
	}
	if _, ok := i.BuildEnvs["NO_CACHE"]; ok {
		buildOptions.NoCache = true
	}else {
		buildOptions.NoCache = false
	}
	err = sources.ImageBuild(i.DockerClient, buildOptions, i.Logger, 3)
	i.Logger.Info("开始构建镜像: ", map[string]string{"step": "builder-exector"})
	if err != nil {
		i.Logger.Error(fmt.Sprintf("构造镜像%s失败: %s", buildImageName, err.Error()), map[string]string{"step":"builder-exector", "status":"failure"})
		logrus.Errorf("build image error: %s", err.Error())
		return err
	}
	err = sources.ImagePush(i.DockerClient, buildImageName, types.ImagePushOptions{}, i.Logger, 2)
	i.Logger.Info("镜像构建成功，开始推送镜像至仓库", map[string]string{"step": "builder-exector"})
	if err != nil {
		i.Logger.Error("推送镜像失败", map[string]string{"step": "builder-exector"})
		logrus.Errorf("push image error: %s", err.Error())
		return err 
	}

	i.Logger.Info("应用同步完成，开始启动应用", map[string]string{"step": "build-exector"})
	if err := apiHandler.UpgradeService(i.TenantName, i.ServiceAlias, i.CreateUpgradeTaskBody()); err != nil {
		i.Logger.Error("启动应用失败，请手动启动", map[string]string{"step": "callback", "status": "failure"})
		logrus.Errorf("rolling update service error, %s", err.Error())
	}
	return nil
}

func (i *SourceCodeBuildItem) buildCode() error {
	i.Logger.Info("开始编译代码包", map[string]string{"step": "build-exector"})
	packageName := fmt.Sprintf("/grdata/build/tenant/%s/slug/%s/%s.tgz",
	i.TenantID, i.ServiceID, i.DeployVersion)
	tgzPath := fmt.Sprintf("/grdata/build/tenant/%s/slug/%s",
		i.TenantID, i.ServiceID)	
	logfile := fmt.Sprintf("/grdata/build/tenant/%s/slug/%s/%s.log",
		i.TenantID, i.ServiceID, i.DeployVersion)
	//repos := strings.Split(i.CodeSouceInfo.RepositoryURL, " ")
	buildCMD := "plugins/scripts/build.pl"
	buildName := func(s, buildVersion string) string {
		mm := []byte(s)
		return string(mm[:8]) + "_" + buildVersion
	}(i.ServiceID, i.DeployVersion)
	cmd := []string{buildCMD,
		"-b", i.CodeSouceInfo.Branch,
		"-s", i.SourceDir,
		"-c", i.CacheDir,
		"-d", tgzPath,
		"-v", i.DeployVersion,
		"-l", logfile,
		"-tid", i.TenantID,
		"-sid", i.ServiceID,
		"-r", i.Runtime,
		"-g", i.Lang,
		"--name", buildName}
	logrus.Debugf("build cmd is %v", cmd)
	if len(i.BuildEnvs) != 0 {
		buildEnvStr := ""
		mm := []string{}
		for k,v := range i.BuildEnvs {
			mm = append(mm, k+"="+v)
		}
		if len(mm) > 1 {
			buildEnvStr = strings.Join(mm, ":::")
		}else {
			buildEnvStr = mm[0]
		}
		cmd = append(cmd, "-e")
		cmd = append(cmd, buildEnvStr)
	}
	if err := ShowExec("perl", cmd, i.Logger); err != nil {
		i.Logger.Error("编译代码包失败", map[string]string{"step":"build-code", "status":"failure"})
		logrus.Error("build perl error")
		return err
	}
	i.Logger.Info("编译代码包完成。", map[string]string{"step":"build-code", "status":"success"})
	fileInfo, err := os.Stat(packageName)
	if err != nil {
		i.Logger.Error("构建代码包检测失败", map[string]string{"step":"build-code", "status":"failure"})
		logrus.Errorf("build package check error")
		return err
	}
	if fileInfo.Size() == 0 {
		i.Logger.Error(fmt.Sprintf("构建失败！ 构建包大小为0 name：%s", packageName), 
		map[string]string{"step":"build-code", "status":"failure"})
		return fmt.Errorf("build package size is 0")
	}
	i.Logger.Info("代码构建完成", map[string]string{"step":"build-code", "status":"success"})
	vi := &dbmodel.VersionInfo {
		DeliveredType: "slug",
		DeliveredPath: packageName,
		EventID: i.EventID,
	}
	if err := i.UpdateVersionInfo(vi); err != nil {
		logrus.Errorf("update version info error: %s", err.Error())
	}
	return nil
}

//CreateUpgradeTaskBody 构造消息体
func (i *SourceCodeBuildItem) CreateUpgradeTaskBody() *model.RollingUpgradeTaskBody{
	return &model.RollingUpgradeTaskBody{
		TenantID: i.TenantID,
		ServiceID: i.ServiceID,
		//TODO: 区分curr version 与 new version 
		CurrentDeployVersion: i.DeployVersion,
		NewDeployVersion: i.DeployVersion,
		EventID: i.EventID,
	}
}

//UpdateVersionInfo 更新任务执行结果
func (i *SourceCodeBuildItem) UpdateVersionInfo(vi *dbmodel.VersionInfo) error {
	version,err :=db.GetManager().VersionInfoDao().GetVersionByEventID(i.EventID)
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
	if err := db.GetManager().VersionInfoDao().UpdateModel(version); err != nil {
		return err
	}
	return nil
} 

//UpdateCheckResult UpdateCheckResult
func (i *SourceCodeBuildItem)UpdateCheckResult(result *dbmodel.CodeCheckResult) error {
	return nil
}
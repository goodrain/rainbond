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
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"gopkg.in/src-d/go-git.v4/plumbing/object"

	"github.com/goodrain/rainbond/util"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/builder/parser/code"
	"github.com/goodrain/rainbond/event"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/tidwall/gjson"
	//"github.com/docker/docker/api/types"
	"github.com/docker/engine-api/types"
	//"github.com/docker/docker/client"

	"github.com/docker/engine-api/client"
	"github.com/goodrain/rainbond/builder/apiHandler"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/worker/discover/model"
)

//REGISTRYDOMAIN REGISTRY_DOMAIN
var REGISTRYDOMAIN = "goodrain.me"

//SourceCodeBuildItem SouceCodeBuildItem
type SourceCodeBuildItem struct {
	Namespace    string       `json:"namespace"`
	TenantName   string       `json:"tenant_name"`
	ServiceAlias string       `json:"service_alias"`
	Action       string       `json:"action"`
	DestImage    string       `json:"dest_image"`
	Logger       event.Logger `json:"logger"`
	EventID      string       `json:"event_id"`
	CacheDir     string       `json:"cache_dir"`
	//SourceDir     string       `json:"source_dir"`
	TGZDir        string `json:"tgz_dir"`
	DockerClient  *client.Client
	TenantID      string
	ServiceID     string
	DeployVersion string
	Lang          string
	Runtime       string
	BuildEnvs     map[string]string
	CodeSouceInfo sources.CodeSourceInfo
	RepoInfo      *sources.RepostoryBuildInfo
	commit        *object.Commit
}

//NewSouceCodeBuildItem create
func NewSouceCodeBuildItem(in []byte) *SourceCodeBuildItem {
	eventID := gjson.GetBytes(in, "event_id").String()
	logger := event.GetManager().GetLogger(eventID)
	csi := sources.CodeSourceInfo{
		ServerType:    gjson.GetBytes(in, "server_type").String(),
		RepositoryURL: gjson.GetBytes(in, "repo_url").String(),
		Branch:        gjson.GetBytes(in, "branch").String(),
		User:          gjson.GetBytes(in, "user").String(),
		Password:      gjson.GetBytes(in, "password").String(),
		TenantID:      gjson.GetBytes(in, "tenant_id").String(),
	}
	envs := gjson.GetBytes(in, "envs").String()
	be := make(map[string]string)
	if err := ffjson.Unmarshal([]byte(envs), &be); err != nil {
		logrus.Errorf("unmarshal build envs error: %s", err.Error())
	}
	scb := &SourceCodeBuildItem{
		Namespace:     gjson.GetBytes(in, "tenant_id").String(),
		TenantName:    gjson.GetBytes(in, "tenant_name").String(),
		ServiceAlias:  gjson.GetBytes(in, "service_alias").String(),
		TenantID:      gjson.GetBytes(in, "tenant_id").String(),
		ServiceID:     gjson.GetBytes(in, "service_id").String(),
		Action:        gjson.GetBytes(in, "action").String(),
		DeployVersion: gjson.GetBytes(in, "deploy_version").String(),
		Logger:        logger,
		EventID:       eventID,
		CodeSouceInfo: csi,
		Lang:          gjson.GetBytes(in, "lang").String(),
		Runtime:       gjson.GetBytes(in, "runtime").String(),
		BuildEnvs:     be,
		commit:        &object.Commit{},
	}
	scb.CacheDir = fmt.Sprintf("/cache/build/%s/cache/%s", scb.TenantID, scb.ServiceID)
	//scb.SourceDir = scb.CodeSouceInfo.GetCodeSourceDir()
	scb.TGZDir = fmt.Sprintf("/grdata/build/tenant/%s/slug/%s", scb.TenantID, scb.ServiceID)
	return scb
}

//Run Run
func (i *SourceCodeBuildItem) Run(timeout time.Duration) error {
	//TODO:
	// 1.clone
	// 2.check dockerfile/ source_code
	// 3.build
	// 4.upload image /upload slug
	rbi, err := sources.CreateRepostoryBuildInfo(i.CodeSouceInfo.RepositoryURL, i.CodeSouceInfo.Branch, i.TenantID)
	if err != nil {
		i.Logger.Error("Git项目仓库地址格式错误", map[string]string{"step": "parse"})
		return err
	}
	i.RepoInfo = rbi
	if err := i.prepare(); err != nil {
		logrus.Errorf("prepare build code error: %s", err.Error())
		i.Logger.Error(fmt.Sprintf("准备源码构建失败"), map[string]string{"step": "builder-exector", "status": "failure"})
		return err
	}
	i.CodeSouceInfo.RepositoryURL = rbi.RepostoryURL
	rs, err := sources.GitCloneOrPull(i.CodeSouceInfo, rbi.GetCodeHome(), i.Logger, 5)
	if err != nil {
		logrus.Errorf("pull git code error: %s", err.Error())
		i.Logger.Error(fmt.Sprintf("拉取代码失败，请确保代码可以被正常下载"), map[string]string{"step": "builder-exector", "status": "failure"})
		return err
	}
	//get last commit
	commit, err := sources.GetLastCommit(rs)
	if err != nil || commit == nil {
		logrus.Errorf("get code commit info error: %s", err.Error())
		i.Logger.Error(fmt.Sprintf("读取代码版本信息失败"), map[string]string{"step": "builder-exector", "status": "failure"})
		return err
	}
	i.commit = commit
	info := fmt.Sprintf("版本:%s 上传者:%s Commit:%s ", commit.Hash.String()[0:7], commit.Author.Name, commit.Message)
	i.Logger.Info(info, map[string]string{"step": "code-version"})

	if i.Lang == string(code.Dockerfile) || i.Lang == string(code.Docker) {
		i.Logger.Info("代码识别出Dockerfile,直接构建镜像。", map[string]string{"step": "builder-exector"})
		if err := i.buildImage(); err != nil {
			logrus.Errorf("build from dockerfile error: %s", err.Error())
			i.Logger.Error("基于Dockerfile构建应用发生错误，请分析日志查找原因", map[string]string{"step": "builder-exector", "status": "failure"})
			return err
		}
	} else {
		i.Logger.Info("开始代码构建", map[string]string{"step": "builder-exector"})
		if err := i.buildCode(); err != nil {
			logrus.Errorf("build from source code error: %s", err.Error())
			i.Logger.Error("编译代码包过程遇到异常", map[string]string{"step": "builder-exector", "status": "failure"})
			return err
		}
	}
	i.Logger.Info("应用同步完成，开始启动应用", map[string]string{"step": "build-exector"})
	if err := apiHandler.UpgradeService(i.TenantName, i.ServiceAlias, i.CreateUpgradeTaskBody()); err != nil {
		i.Logger.Error("启动应用任务发送失败，请手动启动", map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("rolling update service error, %s", err.Error())
		return err
	}
	i.Logger.Info("应用启动任务发送成功", map[string]string{"step": "build-exector"})
	return nil
}

//IsDockerfile CheckDockerfile
func (i *SourceCodeBuildItem) IsDockerfile() bool {
	filepath := path.Join(i.RepoInfo.GetCodeBuildAbsPath(), "Dockerfile")
	_, err := os.Stat(filepath)
	if err != nil {
		return false
	}
	return true
}

func (i *SourceCodeBuildItem) buildImage() error {
	filepath := path.Join(i.RepoInfo.GetCodeBuildAbsPath(), "Dockerfile")
	i.Logger.Info("开始解析Dockerfile", map[string]string{"step": "builder-exector"})
	_, err := sources.ParseFile(filepath)
	if err != nil {
		logrus.Error("parse dockerfile error.", err.Error())
		i.Logger.Error(fmt.Sprintf("预解析Dockerfile失败"), map[string]string{"step": "builder-exector"})
		return err
	}
	reg := regexp.MustCompile(`.*(?:\:|\/)([\w\-\.]+)/([\w\-\.]+)\.git`)
	rc := reg.FindSubmatch([]byte(i.CodeSouceInfo.RepositoryURL))
	var name string
	if len(rc) == 3 {
		name = fmt.Sprintf("%s_%s_%s", i.ServiceAlias, string(rc[1]), string(rc[2]))
	} else {
		name = fmt.Sprintf("%s_%s", i.ServiceAlias, "dockerfilebuild")
	}
	tag := i.DeployVersion
	buildImageName := strings.ToLower(fmt.Sprintf("%s/%s:%s", REGISTRYDOMAIN, name, tag))
	buildOptions := types.ImageBuildOptions{
		Tags:   []string{buildImageName},
		Remove: true,
	}
	if _, ok := i.BuildEnvs["NO_CACHE"]; ok {
		buildOptions.NoCache = true
	} else {
		buildOptions.NoCache = false
	}
	i.Logger.Info("开始构建镜像", map[string]string{"step": "builder-exector"})
	err = sources.ImageBuild(i.DockerClient, i.RepoInfo.GetCodeBuildAbsPath(), buildOptions, i.Logger, 5)
	if err != nil {
		i.Logger.Error(fmt.Sprintf("构造镜像%s失败", buildImageName), map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("build image error: %s", err.Error())
		return err
	}
	// check image exist
	_, err = sources.ImageInspectWithRaw(i.DockerClient, buildImageName)
	if err != nil {
		i.Logger.Error(fmt.Sprintf("构造镜像%s失败,请查看Debug日志", buildImageName), map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("get image inspect error: %s", err.Error())
		return err
	}
	i.Logger.Info("镜像构建成功，开始推送镜像至仓库", map[string]string{"step": "builder-exector"})
	err = sources.ImagePush(i.DockerClient, buildImageName, "", "", i.Logger, 5)
	if err != nil {
		i.Logger.Error("推送镜像失败", map[string]string{"step": "builder-exector"})
		logrus.Errorf("push image error: %s", err.Error())
		return err
	}
	i.Logger.Info("镜像推送镜像至仓库成功", map[string]string{"step": "builder-exector"})
	//更新应用的镜像名称
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(i.ServiceID)
	if err != nil {
		i.Logger.Error("更新应用镜像信息失败", map[string]string{"step": "builder-exector"})
		logrus.Errorf("get service from db error: %s", err.Error())
		return err
	}
	service.ImageName = buildImageName
	err = db.GetManager().TenantServiceDao().UpdateModel(service)
	if err != nil {
		i.Logger.Error("更新应用镜像信息失败", map[string]string{"step": "builder-exector"})
		logrus.Errorf("update service from db error: %s", err.Error())
		return err
	}
	vi := &dbmodel.VersionInfo{
		DeliveredType: "image",
		DeliveredPath: buildImageName,
		EventID:       i.EventID,
		FinalStatus:   "success",
		CodeVersion:   i.commit.Hash.String(),
		CommitMsg:     i.commit.Message,
		Author:        i.commit.Author.Name,
	}
	logrus.Debugf("update app version commit info %s, author %s", i.commit.Message, i.commit.Author.Name)
	if err := i.UpdateVersionInfo(vi); err != nil {
		logrus.Errorf("update version info error: %s", err.Error())
		i.Logger.Error("更新应用版本信息失败", map[string]string{"step": "builder-exector", "status": "failure"})
		return err
	}
	return nil
}
func (i *SourceCodeBuildItem) prepare() error {
	if err := util.CheckAndCreateDir(i.CacheDir); err != nil {
		return err
	}
	if err := util.CheckAndCreateDir(i.TGZDir); err != nil {
		return err
	}
	if err := util.CheckAndCreateDir(i.RepoInfo.GetCodeHome()); err != nil {
		return err
	}
	if i.BuildEnvs["NO_CACHE"] == "true" {
		if !util.DirIsEmpty(i.RepoInfo.GetCodeHome()) {
			os.RemoveAll(i.RepoInfo.GetCodeHome())
		}
		if err := os.RemoveAll(i.CacheDir); err != nil {
			logrus.Error("remove cache dir error", err.Error())
		}
		if err := os.MkdirAll(i.CacheDir, 0755); err != nil {
			logrus.Error("make cache dir error", err.Error())
		}
	}
	os.Chown(i.CacheDir, 200, 200)
	os.Chown(i.TGZDir, 200, 200)
	return nil
}

func (i *SourceCodeBuildItem) buildCode() error {
	i.Logger.Info("开始编译代码包", map[string]string{"step": "build-exector"})
	packageName := fmt.Sprintf("%s/%s.tgz", i.TGZDir, i.DeployVersion)
	logfile := fmt.Sprintf("/grdata/build/tenant/%s/slug/%s/%s.log",
		i.TenantID, i.ServiceID, i.DeployVersion)
	buildName := func(s, buildVersion string) string {
		mm := []byte(s)
		return string(mm[:8]) + "_" + buildVersion
	}(i.ServiceID, i.DeployVersion)
	cmd := []string{"build.pl",
		"-b", i.CodeSouceInfo.Branch,
		"-s", i.RepoInfo.GetCodeBuildAbsPath(),
		"-c", i.CacheDir,
		"-d", i.TGZDir,
		"-v", i.DeployVersion,
		"-l", logfile,
		"-tid", i.TenantID,
		"-sid", i.ServiceID,
		"-r", i.Runtime,
		"-g", i.Lang,
		"--name", buildName}
	if len(i.BuildEnvs) != 0 {
		buildEnvStr := ""
		mm := []string{}
		for k, v := range i.BuildEnvs {
			mm = append(mm, k+"="+v)
		}
		if len(mm) > 1 {
			buildEnvStr = strings.Join(mm, ":::")
		} else {
			buildEnvStr = mm[0]
		}
		cmd = append(cmd, "-e")
		cmd = append(cmd, buildEnvStr)
	}

	if err := ShowExec("perl", cmd, i.Logger); err != nil {
		i.Logger.Error("编译代码包失败", map[string]string{"step": "build-code", "status": "failure"})
		logrus.Error("build perl error,", err.Error())
		return err
	}
	i.Logger.Info("编译代码包完成。", map[string]string{"step": "build-code", "status": "success"})
	fileInfo, err := os.Stat(packageName)
	if err != nil {
		i.Logger.Error("构建代码包检测失败", map[string]string{"step": "build-code", "status": "failure"})
		logrus.Error("build package check error", err.Error())
		return err
	}
	if fileInfo.Size() == 0 {
		i.Logger.Error(fmt.Sprintf("构建失败！ 构建包大小为0 name：%s", packageName),
			map[string]string{"step": "build-code", "status": "failure"})
		return fmt.Errorf("build package size is 0")
	}
	i.Logger.Info("代码构建完成", map[string]string{"step": "build-code", "status": "success"})
	vi := &dbmodel.VersionInfo{
		DeliveredType: "slug",
		DeliveredPath: packageName,
		EventID:       i.EventID,
		FinalStatus:   "success",
		CodeVersion:   i.commit.Hash.String(),
		CommitMsg:     i.commit.Message,
		Author:        i.commit.Author.Name,
	}
	if err := i.UpdateVersionInfo(vi); err != nil {
		logrus.Errorf("update version info error: %s", err.Error())
		i.Logger.Error("更新应用版本信息失败", map[string]string{"step": "build-code", "status": "failure"})
		return err
	}
	return nil
}

//CreateUpgradeTaskBody 构造消息体
func (i *SourceCodeBuildItem) CreateUpgradeTaskBody() *model.RollingUpgradeTaskBody {
	return &model.RollingUpgradeTaskBody{
		TenantID:  i.TenantID,
		ServiceID: i.ServiceID,
		//TODO: 区分curr version 与 new version
		CurrentDeployVersion: i.DeployVersion,
		NewDeployVersion:     i.DeployVersion,
		EventID:              i.EventID,
	}
}

//UpdateVersionInfo 更新任务执行结果
func (i *SourceCodeBuildItem) UpdateVersionInfo(vi *dbmodel.VersionInfo) error {
	version, err := db.GetManager().VersionInfoDao().GetVersionByDeployVersion(i.DeployVersion, i.ServiceID)
	if err != nil {
		return err
	}
	if vi.DeliveredType != "" {
		version.DeliveredType = vi.DeliveredType
	}
	if vi.DeliveredPath != "" {
		version.DeliveredPath = vi.DeliveredPath
		if vi.DeliveredType == "image" {
			version.ImageName = vi.DeliveredPath
		}
	}
	if vi.FinalStatus != "" {
		version.FinalStatus = vi.FinalStatus
	}
	version.CommitMsg = vi.CommitMsg
	version.Author = vi.Author
	version.CodeVersion = vi.CodeVersion
	logrus.Debugf("update app version %+v", *version)
	if err := db.GetManager().VersionInfoDao().UpdateModel(version); err != nil {
		return err
	}
	return nil
}

//UpdateCheckResult UpdateCheckResult
func (i *SourceCodeBuildItem) UpdateCheckResult(result *dbmodel.CodeCheckResult) error {
	return nil
}

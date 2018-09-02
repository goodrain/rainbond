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
	"time"

	"github.com/goodrain/rainbond/util"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/builder/parser"
	"github.com/goodrain/rainbond/builder/parser/code"
	"github.com/goodrain/rainbond/event"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/tidwall/gjson"

	//"github.com/docker/docker/api/types"

	//"github.com/docker/docker/client"

	"github.com/docker/engine-api/client"
	"github.com/goodrain/rainbond/builder/apiHandler"
	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/worker/discover/model"
)

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
	commit        Commit
}

//Commit code Commit
type Commit struct {
	Hash    string
	Author  string
	Message string
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
		ServiceID:     gjson.GetBytes(in, "service_id").String(),
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
	}
	scb.CacheDir = fmt.Sprintf("/cache/build/%s/cache/%s", scb.TenantID, scb.ServiceID)
	//scb.SourceDir = scb.CodeSouceInfo.GetCodeSourceDir()
	scb.TGZDir = fmt.Sprintf("/grdata/build/tenant/%s/slug/%s", scb.TenantID, scb.ServiceID)
	scb.CodeSouceInfo.InitServerType()
	return scb
}

//Run Run
func (i *SourceCodeBuildItem) Run(timeout time.Duration) error {
	//TODO:
	// 1.clone
	// 2.check dockerfile/ source_code
	// 3.build
	// 4.upload image /upload slug
	rbi, err := sources.CreateRepostoryBuildInfo(i.CodeSouceInfo.RepositoryURL, i.CodeSouceInfo.ServerType, i.CodeSouceInfo.Branch, i.TenantID, i.ServiceID)
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
	switch i.CodeSouceInfo.ServerType {
	case "svn":
		csi := i.CodeSouceInfo
		svnclient := sources.NewClient(csi.User, csi.Password, csi.RepositoryURL, rbi.GetCodeHome(), i.Logger)
		rs, err := svnclient.Checkout()
		if err != nil {
			logrus.Errorf("checkout svn code error: %s", err.Error())
			i.Logger.Error(fmt.Sprintf("拉取代码失败，请确保代码可以被正常下载"), map[string]string{"step": "builder-exector", "status": "failure"})
			return err
		}
		if len(rs.Logs.CommitEntrys) < 1 {
			logrus.Errorf("get code commit info error: %s", err.Error())
			i.Logger.Error(fmt.Sprintf("读取代码版本信息失败"), map[string]string{"step": "builder-exector", "status": "failure"})
			return err
		}
		i.commit = Commit{
			Hash:    rs.Logs.CommitEntrys[0].Revision,
			Message: rs.Logs.CommitEntrys[0].Msg,
			Author:  rs.Logs.CommitEntrys[0].Author,
		}
	default:
		//default git
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
		i.commit = Commit{
			Hash:    commit.Hash.String(),
			Author:  commit.Author.Name,
			Message: commit.Message,
		}
	}
	hash := i.commit.Hash
	if len(hash) >= 8 {
		hash = i.commit.Hash[0:7]
	}
	info := fmt.Sprintf("CodeVersion:%s Author:%s Commit:%s ", hash, i.commit.Author, i.commit.Message)
	i.Logger.Info(info, map[string]string{"step": "code-version"})
	if _, ok := i.BuildEnvs["REPARSE"]; ok {
		_, lang, err := parser.ReadRbdConfigAndLang(rbi)
		if err != nil {
			logrus.Errorf("reparse code lange error %s", err.Error())
			i.Logger.Error(fmt.Sprintf("重新解析代码语言错误"), map[string]string{"step": "builder-exector", "status": "failure"})
			return err
		}
		i.Lang = string(lang)
	}

	res, err := i.codeBuild()
	if err != nil {
		i.Logger.Error("Build app version from source code failure,"+err.Error(), map[string]string{"step": "builder-exector", "status": "failure"})
		return err
	}
	if err := i.UpdateBuildVersionInfo(res); err != nil {
		return err
	}
	//TODO:move to pipeline controller
	i.Logger.Info("Build app version complete, will upgrade app.", map[string]string{"step": "build-exector"})
	if err := apiHandler.UpgradeService(i.TenantName, i.ServiceAlias, i.CreateUpgradeTaskBody()); err != nil {
		i.Logger.Error("Failed to send start service tasks. Please start manually", map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("rolling update service error, %s", err.Error())
		return err
	}
	i.Logger.Info("Start app task send success", map[string]string{"step": "build-exector"})
	event.GetManager().ReleaseLogger(i.Logger)
	return nil
}
func (i *SourceCodeBuildItem) codeBuild() (*build.Response, error) {
	codeBuild, err := build.GetBuild(code.Lang(i.Lang))
	if err != nil {
		logrus.Errorf("get code build error: %s lang %s", err.Error(), i.Lang)
		i.Logger.Error(util.Translation("No way of compiling to support this source type was found"), map[string]string{"step": "builder-exector", "status": "failure"})
		return nil, err
	}
	buildReq := &build.Request{
		SourceDir:     i.RepoInfo.GetCodeBuildAbsPath(),
		CacheDir:      i.CacheDir,
		TGZDir:        i.TGZDir,
		RepositoryURL: i.RepoInfo.RepostoryURL,
		ServiceAlias:  i.ServiceAlias,
		ServiceID:     i.ServiceID,
		TenantID:      i.TenantID,
		ServerType:    i.CodeSouceInfo.ServerType,
		Runtime:       i.Runtime,
		Branch:        i.CodeSouceInfo.Branch,
		DeployVersion: i.DeployVersion,
		Commit:        build.Commit{User: i.commit.Author, Message: i.commit.Message, Hash: i.commit.Hash},
		Lang:          code.Lang(i.Lang),
		BuildEnvs:     i.BuildEnvs,
		Logger:        i.Logger,
		DockerClient:  i.DockerClient,
	}
	res, err := codeBuild.Build(buildReq)
	return res, err
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

//CreateUpgradeTaskBody Constructing  upgrade message bodies
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

//UpdateVersionInfo Update build application service version info
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

//UpdateBuildVersionInfo update service build version info to db
func (i *SourceCodeBuildItem) UpdateBuildVersionInfo(res *build.Response) error {
	vi := &dbmodel.VersionInfo{
		DeliveredType: string(res.MediumType),
		DeliveredPath: res.MediumPath,
		EventID:       i.EventID,
		FinalStatus:   "success",
		CodeVersion:   i.commit.Hash,
		CommitMsg:     i.commit.Message,
		Author:        i.commit.Author,
	}
	if err := i.UpdateVersionInfo(vi); err != nil {
		logrus.Errorf("update version info error: %s", err.Error())
		i.Logger.Error("Update application service version information failed", map[string]string{"step": "build-code", "status": "failure"})
		return err
	}
	return nil
}

//UpdateCheckResult UpdateCheckResult
func (i *SourceCodeBuildItem) UpdateCheckResult(result *dbmodel.CodeCheckResult) error {
	return nil
}

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

package parser

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"

	"github.com/pquerna/ffjson/ffjson"

	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/parser/code"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"

	//"github.com/docker/docker/client"
	"github.com/docker/engine-api/client"
)

//SourceCodeParse docker run 命令解析或直接镜像名解析
type SourceCodeParse struct {
	ports        map[int]*Port
	volumes      map[string]*Volume
	envs         map[string]*Env
	source       string
	memory       int
	image        Image
	args         []string
	branchs      []string
	errors       []ParseError
	dockerclient *client.Client
	logger       event.Logger
	Lang         code.Lang
	Runtime      bool `json:"runtime"`
	Dependencies bool `json:"dependencies"`
	Procfile     bool `json:"procfile"`
}

//CreateSourceCodeParse create parser
func CreateSourceCodeParse(source string, logger event.Logger) Parser {
	return &SourceCodeParse{
		source:  source,
		ports:   make(map[int]*Port),
		volumes: make(map[string]*Volume),
		envs:    make(map[string]*Env),
		logger:  logger,
		image:   parseImageName(builder.RUNNERIMAGENAME),
		args:    []string{"start", "web"},
	}
}

//Parse 获取代码 解析代码 检验代码
func (d *SourceCodeParse) Parse() ParseErrorList {
	if d.source == "" {
		d.logger.Error("源码检查输入参数错误", map[string]string{"step": "parse"})
		d.errappend(Errorf(FatalError, "source can not be empty"))
		return d.errors
	}
	var csi sources.CodeSourceInfo
	err := ffjson.Unmarshal([]byte(d.source), &csi)
	if err != nil {
		d.logger.Error("源码检查输入参数错误", map[string]string{"step": "parse"})
		d.errappend(Errorf(FatalError, "source data can not be read"))
		return d.errors
	}
	if csi.Branch == "" {
		csi.Branch = "master"
	}
	csi.InitServerType()
	if csi.RepositoryURL == "" {
		d.logger.Error("Git项目仓库地址不能为空", map[string]string{"step": "parse"})
		d.errappend(ErrorAndSolve(FatalError, "Git项目仓库地址格式错误", SolveAdvice("modify_url", "请确认并修改仓库地址")))
		return d.errors
	}
	//验证仓库地址
	buildInfo, err := sources.CreateRepostoryBuildInfo(csi.RepositoryURL, csi.ServerType, csi.Branch, csi.TenantID, csi.ServiceID)
	if err != nil {
		d.logger.Error("Git项目仓库地址格式错误", map[string]string{"step": "parse"})
		d.errappend(ErrorAndSolve(FatalError, "Git项目仓库地址格式错误", SolveAdvice("modify_url", "请确认并修改仓库地址")))
		return d.errors
	}
	gitFunc := func() ParseErrorList {
		//获取代码
		if sources.CheckFileExist(buildInfo.GetCodeHome()) {
			if err := sources.RemoveDir(buildInfo.GetCodeHome()); err != nil {
				//d.errappend(ErrorAndSolve(err, "清理cache dir错误", "请提交代码到仓库"))
				return d.errors
			}
		}
		csi.RepositoryURL = buildInfo.RepostoryURL
		rs, err := sources.GitClone(csi, buildInfo.GetCodeHome(), d.logger, 5)
		if err != nil {
			if err == transport.ErrAuthenticationRequired || err == transport.ErrAuthorizationFailed {
				if buildInfo.GetProtocol() == "ssh" {
					d.errappend(ErrorAndSolve(FatalError, "Git项目仓库需要安全验证", SolveAdvice("get_publickey", "请获取授权Key配置到你的仓库项目中")))
				} else {
					d.errappend(ErrorAndSolve(FatalError, "Git项目仓库需要安全验证", SolveAdvice("modify_userpass", "请提供正确的账号密码")))
				}
				return d.errors
			}
			if err == plumbing.ErrReferenceNotFound {
				solve := "请到代码仓库查看正确的分支情况"
				d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("Git项目仓库指定分支 %s 不存在", csi.Branch), solve))
				return d.errors
			}
			if err == transport.ErrRepositoryNotFound {
				solve := SolveAdvice("modify_repo", "请确认仓库地址是否正确")
				d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("Git项目仓库不存在"), solve))
				return d.errors
			}
			if err == transport.ErrEmptyRemoteRepository {
				solve := SolveAdvice("open_repo", "请确认已提交代码")
				d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("Git项目仓库无有效文件"), solve))
				return d.errors
			}
			if strings.Contains(err.Error(), "ssh: unable to authenticate") {
				solve := SolveAdvice("get_publickey", "请获取授权Key配置到你的仓库项目试试？")
				d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("远程仓库SSH验证错误"), solve))
				return d.errors
			}
			if strings.Contains(err.Error(), "context deadline exceeded") {
				solve := "请确认源码仓库能否正常访问"
				d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("获取代码超时"), solve))
				return d.errors
			}
			logrus.Errorf("git clone error,%s", err.Error())
			d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("获取代码失败"), "请确认仓库能否正常访问，或联系客服咨询"))
			return d.errors
		}
		//获取分支
		branch, err := rs.Branches()
		if err == nil {
			branch.ForEach(func(re *plumbing.Reference) error {
				name := re.Name()
				if name.IsBranch() {
					d.branchs = append(d.branchs, name.Short())
				}
				return nil
			})
		} else {
			d.branchs = append(d.branchs, csi.Branch)
		}
		return nil
	}

	svnFunc := func() ParseErrorList {
		if sources.CheckFileExist(buildInfo.GetCodeHome()) {
			if err := sources.RemoveDir(buildInfo.GetCodeHome()); err != nil {
				//d.errappend(ErrorAndSolve(err, "清理cache dir错误", "请提交代码到仓库"))
				return d.errors
			}
		}
		csi.RepositoryURL = buildInfo.RepostoryURL
		svnclient := sources.NewClient(csi.User, csi.Password, csi.RepositoryURL, buildInfo.GetCodeHome(), d.logger)
		rs, err := svnclient.Checkout()
		if err != nil {
			logrus.Errorf("svn checkout error,%s", err.Error())
			d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("获取代码失败"), "请确认仓库能否正常访问，或查看社区文档"))
			return d.errors
		}
		//get branchs
		d.branchs = rs.Branchs
		return nil
	}

	//获取代码仓库
	switch csi.ServerType {
	case "git":
		if err := gitFunc(); err != nil && err.IsFatalError() {
			return err
		}
	case "svn":
		if err := svnFunc(); err != nil && err.IsFatalError() {
			return err
		}
	default:
		//default git
		if err := gitFunc(); err != nil && err.IsFatalError() {
			return err
		}
	}

	//read rainbondfile
	rbdfileConfig, err := code.ReadRainbondFile(buildInfo.GetCodeHome())
	if err != nil {
		if err == code.ErrRainbondFileNotFound {
			d.errappend(ErrorAndSolve(NegligibleError, "rainbondfile未定义", "可以参考文档说明配置此文件定义应用属性"))
		} else {
			d.errappend(ErrorAndSolve(NegligibleError, "rainbondfile定义格式有误", "可以参考文档说明配置此文件定义应用属性"))
		}
	}
	//判断对象目录
	var buildPath = buildInfo.GetCodeBuildAbsPath()
	//解析代码类型
	var lang code.Lang
	if rbdfileConfig != nil && rbdfileConfig.Language != "" {
		lang = code.Lang(rbdfileConfig.Language)
	} else {
		lang, err = code.GetLangType(buildPath)
		if err != nil {
			if err == code.ErrCodeDirNotExist {
				d.errappend(ErrorAndSolve(FatalError, "源码目录不存在", "获取代码任务失败，请联系客服"))
			} else if err == code.ErrCodeNotExist {
				d.errappend(ErrorAndSolve(FatalError, "仓库中代码不存在", "请提交代码到仓库"))
			} else {
				d.errappend(ErrorAndSolve(FatalError, "代码无法识别语言类型", "请参考文档查看平台语言支持规范"))
			}
			return d.errors
		}
	}
	d.Lang = lang
	if lang == code.NO {
		d.errappend(ErrorAndSolve(FatalError, "代码无法识别语言类型", "请参考文档查看平台语言支持规范"))
		return d.errors
	}
	//check code Specification
	spec := code.CheckCodeSpecification(buildPath, lang)
	if spec.Advice != nil {
		for k, v := range spec.Advice {
			d.errappend(ErrorAndSolve(NegligibleError, k, v))
		}
	}
	if spec.Noconform != nil {
		for k, v := range spec.Noconform {
			d.errappend(ErrorAndSolve(FatalError, k, v))
		}
	}
	if !spec.Conform {
		return d.errors
	}
	//如果是dockerfile 解析dockerfile文件
	if lang == code.Dockerfile {
		if ok := d.parseDockerfileInfo(path.Join(buildPath, "Dockerfile")); !ok {
			return d.errors
		}
	}
	d.Dependencies = code.CheckDependencies(buildPath, lang)
	d.Runtime = code.CheckRuntime(buildPath, lang)
	d.memory = getRecommendedMemory(lang)
	d.Procfile = code.CheckProcfile(buildPath, lang)
	if rbdfileConfig != nil {
		//handle profile env
		for k, v := range rbdfileConfig.Envs {
			d.envs[k] = &Env{Name: k, Value: v}
		}
		//handle profile port
		for _, port := range rbdfileConfig.Ports {
			d.ports[port.Port] = &Port{ContainerPort: port.Port, Protocol: port.Protocol}
		}
		if rbdfileConfig.Cmd != "" {
			d.args = strings.Split(rbdfileConfig.Cmd, " ")
		}
	}
	return d.errors
}

//ReadRbdConfigAndLang read rainbondfile  and lang
func ReadRbdConfigAndLang(buildInfo *sources.RepostoryBuildInfo) (*code.RainbondFileConfig, code.Lang, error) {
	rbdfileConfig, err := code.ReadRainbondFile(buildInfo.GetCodeHome())
	if err != nil {
		return nil, code.NO, err
	}
	var lang code.Lang
	if rbdfileConfig != nil && rbdfileConfig.Language != "" {
		lang = code.Lang(rbdfileConfig.Language)
	} else {
		lang, err = code.GetLangType(buildInfo.GetCodeBuildAbsPath())
		if err != nil {
			return rbdfileConfig, code.NO, err
		}
	}
	return rbdfileConfig, lang, nil
}

func getRecommendedMemory(lang code.Lang) int {
	//java语言推荐1024
	if lang == code.JavaJar || lang == code.JavaMaven || lang == code.JaveWar {
		return 1024
	}
	if lang == code.Python {
		return 512
	}
	if lang == code.Nodejs {
		return 512
	}
	if lang == code.PHP {
		return 512
	}
	return 128
}

func (d *SourceCodeParse) errappend(pe ParseError) {
	d.errors = append(d.errors, pe)
}

//GetBranchs 获取分支列表
func (d *SourceCodeParse) GetBranchs() []string {
	return d.branchs
}

//GetPorts 获取端口列表
func (d *SourceCodeParse) GetPorts() (ports []Port) {
	for _, cv := range d.ports {
		ports = append(ports, *cv)
	}
	return ports
}

//GetVolumes 获取存储列表
func (d *SourceCodeParse) GetVolumes() (volumes []Volume) {
	for _, cv := range d.volumes {
		volumes = append(volumes, *cv)
	}
	return
}

//GetValid 获取源是否合法
func (d *SourceCodeParse) GetValid() bool {
	return false
}

//GetEnvs 环境变量
func (d *SourceCodeParse) GetEnvs() (envs []Env) {
	for _, cv := range d.envs {
		envs = append(envs, *cv)
	}
	return
}

//GetImage 获取镜像
func (d *SourceCodeParse) GetImage() Image {
	return d.image
}

//GetArgs 启动参数
func (d *SourceCodeParse) GetArgs() []string {
	return d.args
}

//GetMemory 获取内存
func (d *SourceCodeParse) GetMemory() int {
	return d.memory
}

//GetLang 获取识别语言
func (d *SourceCodeParse) GetLang() code.Lang {
	return d.Lang
}

//GetRuntime GetRuntime
func (d *SourceCodeParse) GetRuntime() bool {
	return d.Runtime
}

//GetServiceInfo 获取service info
func (d *SourceCodeParse) GetServiceInfo() []ServiceInfo {
	serviceInfo := ServiceInfo{
		Ports:        d.GetPorts(),
		Envs:         d.GetEnvs(),
		Volumes:      d.GetVolumes(),
		Image:        d.GetImage(),
		Args:         d.GetArgs(),
		Branchs:      d.GetBranchs(),
		Memory:       d.memory,
		Lang:         d.GetLang(),
		Dependencies: d.Dependencies,
		Procfile:     d.Procfile,
		Runtime:      d.Runtime,
	}
	return []ServiceInfo{serviceInfo}
}

func (d *SourceCodeParse) parseDockerfileInfo(dockerfile string) bool {
	commands, err := sources.ParseFile(dockerfile)
	if err != nil {
		d.errappend(ErrorAndSolve(FatalError, err.Error(), "请确认Dockerfile格式是否符合规范"))
		return false
	}

	for _, cm := range commands {
		switch cm.Cmd {
		case "arg":
			length := len(cm.Value)
			for i := 0; i < length; i++ {
				if kv := strings.Split(cm.Value[i], "="); len(kv) > 1 {
					key := "BUILD_ARG_" + kv[0]
					d.envs[key] = &Env{Name: key, Value: kv[1]}
				} else {
					if i+1 >= length {
						logrus.Error("Parse ARG format error at ", cm.Value[1])
						continue
					}
					key := "BUILD_ARG_" + cm.Value[i]
					d.envs[key] = &Env{Name: key, Value: cm.Value[i+1]}
					i++
				}
			}
		case "env":
			length := len(cm.Value)
			for i := 0; i < len(cm.Value); i++ {
				if kv := strings.Split(cm.Value[i], "="); len(kv) > 1 {
					d.envs[kv[0]] = &Env{Name: kv[0], Value: kv[1]}
				} else {
					if i+1 >= length {
						logrus.Error("Parse ENV format error at ", cm.Value[1])
						continue
					}
					d.envs[cm.Value[i]] = &Env{Name: cm.Value[i], Value: cm.Value[i+1]}
					i++
				}
			}
		case "expose":
			for _, v := range cm.Value {
				port, _ := strconv.Atoi(v)
				if port != 0 {
					d.ports[port] = &Port{ContainerPort: port, Protocol: GetPortProtocol(port)}
				}
			}
		case "volume":
			for _, v := range cm.Value {
				d.volumes[v] = &Volume{VolumePath: v, VolumeType: model.ShareFileVolumeType.String()}
			}
		}
	}
	// dockerfile empty args
	d.args = []string{}
	return true
}

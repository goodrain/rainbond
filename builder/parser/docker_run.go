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
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/docker/distribution/reference" //"github.com/docker/docker/api/types"
	"github.com/goodrain/rainbond/builder/parser/types"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/net/context"
	"runtime"
	"strconv"
	"strings" //"github.com/docker/docker/client"
)

//DockerRunOrImageParse docker run 命令解析或直接镜像名解析
type DockerRunOrImageParse struct {
	user, pass  string
	ports       map[int]*types.Port
	volumes     map[string]*types.Volume
	envs        map[string]*types.Env
	source      string
	serviceType string
	memory      int
	image       Image
	args        []string
	errors      []ParseError
	//containerdClient *containerd.Client
	imageClient sources.ImageClient
	logger      event.Logger
}

//CreateDockerRunOrImageParse create parser
func CreateDockerRunOrImageParse(user, pass, source string, imageClient sources.ImageClient, logger event.Logger) *DockerRunOrImageParse {
	source = strings.TrimLeft(source, " ")
	source = strings.Replace(source, "\n", "", -1)
	source = strings.Replace(source, "\\", "", -1)
	source = strings.Replace(source, "  ", " ", -1)
	return &DockerRunOrImageParse{
		user:        user,
		pass:        pass,
		source:      source,
		imageClient: imageClient,
		ports:       make(map[int]*types.Port),
		volumes:     make(map[string]*types.Volume),
		envs:        make(map[string]*types.Env),
		logger:      logger,
	}
}

//Parse 解码，获取镜像，解析镜像
//eg. docker run -it -p 80:80 nginx
func (d *DockerRunOrImageParse) Parse() ParseErrorList {
	if d.source == "" {
		d.errappend(Errorf(FatalError, "source can not be empty"))
		return d.errors
	}
	//docker run
	if strings.HasPrefix(d.source, "docker") {
		d.ParseDockerun(d.source)
		if d.image.String() == "" || d.image.String() == ":" {
			d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("镜像名称识别失败"), SolveAdvice("modify_image", "请确认输入DockerRun命令是否正确")))
			return d.errors
		}
		if _, err := reference.ParseAnyReference(d.image.String()); err != nil {
			d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("镜像名称(%s)不合法", d.image.String()), SolveAdvice("modify_image", "请确认输入DockerRun命令是否正确")))
			return d.errors
		}
	} else {
		//else image
		_, err := reference.ParseAnyReference(d.source)
		if err != nil {
			d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("镜像名称(%s)不合法", d.image.String()), SolveAdvice("modify_image", "请确认输入镜像名是否正确")))
			return d.errors
		}
		d.image = ParseImageName(d.source)
	}
	//获取镜像，验证是否存在
	imageInspect, err := d.imageClient.ImagePull(d.image.Source(), d.user, d.pass, d.logger, 10)
	if err != nil {
		if strings.Contains(err.Error(), "No such image") {
			d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("镜像(%s)不存在", d.image.String()), SolveAdvice("modify_image", "请确认输入镜像名是否正确")))
		} else {
			if d.image.IsOfficial() {
				d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("镜像(%s)获取失败,国内访问Docker官方仓库经常不稳定", d.image.String()), SolveAdvice("modify_image", "请确认输入镜像可以正常获取")))
			} else {
				d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("镜像(%s)获取失败", d.image.String()), SolveAdvice("modify_image", "请确认输入镜像可以正常获取")))
			}
		}
		return d.errors
	}
	if imageInspect != nil {
		for _, env := range imageInspect.Env {
			envinfo := strings.Split(env, "=")
			if len(envinfo) == 2 {
				if _, ok := d.envs[envinfo[0]]; !ok {
					d.envs[envinfo[0]] = &types.Env{Name: envinfo[0], Value: envinfo[1]}
				}
			}
		}
		for k := range imageInspect.Volumes {
			if _, ok := d.volumes[k]; !ok {
				d.volumes[k] = &types.Volume{VolumePath: k, VolumeType: model.ShareFileVolumeType.String()}
			}
		}
		for k := range imageInspect.ExposedPorts {
			res := strings.Split(k, "/")
			if len(res) > 2 {
				fmt.Println("The exposedPorts format is incorrect")
			}
			proto := res[1]
			port, err := strconv.Atoi(res[0])
			if err != nil {
				fmt.Println("port int error", err)
				return nil
			}
			if proto != "udp" {
				proto = GetPortProtocol(port)
			}
			if _, ok := d.ports[port]; ok {
				d.ports[port].Protocol = proto
			} else {
				d.ports[port] = &types.Port{Protocol: proto, ContainerPort: port}
			}
		}
	}
	d.serviceType = DetermineDeployType(d.image)
	return d.errors
}

//ParseDockerun parse docker run command
func (d *DockerRunOrImageParse) ParseDockerun(cmd string) {
	var name string
	cmd = strings.TrimLeft(cmd, " ")
	cmd = strings.Replace(cmd, "\n", "", -1)
	cmd = strings.Replace(cmd, "\r", "", -1)
	cmd = strings.Replace(cmd, "\t", "", -1)
	cmd = strings.Replace(cmd, "\\", "", -1)
	cmd = strings.Replace(cmd, "  ", " ", -1)
	source := util.RemoveSpaces(strings.Split(cmd, " "))
	for i, s := range source {
		if s == "docker" || s == "run" {
			continue
		}
		if strings.HasPrefix(s, "-") {
			name = strings.TrimLeft(s, "-")
			index := strings.Index(name, "=")
			if index > 0 && index < len(name)-1 {
				s = name[index+1:]
				name = name[0:index]
				switch name {
				case "e", "env":
					info := strings.Split(s, "=")
					if len(info) == 2 {
						d.envs[info[0]] = &types.Env{Name: info[0], Value: info[1]}
					}
				case "p", "public":
					info := strings.Split(s, ":")
					if len(info) == 2 {
						port, _ := strconv.Atoi(info[0])
						if port != 0 {
							d.ports[port] = &types.Port{ContainerPort: port, Protocol: GetPortProtocol(port)}
						}
					}
				case "v", "volume":
					info := strings.Split(s, ":")
					if len(info) >= 2 {
						d.volumes[info[1]] = &types.Volume{VolumePath: info[1], VolumeType: model.ShareFileVolumeType.String()}
					}
				case "memory", "m":
					d.memory = readmemory(s)
				}
				name = ""
			}
		} else {
			switch name {
			case "e", "env":
				info := strings.Split(removeQuotes(s), "=")
				if len(info) == 2 {
					d.envs[info[0]] = &types.Env{Name: info[0], Value: info[1]}
				}
			case "p", "public":
				info := strings.Split(removeQuotes(s), ":")
				if len(info) == 2 {
					port, _ := strconv.Atoi(info[1])
					if port != 0 {
						d.ports[port] = &types.Port{ContainerPort: port, Protocol: GetPortProtocol(port)}
					}
				}
			case "v", "volume":
				info := strings.Split(removeQuotes(s), ":")
				if len(info) >= 2 {
					d.volumes[info[1]] = &types.Volume{VolumePath: info[1], VolumeType: model.ShareFileVolumeType.String()}
				}
			case "memory", "m":
				d.memory = readmemory(s)
			case "", "d", "i", "t", "it", "P", "rm", "init", "interactive", "no-healthcheck", "oom-kill-disable", "privileged", "read-only", "tty", "sig-proxy":
				d.image = ParseImageName(s)
				if len(source) > i+1 {
					d.args = source[i+1:]
				}
				return
			}
			name = ""
			continue
		}
	}

}

func (d *DockerRunOrImageParse) errappend(pe ParseError) {
	d.errors = append(d.errors, pe)
}

//GetBranchs 获取分支列表
func (d *DockerRunOrImageParse) GetBranchs() []string {
	return nil
}

//GetPorts 获取端口列表
func (d *DockerRunOrImageParse) GetPorts() (ports []types.Port) {
	for _, cv := range d.ports {
		ports = append(ports, *cv)
	}
	return ports
}

//GetVolumes 获取存储列表
func (d *DockerRunOrImageParse) GetVolumes() (volumes []types.Volume) {
	for _, cv := range d.volumes {
		volumes = append(volumes, *cv)
	}
	return
}

//GetValid 获取源是否合法
func (d *DockerRunOrImageParse) GetValid() bool {
	return false
}

//GetEnvs 环境变量
func (d *DockerRunOrImageParse) GetEnvs() (envs []types.Env) {
	for _, cv := range d.envs {
		envs = append(envs, *cv)
	}
	return
}

//GetImage 获取镜像
func (d *DockerRunOrImageParse) GetImage() Image {
	return d.image
}

//GetArgs 启动参数
func (d *DockerRunOrImageParse) GetArgs() []string {
	return d.args
}

//GetMemory 获取内存
func (d *DockerRunOrImageParse) GetMemory() int {
	return d.memory
}

//GetServiceInfo 获取service info
func (d *DockerRunOrImageParse) GetServiceInfo() []ServiceInfo {
	serviceInfo := ServiceInfo{
		Ports:       d.GetPorts(),
		Envs:        d.GetEnvs(),
		Volumes:     d.GetVolumes(),
		Image:       d.GetImage(),
		Args:        d.GetArgs(),
		Branchs:     d.GetBranchs(),
		Memory:      d.memory,
		ServiceType: d.serviceType,
		OS:          runtime.GOOS,
	}
	if serviceInfo.Memory == 0 {
		serviceInfo.Memory = 512
	}
	return []ServiceInfo{serviceInfo}
}

func getImageConfig(ctx context.Context, image containerd.Image) (*ocispec.ImageConfig, error) {
	desc, err := image.Config(ctx)
	if err != nil {
		return nil, err
	}
	switch desc.MediaType {
	case ocispec.MediaTypeImageConfig, images.MediaTypeDockerSchema2Config:
		var ocispecImage ocispec.Image
		b, err := content.ReadBlob(ctx, image.ContentStore(), desc)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(b, &ocispecImage); err != nil {
			return nil, err
		}
		return &ocispecImage.Config, nil
	default:
		return nil, fmt.Errorf("unknown media type %q", desc.MediaType)
	}
}

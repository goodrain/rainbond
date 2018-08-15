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
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/util"

	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"

	"github.com/docker/distribution/reference"
	//"github.com/docker/docker/api/types"

	//"github.com/docker/docker/client"
	"github.com/docker/engine-api/client"
)

//DockerRunOrImageParse docker run 命令解析或直接镜像名解析
type DockerRunOrImageParse struct {
	user, pass   string
	ports        map[int]*Port
	volumes      map[string]*Volume
	envs         map[string]*Env
	source       string
	memory       int
	image        Image
	args         []string
	errors       []ParseError
	dockerclient *client.Client
	logger       event.Logger
}

//CreateDockerRunOrImageParse create parser
func CreateDockerRunOrImageParse(user, pass, source string, dockerclient *client.Client, logger event.Logger) Parser {
	source = strings.TrimLeft(source, " ")
	source = strings.Replace(source, "\n", "", -1)
	source = strings.Replace(source, "\\", "", -1)
	source = strings.Replace(source, "  ", " ", -1)
	return &DockerRunOrImageParse{
		user:         user,
		pass:         pass,
		source:       source,
		dockerclient: dockerclient,
		ports:        make(map[int]*Port),
		volumes:      make(map[string]*Volume),
		envs:         make(map[string]*Env),
		logger:       logger,
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
		d.dockerun(strings.Split(d.source, " "))
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
		d.image = parseImageName(d.source)
	}
	//获取镜像，验证是否存在
	imageInspect, err := sources.ImagePull(d.dockerclient, d.image.String(), d.user, d.pass, d.logger, 10)
	if err != nil {
		if strings.Contains(err.Error(), "No such image") {
			d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("镜像(%s)不存在", d.image.String()), SolveAdvice("modify_image", "请确认输入镜像名是否正确")))
		} else {
			d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("镜像(%s)获取失败", d.image.String()), SolveAdvice("modify_image", "请确认输入镜像可以正常获取")))
		}
		return d.errors
	}
	if imageInspect != nil && imageInspect.ContainerConfig != nil {
		for _, env := range imageInspect.ContainerConfig.Env {
			envinfo := strings.Split(env, "=")
			if len(envinfo) == 2 {
				if _, ok := d.envs[envinfo[0]]; !ok {
					d.envs[envinfo[0]] = &Env{Name: envinfo[0], Value: envinfo[1]}
				}
			}
		}
		for k := range imageInspect.ContainerConfig.Volumes {
			if _, ok := d.volumes[k]; !ok {
				d.volumes[k] = &Volume{VolumePath: k, VolumeType: model.ShareFileVolumeType.String()}
			}
		}
		for k := range imageInspect.ContainerConfig.ExposedPorts {
			proto := k.Proto()
			port := k.Int()
			if proto != "udp" {
				proto = GetPortProtocol(port)
			}
			if _, ok := d.ports[port]; ok {
				d.ports[port].Protocol = proto
			} else {
				d.ports[port] = &Port{Protocol: proto, ContainerPort: port}
			}
		}
	}
	return d.errors
}

func (d *DockerRunOrImageParse) dockerun(source []string) {
	var name string
	source = util.RemoveSpaces(source)
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
						d.envs[info[0]] = &Env{Name: info[0], Value: info[1]}
					}
				case "p", "public":
					info := strings.Split(s, ":")
					if len(info) == 2 {
						port, _ := strconv.Atoi(info[0])
						if port != 0 {
							d.ports[port] = &Port{ContainerPort: port, Protocol: GetPortProtocol(port)}
						}
					}
				case "v", "volume":
					info := strings.Split(s, ":")
					if len(info) >= 2 {
						d.volumes[info[1]] = &Volume{VolumePath: info[1], VolumeType: model.ShareFileVolumeType.String()}
					}
				case "memory", "m":
					d.memory = readmemory(s)
				}
				name = ""
			}
		} else {
			switch name {
			case "e", "env":
				info := strings.Split(s, "=")
				if len(info) == 2 {
					d.envs[info[0]] = &Env{Name: info[0], Value: info[1]}
				}
			case "p", "public":
				info := strings.Split(s, ":")
				if len(info) == 2 {
					port, _ := strconv.Atoi(info[1])
					if port != 0 {
						d.ports[port] = &Port{ContainerPort: port, Protocol: GetPortProtocol(port)}
					}
				}
			case "v", "volume":
				info := strings.Split(s, ":")
				if len(info) >= 2 {
					d.volumes[info[1]] = &Volume{VolumePath: info[1], VolumeType: model.ShareFileVolumeType.String()}
				}
			case "memory", "m":
				d.memory = readmemory(s)
			case "", "d", "i", "t", "it", "P":
				d.image = parseImageName(s)
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
func (d *DockerRunOrImageParse) GetPorts() (ports []Port) {
	for _, cv := range d.ports {
		ports = append(ports, *cv)
	}
	return ports
}

//GetVolumes 获取存储列表
func (d *DockerRunOrImageParse) GetVolumes() (volumes []Volume) {
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
func (d *DockerRunOrImageParse) GetEnvs() (envs []Env) {
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
		Ports:   d.GetPorts(),
		Envs:    d.GetEnvs(),
		Volumes: d.GetVolumes(),
		Image:   d.GetImage(),
		Args:    d.GetArgs(),
		Branchs: d.GetBranchs(),
		Memory:  d.memory,
	}
	if serviceInfo.Memory == 0 {
		serviceInfo.Memory = 256
	}
	return []ServiceInfo{serviceInfo}
}

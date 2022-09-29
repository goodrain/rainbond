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
	"runtime"
	"strings"

	"github.com/docker/docker/client"
	"github.com/goodrain/rainbond/builder/parser/compose"
	"github.com/goodrain/rainbond/builder/parser/types"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/sirupsen/logrus"
)

//DockerComposeParse docker compose 文件解析
type DockerComposeParse struct {
	services     map[string]*ServiceInfoFromDC
	errors       []ParseError
	dockerclient *client.Client
	logger       event.Logger
	source       string
	user         string
	password     string
}

//ServiceInfoFromDC service info from dockercompose
type ServiceInfoFromDC struct {
	ports       map[int]*types.Port
	volumes     map[string]*types.Volume
	envs        map[string]*types.Env
	source      string
	memory      int
	image       Image
	args        []string
	depends     []string
	imageAlias  string
	serviceType string
	name        string
}

//GetPorts 获取端口列表
func (d *ServiceInfoFromDC) GetPorts() (ports []types.Port) {
	for _, cv := range d.ports {
		ports = append(ports, *cv)
	}
	return ports
}

//GetVolumes 获取存储列表
func (d *ServiceInfoFromDC) GetVolumes() (volumes []types.Volume) {
	for _, cv := range d.volumes {
		volumes = append(volumes, *cv)
	}
	return
}

//GetEnvs 环境变量
func (d *ServiceInfoFromDC) GetEnvs() (envs []types.Env) {
	for _, cv := range d.envs {
		envs = append(envs, *cv)
	}
	return
}

//CreateDockerComposeParse create parser
func CreateDockerComposeParse(source string, user, pass string, logger event.Logger) Parser {
	return &DockerComposeParse{
		source:   source,
		logger:   logger,
		services: make(map[string]*ServiceInfoFromDC),
		user:     user,
		password: pass,
	}
}

//Parse 解码
func (d *DockerComposeParse) Parse() ParseErrorList {
	if d.source == "" {
		d.errappend(Errorf(FatalError, "source can not be empty"))
		return d.errors
	}
	comp := compose.Compose{}
	co, err := comp.LoadBytes([][]byte{[]byte(d.source)})
	if err != nil {
		logrus.Warning("parse compose file error,", err.Error())
		d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("ComposeFile解析错误"), SolveAdvice("modify_compose", "请确认ComposeFile输入是否语法正确")))
		return d.errors
	}
	for kev, sc := range co.ServiceConfigs {
		logrus.Debugf("service config is %v, container name is %s", sc, sc.ContainerName)
		ports := make(map[int]*types.Port)

		if sc.Image == "" {
			d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("ComposeFile解析错误"), SolveAdvice(fmt.Sprintf("Service %s has no image specified", kev), fmt.Sprintf("请为%s指定一个镜像", kev))))
			continue
		}

		for _, p := range sc.Port {
			pro := string(p.Protocol)
			if pro != "udp" {
				pro = GetPortProtocol(int(p.ContainerPort))
			}
			ports[int(p.ContainerPort)] = &types.Port{
				ContainerPort: int(p.ContainerPort),
				Protocol:      pro,
			}
		}
		volumes := make(map[string]*types.Volume)
		for _, v := range sc.Volumes {
			if strings.Contains(v.MountPath, ":") {
				infos := strings.Split(v.MountPath, ":")
				if len(infos) > 1 {
					volumes[v.MountPath] = &types.Volume{
						VolumePath: infos[1],
						VolumeType: model.ShareFileVolumeType.String(),
					}
				}
			} else {
				volumes[v.MountPath] = &types.Volume{
					VolumePath: v.MountPath,
					VolumeType: model.ShareFileVolumeType.String(),
				}
			}
		}
		envs := make(map[string]*types.Env)
		for _, e := range sc.Environment {
			envs[e.Name] = &types.Env{
				Name:  e.Name,
				Value: e.Value,
			}
		}
		service := ServiceInfoFromDC{
			ports:      ports,
			volumes:    volumes,
			envs:       envs,
			memory:     int(sc.MemLimit / 1024 / 1024),
			image:      ParseImageName(sc.Image),
			args:       sc.Args,
			depends:    sc.Links,
			imageAlias: sc.ContainerName,
			name:       kev,
		}
		if sc.DependsON != nil {
			service.depends = sc.DependsON
		}
		service.serviceType = DetermineDeployType(service.image)
		d.services[kev] = &service
	}
	for serviceName, service := range d.services {
		//验证depends是否完整
		existDepends := []string{}
		for i, depend := range service.depends {
			if strings.Contains(depend, ":") {
				service.depends[i] = strings.Split(depend, ":")[0]
			}
			if _, ok := d.services[service.depends[i]]; !ok {
				d.errappend(ErrorAndSolve(NegligibleError, fmt.Sprintf("服务%s依赖项定义错误", serviceName), SolveAdvice("modify_compose", fmt.Sprintf("请确认%s服务的依赖服务是否正确", serviceName))))
			} else {
				existDepends = append(existDepends, service.depends[i])
			}
		}
		service.depends = existDepends
		var hubUser = d.user
		var hubPass = d.password
		for _, env := range service.GetEnvs() {
			if env.Name == "HUB_USER" {
				hubUser = env.Value
			}
			if env.Name == "HUB_PASSWORD" {
				hubPass = env.Value
			}
		}
		//do not pull image, but check image exist
		d.logger.Debug(fmt.Sprintf("start check service %s ", service.name), map[string]string{"step": "service_check", "status": "running"})
		exist, err := sources.ImageExist(service.image.String(), hubUser, hubPass)
		if err != nil {
			logrus.Errorf("check image(%s) exist failure %s", service.image.String(), err.Error())
		}
		if !exist {
			d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("服务%s镜像%s检测失败", serviceName, service.image.String()), SolveAdvice("modify_compose", fmt.Sprintf("请确认%s服务镜像名称是否正确或镜像仓库访问是否正常", serviceName))))
		}
	}
	return d.errors
}

func (d *DockerComposeParse) errappend(pe ParseError) {
	d.errors = append(d.errors, pe)
}

//GetServiceInfo 获取service info
func (d *DockerComposeParse) GetServiceInfo() []ServiceInfo {
	var sis []ServiceInfo
	for _, service := range d.services {
		si := ServiceInfo{
			Ports:          service.GetPorts(),
			Envs:           service.GetEnvs(),
			Volumes:        service.GetVolumes(),
			Image:          service.image,
			Args:           service.args,
			DependServices: service.depends,
			ImageAlias:     service.imageAlias,
			ServiceType:    service.serviceType,
			Name:           service.name,
			Cname:          service.name,
			OS:             runtime.GOOS,
		}
		if service.memory != 0 {
			si.Memory = service.memory
		} else {
			si.Memory = 512
		}
		sis = append(sis, si)
	}
	return sis
}

//GetImage 获取镜像名
func (d *DockerComposeParse) GetImage() Image {
	return Image{}
}

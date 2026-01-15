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
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings" //"github.com/docker/docker/client"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/docker/distribution/reference" //"github.com/docker/docker/api/types"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/parser/types"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/pkg/component/storage"
	"github.com/goodrain/rainbond/util"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// DockerRunOrImageParse docker run 命令解析或直接镜像名解析
type DockerRunOrImageParse struct {
	user, pass  string
	ports       map[int]*types.Port
	volumes     map[string]*types.Volume
	envs        map[string]*types.Env
	source      string
	serviceType string
	memory      int
	image       Image
	namespace   string
	args        []string
	tarImages   []*types.Image
	errors      []ParseError
	imageClient sources.ImageClient
	logger      event.Logger
}

// CreateDockerRunOrImageParse create parser
func CreateDockerRunOrImageParse(user, pass, source string, imageClient sources.ImageClient, logger event.Logger, namespace string) *DockerRunOrImageParse {
	source = strings.TrimLeft(source, " ")
	source = strings.Replace(source, "\n", "", -1)
	source = strings.Replace(source, "\\", "", -1)
	source = strings.Replace(source, "  ", " ", -1)
	return &DockerRunOrImageParse{
		user:        user,
		pass:        pass,
		source:      source,
		imageClient: imageClient,
		namespace:   namespace,
		ports:       make(map[int]*types.Port),
		volumes:     make(map[string]*types.Volume),
		envs:        make(map[string]*types.Env),
		logger:      logger,
	}
}

// Parse 解码，获取镜像，解析镜像
// eg. docker run -it -p 80:80 nginx
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
	} else if strings.HasPrefix(d.source, "event") {
		eventID := strings.Split(d.source, " ")[1]
		tarPath := path.Join("/grdata/package_build/temp/events", eventID)
		err := storage.Default().StorageCli.DownloadDirToDir(tarPath, tarPath)
		files, _ := filepath.Glob(path.Join(tarPath, "*"))
		if len(files) == 1 {
			if !strings.HasSuffix(files[0], ".tar") && !strings.HasSuffix(files[0], ".tar.gz") {
				d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("文件格式不正确"), SolveAdvice("modify_image", "请确认上传的文件格式是否正确")))
				return d.errors
			}
		} else {
			d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("镜像文件数超出限制"), SolveAdvice("modify_image", "请确认上传文件数是否为1")))
			return d.errors
		}
		imageNames, err := d.imageClient.ImageLoad(files[0], d.logger)
		if err != nil {
			d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("镜像解析失败"), SolveAdvice("modify_image", "请检查上传的tar包是否正确")))
			return d.errors
		}
		// 检查是否加载了有效的镜像
		if len(imageNames) == 0 {
			d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("tar包中没有找到有效的镜像"), SolveAdvice("modify_image", "请确认上传的tar包中包含有效的镜像文件")))
			return d.errors
		}
		// 过滤空镜像名并验证格式
		var validImageNames []string
		for _, imageName := range imageNames {
			if strings.TrimSpace(imageName) == "" {
				logrus.Warnf("skip empty image name from tar file")
				continue
			}
			if _, err := reference.ParseAnyReference(imageName); err != nil {
				logrus.Errorf("invalid image name format: %s, error: %v", imageName, err)
				d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("镜像名称格式无效: %s", imageName), SolveAdvice("modify_image", "请确认tar包中的镜像格式是否正确")))
				return d.errors
			}
			validImageNames = append(validImageNames, imageName)
		}
		if len(validImageNames) == 0 {
			d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("tar包中没有找到格式正确的镜像"), SolveAdvice("modify_image", "请确认上传的tar包中包含有效的镜像文件")))
			return d.errors
		}
		imageNames = validImageNames
		imagePrefix := path.Join(builder.REGISTRYDOMAIN, d.namespace)
		var tarImages []*types.Image
		for _, imageName := range imageNames {
			name := imageName
			tarImages = append(tarImages, &types.Image{
				Name:   name,
				Prefix: imagePrefix,
			})
		}
		d.tarImages = tarImages
		for _, imageName := range imageNames {
			imageList := strings.Split(imageName, "/")
			var newImageName string
			if len(imageList) == 1 {
				newImageName = path.Join(builder.REGISTRYDOMAIN, d.namespace, imageList[0])
			} else if len(imageList) == 2 {
				newImageName = path.Join(builder.REGISTRYDOMAIN, d.namespace, imageList[1])
			} else if len(imageList) == 3 {
				newImageName = path.Join(builder.REGISTRYDOMAIN, d.namespace, imageList[2])
			}
			err := d.imageClient.ImageTag(imageName, newImageName, d.logger, 3)
			if err != nil {
				logrus.Errorf("tag tar image failure: %v", err)
				d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("镜像修改 tag 失败"), SolveAdvice("modify_image", "请联系平台管理员")))
				return d.errors
			}
			err = d.imageClient.ImagePush(newImageName, builder.REGISTRYUSER, builder.REGISTRYPASS, d.logger, 3)
			if err != nil {
				logrus.Errorf("load tar image push failure: %v", err)
				d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("镜像push 失败"), SolveAdvice("modify_image", "请联系平台管理员")))
				return d.errors
			}
		}
		return d.errors
	} else {
		//else image
		_, err := reference.ParseAnyReference(d.source)
		if err != nil {
			d.errappend(ErrorAndSolve(FatalError, fmt.Sprintf("镜像名称(%s)不合法", d.image.String()), SolveAdvice("modify_image", "请确认输入镜像名是否正确")))
			return d.errors
		}
		d.image = ParseImageName(d.source)
	}
	// ========== 轻量级镜像元数据检测 ==========
	user := d.user
	pass := d.pass

	// 如果是内部镜像仓库，使用默认凭证
	if strings.HasPrefix(d.image.Source(), builder.REGISTRYDOMAIN) {
		if user == "" {
			user = builder.REGISTRYUSER
		}
		if pass == "" {
			pass = builder.REGISTRYPASS
		}
	}

	// 调用新方法获取元数据（轻量级，不下载镜像层）
	imageConfig, err := d.imageClient.GetImageMetadata(d.image.Source(), user, pass, d.logger)
	if err != nil {
		// 检测失败：记录警告但继续构建（降级策略）
		d.logger.Info(
			fmt.Sprintf("无法获取镜像元数据: %v (将在部署时从实际镜像获取)", err),
			map[string]string{"step": "image-parse", "status": "warning"})
		logrus.Warnf("[LightweightDetect] Failed to get metadata for %s: %v", d.image.Source(), err)
	} else {
		// 检测成功：提取元数据
		d.logger.Info(
			fmt.Sprintf("成功获取镜像元数据: %s", d.image.Source()),
			map[string]string{"step": "image-parse"})

		if imageConfig != nil {
			// 1. 提取环境变量
			for _, env := range imageConfig.Env {
				envInfo := strings.Split(env, "=")
				if len(envInfo) == 2 {
					// 只添加用户未明确指定的环境变量
					if _, exists := d.envs[envInfo[0]]; !exists {
						d.envs[envInfo[0]] = &types.Env{Name: envInfo[0], Value: envInfo[1]}
					}
				}
			}

			// 2. 提取卷信息
			for volumePath := range imageConfig.Volumes {
				if _, exists := d.volumes[volumePath]; !exists {
					d.volumes[volumePath] = &types.Volume{
						VolumePath: volumePath,
						VolumeType: model.ShareFileVolumeType.String(),
					}
				}
			}

			// 3. 提取暴露端口
			for portSpec := range imageConfig.ExposedPorts {
				// 解析端口格式：例如 "80/tcp" 或 "53/udp"
				parts := strings.Split(portSpec, "/")
				if len(parts) >= 1 {
					portNum, err := strconv.Atoi(parts[0])
					if err != nil {
						logrus.Warnf("[LightweightDetect] Invalid port format: %s", portSpec)
						continue
					}

					// 确定协议
					protocol := "tcp"
					if len(parts) >= 2 && parts[1] == "udp" {
						protocol = "udp"
					} else {
						protocol = GetPortProtocol(portNum)
					}

					// 更新或添加端口
					if existingPort, exists := d.ports[portNum]; exists {
						existingPort.Protocol = protocol
					} else {
						d.ports[portNum] = &types.Port{
							ContainerPort: portNum,
							Protocol:      protocol,
						}
					}
				}
			}

			logrus.Debugf("[LightweightDetect] Extracted: %d envs, %d volumes, %d ports",
				len(imageConfig.Env), len(imageConfig.Volumes), len(imageConfig.ExposedPorts))
		}
	}
	// ========== 元数据检测结束 ==========

	d.serviceType = DetermineDeployType(d.image)
	return d.errors
}

// ParseDockerun parse docker run command
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

// GetBranchs 获取分支列表
func (d *DockerRunOrImageParse) GetBranchs() []string {
	return nil
}

// GetPorts 获取端口列表
func (d *DockerRunOrImageParse) GetPorts() (ports []types.Port) {
	for _, cv := range d.ports {
		ports = append(ports, *cv)
	}
	return ports
}

// GetVolumes 获取存储列表
func (d *DockerRunOrImageParse) GetVolumes() (volumes []types.Volume) {
	for _, cv := range d.volumes {
		volumes = append(volumes, *cv)
	}
	return
}

// GetValid 获取源是否合法
func (d *DockerRunOrImageParse) GetValid() bool {
	return false
}

// GetEnvs 环境变量
func (d *DockerRunOrImageParse) GetEnvs() (envs []types.Env) {
	for _, cv := range d.envs {
		envs = append(envs, *cv)
	}
	return
}

// GetImage 获取镜像
func (d *DockerRunOrImageParse) GetImage() Image {
	return d.image
}

// GetArgs 启动参数
func (d *DockerRunOrImageParse) GetArgs() []string {
	return d.args
}

// GetTarImages 获取 tar 包解析出来的镜像
func (d *DockerRunOrImageParse) GetTarImages() []*types.Image {
	return d.tarImages
}

// GetMemory 获取内存
func (d *DockerRunOrImageParse) GetMemory() int {
	return d.memory
}

// GetServiceInfo 获取service info
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
		TarImages:   d.GetTarImages(),
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

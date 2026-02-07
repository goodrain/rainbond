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
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/docker/docker/client"
	"github.com/goodrain/rainbond/builder/parser/compose"
	"github.com/goodrain/rainbond/builder/parser/types"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/pkg/component/storage"
	"github.com/goodrain/rainbond/util"
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
	// 新增字段：支持项目包上传
	eventID         string
	composeFilePath string
	projectPath     string
	composeDir      string // compose 文件所在目录，volume 相对路径基于此目录
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

//CreateDockerComposeParseFromProject create parser from project package
func CreateDockerComposeParseFromProject(eventID, composeFilePath, user, pass string, logger event.Logger) Parser {
	return &DockerComposeParse{
		eventID:         eventID,
		composeFilePath: composeFilePath,
		logger:          logger,
		services:        make(map[string]*ServiceInfoFromDC),
		user:            user,
		password:        pass,
	}
}

//Parse 解码
func (d *DockerComposeParse) Parse() ParseErrorList {
	// 1. 如果是项目包方式，从 S3 下载并解压
	if d.eventID != "" {
		if err := d.downloadAndExtractProject(); err != nil {
			d.errappend(Errorf(FatalError, "下载或解压项目失败: "+err.Error()))
			return d.errors
		}
		// 读取 compose 文件内容
		if err := d.loadComposeFile(); err != nil {
			d.errappend(Errorf(FatalError, "读取 compose 文件失败: "+err.Error()))
			return d.errors
		}
	}

	// 2. 验证 source 不为空
	if d.source == "" {
		d.errappend(Errorf(FatalError, "source can not be empty"))
		return d.errors
	}

	// 3. 解析 compose 文件
	comp := compose.Compose{}
	co, err := comp.LoadBytesWithWorkDir([][]byte{[]byte(d.source)}, d.composeDir)
	if err != nil {
		logrus.Warning("parse compose file error,", err.Error())
		// 将详细的错误信息包含在响应中
		errorInfo := fmt.Sprintf("ComposeFile解析错误: %s", err.Error())
		d.errappend(ErrorAndSolve(FatalError, errorInfo, SolveAdvice("modify_compose", "请检查 Docker Compose 文件格式是否正确")))
		return d.errors
	}

	// Process field support report if available
	if co.SupportReport != nil && co.SupportReport.HasIssues() {
		d.logFieldSupportReport(co.SupportReport)
		// Also add warnings to error list for API response
		d.addSupportReportToErrors(co.SupportReport)
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
			targetPath := v.MountPath  // 容器内挂载路径
			sourcePath := v.Host       // 宿主机/项目中的源路径
			volumeType := model.ShareFileVolumeType.String()
			fileContent := ""

			// Check if compose parser already identified this as a config file
			if v.VolumeType == "config-file" {
				volumeType = model.ConfigFileVolumeType.String()
			}

			// 如果有源路径，尝试从项目中读取文件内容
			// compose-go 会把相对路径解析为基于 WorkingDir 的绝对路径
			// 当使用项目包模式时，WorkingDir 就是 composeDir，所以 sourcePath 已经是正确的绝对路径
			if sourcePath != "" && d.composeDir != "" {
				resolvedPath := sourcePath
				if !path.IsAbs(sourcePath) {
					resolvedPath = path.Join(d.composeDir, sourcePath)
				}

				fileInfo, err := os.Stat(resolvedPath)
				if err == nil {
					if fileInfo.IsDir() {
						volumeType = model.ShareFileVolumeType.String()
						logrus.Infof("detected directory volume: %s -> %s", sourcePath, targetPath)
					} else if fileInfo.Size() > 1<<20 {
						// 超过 1MB，ConfigMap 不支持，直接忽略
						logrus.Infof("skipping volume (size %d > 1MB): %s -> %s", fileInfo.Size(), sourcePath, targetPath)
						continue
					} else if fileInfo.Size() == 0 {
						// 空文件，直接忽略
						logrus.Infof("skipping empty config file: %s -> %s", sourcePath, targetPath)
						continue
					} else {
						volumeType = model.ConfigFileVolumeType.String()
						content, readErr := ioutil.ReadFile(resolvedPath)
						if readErr == nil {
							fileContent = string(content)
							logrus.Infof("detected config file volume: %s -> %s, size: %d bytes", sourcePath, targetPath, len(content))
						} else {
							logrus.Warnf("failed to read config file %s: %v", resolvedPath, readErr)
						}
					}
				} else {
					logrus.Warnf("volume source not found: %s (resolved: %s)", sourcePath, resolvedPath)
				}
			}

			volumes[targetPath] = &types.Volume{
				VolumePath:  targetPath,
				VolumeType:  volumeType,
				FileContent: fileContent,
			}
		}
		envs := make(map[string]*types.Env)
		for _, e := range sc.Environment {
			envs[e.Name] = &types.Env{
				Name:  e.Name,
				Value: e.Value,
			}
		}

		// 处理 env_file：读取项目中的环境变量文件
		if d.composeDir != "" && len(sc.EnvFile) > 0 {
			for _, envFile := range sc.EnvFile {
				envFilePath := path.Join(d.composeDir, envFile)
				content, err := ioutil.ReadFile(envFilePath)
				if err == nil {
					logrus.Infof("loading env file: %s", envFile)
					// 解析 env 文件
					lines := strings.Split(string(content), "\n")
					for _, line := range lines {
						line = strings.TrimSpace(line)
						// 跳过空行和注释
						if line == "" || strings.HasPrefix(line, "#") {
							continue
						}
						// 解析 KEY=VALUE 格式
						parts := strings.SplitN(line, "=", 2)
						if len(parts) == 2 {
							key := strings.TrimSpace(parts[0])
							value := strings.TrimSpace(parts[1])
							// 移除值两端的引号
							value = strings.Trim(value, "\"'")
							envs[key] = &types.Env{
								Name:  key,
								Value: value,
							}
							logrus.Debugf("loaded env from file: %s=%s", key, value)
						}
					}
				} else {
					logrus.Warnf("failed to read env file %s: %v", envFile, err)
				}
			}
		}

		service := ServiceInfoFromDC{
			ports:      ports,
			volumes:    volumes,
			envs:       envs,
			memory:     int(sc.MemLimit / 1024 / 1024),
			image:      ParseImageName(proxyDockerIOImage(sc.Image)),
			depends:    sc.Links,
			imageAlias: kev, // Use service name instead of container_name
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
		missingDepends := []string{}
		for i, depend := range service.depends {
			if strings.Contains(depend, ":") {
				service.depends[i] = strings.Split(depend, ":")[0]
			}
			if _, ok := d.services[service.depends[i]]; !ok {
				missingDepends = append(missingDepends, service.depends[i])
			} else {
				existDepends = append(existDepends, service.depends[i])
			}
		}
		// Only add one error per service with all missing dependencies
		if len(missingDepends) > 0 {
			d.errappend(ErrorAndSolve(NegligibleError,
				fmt.Sprintf("服务 %s 依赖的服务不存在：%s", serviceName, strings.Join(missingDepends, ", ")),
				"请检查这些依赖服务是否在 compose 文件中定义"))
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
		logrus.Infof("开始检查服务 %s 的镜像: %s", serviceName, service.image.String())
		d.logger.Debug(fmt.Sprintf("start check service %s ", service.name), map[string]string{"step": "service_check", "status": "running"})
		exist, err := sources.ImageExist(service.image.String(), hubUser, hubPass)
		if err != nil {
			logrus.Errorf("服务 %s 镜像检查失败: %s, 错误: %s", serviceName, service.image.String(), err.Error())
		}
		if !exist {
			logrus.Warnf("服务 %s 镜像不存在: %s", serviceName, service.image.String())
			d.errappend(ErrorAndSolve(NegligibleError, fmt.Sprintf("服务%s镜像%s检测失败", serviceName, service.image.String()), SolveAdvice("modify_compose", fmt.Sprintf("请确认%s服务镜像名称是否正确或镜像仓库访问是否正常", serviceName))))
		} else {
			logrus.Infof("服务 %s 镜像检查成功: %s", serviceName, service.image.String())
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

// logFieldSupportReport logs field support issues to the event logger
func (d *DockerComposeParse) logFieldSupportReport(report *compose.FieldSupportReport) {
	if report == nil || !report.HasIssues() {
		return
	}

	// Only log degraded (limited support) warnings
	degradedIssues := report.GetIssuesByLevel(compose.SupportLevelDegraded)
	if len(degradedIssues) == 0 {
		return
	}

	// Log each degraded issue with simplified message
	for _, issue := range degradedIssues {
		// Simplified user-friendly message format
		msg := d.getSimplifiedWarningMessage(issue)
		d.logger.Info(msg, map[string]string{
			"level": "warning",
		})
	}

	// Log summary
	if len(degradedIssues) > 0 {
		summary := fmt.Sprintf("检测到 %d 个配置项有限支持或将被忽略", len(degradedIssues))
		d.logger.Info(summary, map[string]string{"level": "info"})
	}
}

// addSupportReportToErrors adds field support issues to the error list for API response
func (d *DockerComposeParse) addSupportReportToErrors(report *compose.FieldSupportReport) {
	if report == nil || !report.HasIssues() {
		return
	}

	// Only add degraded (limited support) warnings
	// Ignore unsupported and info level issues
	for _, issue := range report.GetIssuesByLevel(compose.SupportLevelDegraded) {
		// Use simplified warning message
		warningMsg := d.getSimplifiedWarningMessage(issue)
		d.errappend(ErrorAndSolve(NegligibleError, warningMsg, ""))
	}
}

// getSimplifiedWarningMessage generates user-friendly warning messages
func (d *DockerComposeParse) getSimplifiedWarningMessage(issue compose.FieldIssue) string {
	serviceName := issue.Service
	field := issue.Field

	// Simplified messages based on field type
	switch field {
	case "networks":
		return fmt.Sprintf("服务 %s：网络配置将被忽略，平台会自动管理服务间网络连接", serviceName)
	case "depends_on":
		return fmt.Sprintf("服务 %s：依赖关系中的健康检查条件将被忽略，仅保留启动顺序", serviceName)
	case "logging":
		return fmt.Sprintf("服务 %s：日志配置将被忽略，平台会统一收集和管理日志", serviceName)
	case "container_name":
		return fmt.Sprintf("服务 %s：自定义容器名称在多副本时会被自动生成", serviceName)
	case "profiles":
		return fmt.Sprintf("服务 %s：profiles 配置将被忽略，所有服务都会被部署", serviceName)
	default:
		// Fallback to generic message
		return fmt.Sprintf("服务 %s：%s 配置有限支持，可能会被调整", serviceName, field)
	}
}

// downloadAndExtractProject downloads and extracts the project package from S3
func (d *DockerComposeParse) downloadAndExtractProject() error {
	projectPath := fmt.Sprintf("/grdata/package_build/temp/events/%s", d.eventID)
	d.projectPath = projectPath
	logrus.Infof("[DEBUG] 项目路径: %s, event_id: %s", projectPath, d.eventID)

	// Download project files from S3
	err := storage.Default().StorageCli.DownloadDirToDir(projectPath, projectPath)
	if err != nil {
		logrus.Errorf("download project from S3 failed: %v", err)
		return fmt.Errorf("下载项目文件失败: %v", err)
	}
	logrus.Infof("[DEBUG] S3 下载完成")

	// Read directory to find the archive file
	fileList, err := ioutil.ReadDir(projectPath)
	if err != nil || len(fileList) == 0 {
		logrus.Errorf("[DEBUG] 读取目录失败或目录为空: %v", err)
		return fmt.Errorf("项目目录为空或无法读取")
	}

	// Log all files in directory
	logrus.Infof("[DEBUG] 下载后目录内容 (%d 个文件):", len(fileList))
	for i, f := range fileList {
		logrus.Infof("[DEBUG]   [%d] %s (size: %d, isDir: %v)", i, f.Name(), f.Size(), f.IsDir())
	}

	// Get the first file (should be the archive)
	filePath := path.Join(projectPath, fileList[0].Name())
	ext := path.Ext(fileList[0].Name())
	logrus.Infof("[DEBUG] 准备解压文件: %s, 扩展名: %s", filePath, ext)

	// Extract based on file extension
	switch ext {
	case ".tar":
		if err := util.UnTar(filePath, projectPath, false); err != nil {
			logrus.Errorf("untar project file failed: %v", err)
			return fmt.Errorf("解压 tar 文件失败: %v", err)
		}
	case ".tgz", ".gz":
		if err := util.UnTar(filePath, projectPath, true); err != nil {
			logrus.Errorf("untar project file failed: %v", err)
			return fmt.Errorf("解压 tgz 文件失败: %v", err)
		}
	case ".zip":
		if err := util.Unzip(filePath, projectPath); err != nil {
			logrus.Errorf("unzip project file failed: %v", err)
			return fmt.Errorf("解压 zip 文件失败: %v", err)
		}
	default:
		return fmt.Errorf("不支持的文件格式: %s", ext)
	}

	// Log files after extraction
	fileListAfter, err := ioutil.ReadDir(projectPath)
	if err == nil {
		logrus.Infof("[DEBUG] 解压后目录内容 (%d 个文件):", len(fileListAfter))
		for i, f := range fileListAfter {
			logrus.Infof("[DEBUG]   [%d] %s (size: %d, isDir: %v)", i, f.Name(), f.Size(), f.IsDir())
		}
	}

	logrus.Infof("project extracted successfully to: %s", projectPath)
	return nil
}

// loadComposeFile loads the compose file content from the extracted project
func (d *DockerComposeParse) loadComposeFile() error {
	composeFilePath := path.Join(d.projectPath, d.composeFilePath)
	logrus.Infof("[DEBUG] 尝试读取 compose 文件: %s", composeFilePath)
	logrus.Infof("[DEBUG] 项目路径: %s, compose 文件路径参数: %s", d.projectPath, d.composeFilePath)

	var foundPath string

	// Try to read the specified compose file
	content, err := ioutil.ReadFile(composeFilePath)
	if err == nil {
		foundPath = composeFilePath
		logrus.Infof("[DEBUG] 成功读取指定的 compose 文件")
	} else {
		logrus.Warnf("[DEBUG] 读取指定文件失败: %v", err)

		commonNames := []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"}

		// Try root directory
		for _, name := range commonNames {
			tryPath := path.Join(d.projectPath, name)
			content, err = ioutil.ReadFile(tryPath)
			if err == nil {
				foundPath = tryPath
				logrus.Infof("found compose file at: %s", tryPath)
				break
			}
		}

		// Try common subdirectories
		if foundPath == "" {
			commonSubdirs := []string{"docker", "compose", "deployment", "deploy"}
			for _, subdir := range commonSubdirs {
				for _, name := range commonNames {
					tryPath := path.Join(d.projectPath, subdir, name)
					content, err = ioutil.ReadFile(tryPath)
					if err == nil {
						foundPath = tryPath
						logrus.Infof("found compose file at: %s", tryPath)
						break
					}
				}
				if foundPath != "" {
					break
				}
			}
		}

		if foundPath == "" {
			return fmt.Errorf("未找到 compose 文件: %s", d.composeFilePath)
		}
	}

	// 记录 compose 文件所在目录，volume 相对路径基于此目录
	d.composeDir = path.Dir(foundPath)
	d.source = string(content)
	logrus.Infof("compose file loaded: %s, composeDir: %s, size: %d bytes", foundPath, d.composeDir, len(content))
	return nil
}

// proxyDockerIOImage 将 docker.io 镜像替换为代理地址
// 例如: nginx:latest -> docker.1ms.run/library/nginx:latest
//       langgenius/dify-api:1.12.1 -> docker.1ms.run/langgenius/dify-api:1.12.1
//       docker.io/library/nginx:latest -> docker.1ms.run/library/nginx:latest
func proxyDockerIOImage(imageName string) string {
	// 已经是代理地址，不处理
	if strings.HasPrefix(imageName, "docker.1ms.run/") {
		return imageName
	}

	// 去掉 docker.io/ 前缀（如果有）
	name := imageName
	if strings.HasPrefix(name, "docker.io/") {
		name = strings.TrimPrefix(name, "docker.io/")
	}

	// 判断是否是 docker.io 镜像：
	// 没有 / 或第一段不含 .（非域名）的都是 docker.io 镜像
	// 排除：包含 . 的域名（如 ghcr.io/xxx, registry.example.com/xxx）
	parts := strings.SplitN(name, "/", 2)
	if len(parts) == 1 || !strings.Contains(parts[0], ".") {
		// 对于官方镜像（没有 /），需要加 library/ 前缀
		if !strings.Contains(name, "/") {
			return "docker.1ms.run/library/" + name
		}
		return "docker.1ms.run/" + name
	}

	// 非 docker.io 镜像，不处理
	return imageName
}

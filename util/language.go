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

package util

import "os"

var translationMetadata = map[string]string{
	"write console level metadata success":           "写控制台级应用元数据成功",
	"write region level metadata success":            "写数据中心级应用元数据成功",
	"Asynchronous tasks are sent successfully":       "异步任务发送成功",
	"create ftp client error":                        "创建FTP客户端失败",
	"push slug file to local dir error":              "上传应用代码包到本地目录失败",
	"push slug file to sftp server error":            "上传应用代码包到服务端失败",
	"down slug file from sftp server error":          "从服务端下载文件失败",
	"save image to local dir error":                  "保存镜像到本地目录失败",
	"save image to hub error":                        "保存镜像到仓库失败",
	"Please try again or contact customer service":   "后端服务开小差，请重试或联系客服",
	"unzip metadata file error":                      "解压数据失败",
	"start service error":                            "启动服务失败,请检查集群服务信息或查看日志",
	"start service timeout":                          "启动服务超时,建议观察服务实例运行状态",
	"stop service error":                             "停止服务失败,建议观察服务实例运行状态",
	"stop service timeout":                           "停止服务超时,建议观察服务实例运行状态",
	"(restart)stop service error":                    "停止服务失败,建议观察服务实例运行状态,待其停止后手动启动",
	"(restart)Application model init create failure": "初始化应用元数据模型失败,请检查集群运行状态或查看日志",
	"horizontal scaling service error":               "水平扩容失败,请检查集群运行状态或查看日志",
	"horizontal scaling service timeout":             "水平扩容超时,建议观察服务实例运行状态",
	"upgrade service error":                          "升级服务失败,请检查服务信息或查看日志",
	"upgrade service timeout":                        "升级服务超时, 建议观察服务实例运行状态",
	"Check for log location code errors":             "建议查看日志定位代码错误",
	"Check for log location imgae source errors":     "建议查看日志定位镜像源错误",
	"create share image task error":                  "分享任务失败，请检查服务信息或查看日志",
	"get rbd-repo ip failure":                        "获取依赖仓库IP地址失败，请检查rbd-repo组件信息",
	"reparse code lange error":                       "重新解析代码语言错误",
	"get code commit info error":                     "读取代码版本信息失败",
	"pull git code error":                            "拉取代码失败",
	"git project warehouse address format error":     "Git项目仓库地址格式错误",
	"prepare build code error":                       "准备源码构建失败",
	"Checkout svn code failed, please make sure the code can be downloaded properly":    "检查svn代码失败，请确保代码可以被正常下载",
	"Pull image failed, please check if the image is accessible":                        "拉取镜像失败，请排查镜像是否可以访问",
	"Pull source code failed, please check if the repository is accessible":             "拉取源码失败，请排查仓库是否可以访问",
	"Build timeout, exceeded maximum build time of 60 minutes, please check build logs": "编译超时，超过最大编译时间60分钟，请查看构建日志",
	"Build failed, please check build logs":                                             "编译失败，请查看构建日志",
	"Pull runner image failed, please check if the image is accessible":                 "拉取运行时镜像失败，请排查镜像是否可以访问",
	"Build image failed, please check build logs":                                       "构建镜像失败，请查看构建日志",

	// Image build related errors
	"Tag image failed":                                      "修改镜像标签失败",
	"Push image to registry failed":                         "推送镜像至镜像仓库失败",
	"Update version info failed":                            "更新应用版本信息失败",
	"Update application service version information failed": "更新应用服务版本信息失败",

	// Git related errors
	"Pull code error, authentication required":         "拉取代码发生错误，代码源需要授权访问",
	"Pull code error, authorization failed":            "拉取代码发生错误，代码源鉴权失败",
	"Pull code error, repository not found":            "拉取代码发生错误，仓库不存在",
	"Pull code error, empty remote repository":         "拉取代码发生错误，远程仓库为空",
	"Code branch does not exist":                       "代码分支不存在",
	"Remote repository requires SSH key configuration": "远程代码库需要配置SSH Key",
	"Pull code timeout":                                "拉取代码超时",
	"Create SSH public keys error":                     "创建SSH公钥错误",
	"Pull code error, failed to delete code directory": "拉取代码发生错误，删除代码目录失败",
	"Clear code directory failed":                      "清空代码目录失败",

	// Dockerfile build errors
	"Parse dockerfile error":            "解析Dockerfile失败",
	"Compiling the source code failure": "编译源代码失败",
	"Create build job failed":           "创建构建任务失败",
	"Get tenant info failed":            "获取租户信息失败",
	"Create image pull secret failed":   "创建镜像拉取凭证失败",

	// Code build errors
	"Check that the build result failure":            "检查构建结果失败",
	"Source build failure and result slug size is 0": "源码构建失败，构建结果大小为0",
	"Build runner image failure":                     "构建运行时镜像失败",
	"Handle nodejs code error":                       "处理NodeJS代码错误",
	"Pull builder image failed":                      "拉取构建器镜像失败",

	// Market slug build errors
	"Create FTP client failed":                     "创建FTP客户端失败",
	"Download slug package from remote FTP failed": "从远程FTP下载源码包失败",
	"Get slug package from local storage failed":   "从本地存储获取源码包失败",
	"Change slug package permission failed":        "修改源码包权限失败",

	// Deployment related errors - resource issues
	"Deployment failed: namespace resource quota exceeded": "部署失败：命名空间资源配额已超限，请联系管理员增加CPU/内存配额",
	"Deployment failed: insufficient CPU resources":        "部署失败：节点CPU资源不足，请降低CPU请求值或联系管理员增加节点",
	"Deployment failed: insufficient memory resources":     "部署失败：节点内存资源不足，请降低内存请求值或联系管理员增加节点",
	"Deployment failed: insufficient storage resources":    "部署失败：存储资源不足，请联系管理员检查存储配置",
	"Deployment failed: no nodes available for scheduling": "部署失败：没有可用节点进行调度，请联系管理员检查集群节点状态",

	// Deployment related errors - storage issues
	"Deployment failed: persistent volume claim is pending": "部署失败：存储卷申请处于等待状态，请检查存储卷配置或联系管理员",

	// Deployment related errors - image issues
	"Deployment failed: invalid image name":               "部署失败：镜像名称格式无效，请检查镜像地址格式",
	"Deployment failed: image pull failed":                "部署失败：拉取镜像失败，请检查镜像是否存在或镜像仓库是否可访问",
	"Deployment failed: image pull authentication failed": "部署失败：拉取镜像认证失败，请检查镜像仓库的用户名和密码",
	"Deployment failed: image not found":                  "部署失败：镜像不存在，请检查镜像名称和标签是否正确",

	// Deployment related errors - container issues
	"Deployment failed: container configuration error":            "部署失败：容器配置错误，请检查环境变量、存储卷挂载等配置",
	"Deployment failed: container creation failed":                "部署失败：容器创建失败，请查看实例日志定位问题",
	"Deployment failed: container startup failed":                 "部署失败：容器启动失败，请查看实例日志定位启动脚本或命令问题",
	"Deployment failed: container is being terminated repeatedly": "部署失败：容器反复崩溃重启，请查看实例日志定位应用代码问题",
	"Deployment failed: container out of memory killed":           "部署失败：容器因内存超限被终止，请增加内存限制或优化应用内存使用",

	// Deployment related errors - health check issues
	"Deployment failed: readiness probe failed": "部署失败：就绪检查失败，应用未能正常响应健康检查，请查看实例日志或调整健康检查配置",
	"Deployment failed: liveness probe failed":  "部署失败：存活检查失败，应用未能正常响应健康检查，请查看实例日志或调整健康检查配置",
	"Deployment failed: startup probe failed":   "部署失败：启动检查失败，应用启动超时，请查看实例日志或增加启动超时时间",

	// Deployment related errors - permission and security
	"Deployment failed: insufficient permissions":  "部署失败：权限不足，请联系管理员检查服务账号权限",
	"Deployment failed: security policy violation": "部署失败：违反安全策略，请联系管理员检查Pod安全策略配置",

	// Deployment related errors - general
	"Create workload failed":              "创建工作负载失败",
	"Deployment timeout: Pod not created": "部署超时：实例未创建，请检查资源配额、节点状态和存储配置",
	"Service deploy timeout":              "服务部署超时",

	// Pod status descriptions (user-friendly)
	"Pod is initializing":   "实例正在初始化中",
	"Pod scheduling failed": "实例调度失败",
}

// Translation Translation English to Chinese
func Translation(english string) string {
	if chinese, ok := translationMetadata[english]; ok {
		if os.Getenv("RAINBOND_LANG") == "en" {
			return english
		}
		return chinese
	}
	return english
}

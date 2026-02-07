// RAINBOND, Application Management Platform
// Copyright (C) 2014-2019 Goodrain Co., Ltd.

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

package sources

import (
	"fmt"
	"strings"
	"time"

	"github.com/goodrain/rainbond/builder/sources/registry"

	"github.com/docker/distribution/reference"
	"github.com/sirupsen/logrus"
)

//GetTagFromNamedRef get image tag by name
func GetTagFromNamedRef(ref reference.Named) string {
	if digested, ok := ref.(reference.Digested); ok {
		return digested.Digest().String()
	}
	ref = reference.TagNameOnly(ref)
	if tagged, ok := ref.(reference.Tagged); ok {
		return tagged.Tag()
	}
	return ""
}

//ImageExist check image exist
func ImageExist(imageName, user, password string) (bool, error) {
	startTime := time.Now()
	logrus.Infof("开始检查镜像是否存在: %s", imageName)

	// 首先尝试原始镜像名称
	exist, err := checkImageExist(imageName, user, password)
	if exist {
		logrus.Infof("镜像 %s 存在，总耗时: %v", imageName, time.Since(startTime))
		return true, nil
	}

	// 如果失败，检查是否是 docker.io 镜像，尝试使用代理
	ref, parseErr := reference.ParseAnyReference(imageName)
	if parseErr == nil {
		name, nameErr := reference.ParseNamed(ref.String())
		if nameErr == nil {
			domain := reference.Domain(name)
			// 如果是 docker.io 或没有域名前缀（默认是 docker.io），使用镜像代理
			if domain == "docker.io" || !strings.Contains(imageName, "/") || strings.Count(imageName, "/") == 1 {
				proxyImageName := "docker.1ms.run/" + imageName
				// 移除可能的 docker.io 前缀避免重复
				proxyImageName = strings.Replace(proxyImageName, "docker.1ms.run/docker.io/", "docker.1ms.run/", 1)

				logrus.Infof("原始镜像检查失败，尝试使用镜像代理: %s", proxyImageName)
				exist, proxyErr := checkImageExist(proxyImageName, user, password)
				if exist {
					logrus.Infof("通过镜像代理找到镜像: %s，总耗时: %v", proxyImageName, time.Since(startTime))
					return true, nil
				}
				logrus.Warnf("镜像代理也失败: %v", proxyErr)
			}
		}
	}

	logrus.Errorf("镜像 %s 检查失败，总耗时: %v, 错误: %v", imageName, time.Since(startTime), err)
	return false, err
}

// checkImageExist 实际执行镜像检查的内部函数
func checkImageExist(imageName, user, password string) (bool, error) {
	ref, err := reference.ParseAnyReference(imageName)
	if err != nil {
		logrus.Errorf("镜像名称解析失败: %s, 错误: %s", imageName, err.Error())
		return false, err
	}
	name, err := reference.ParseNamed(ref.String())
	if err != nil {
		logrus.Errorf("镜像名称格式化失败: %s, 错误: %s", imageName, err.Error())
		return false, err
	}
	domain := reference.Domain(name)
	if domain == "docker.io" {
		domain = "registry-1.docker.io"
	}
	logrus.Infof("镜像仓库地址: %s, 镜像路径: %s, 标签: %s", domain, reference.Path(name), GetTagFromNamedRef(name))

	retry := 2
	var rerr error
	for retry > 0 {
		retry--
		attemptStart := time.Now()
		logrus.Infof("尝试连接镜像仓库 (剩余重试次数: %d): %s", retry+1, domain)

		reg, err := registry.New(domain, user, password)
		if err != nil {
			logrus.Debugf("创建安全连接失败: %s, 尝试不安全连接", err.Error())
			reg, err = registry.NewInsecure(domain, user, password)
			if err != nil {
				logrus.Debugf("创建不安全 HTTPS 连接失败: %s, 尝试 HTTP 连接", err.Error())
				reg, err = registry.NewInsecure("http://"+domain, user, password)
				if err != nil {
					logrus.Errorf("所有连接方式均失败，镜像仓库: %s, 错误: %s, 耗时: %v", domain, err.Error(), time.Since(attemptStart))
					rerr = err
					continue
				}
			}
		}
		logrus.Infof("成功连接到镜像仓库: %s, 耗时: %v", domain, time.Since(attemptStart))

		checkStart := time.Now()
		tag := GetTagFromNamedRef(name)
		logrus.Infof("开始检查镜像清单: %s:%s", reference.Path(name), tag)
		if err := reg.CheckManifest(reference.Path(name), tag); err != nil {
			logrus.Errorf("镜像清单检查失败: %s:%s, 错误: %v, 耗时: %v", reference.Path(name), tag, err, time.Since(checkStart))
			rerr = fmt.Errorf("[ImageExist] check manifest v2: %v", err)
			continue
		}
		logrus.Infof("镜像清单检查成功: %s:%s, 耗时: %v", reference.Path(name), tag, time.Since(checkStart))
		return true, nil
	}
	return false, rerr
}

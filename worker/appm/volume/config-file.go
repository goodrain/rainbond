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

// 本文件定义了与 Rainbond 平台中配置文件卷相关的结构体和方法，
// 主要用于在应用服务中创建和管理配置文件类型的存储卷。

// 文件内容包括以下几个主要部分：
// 1. `ConfigFileVolume` 结构体：这是一个与配置文件卷相关的结构体，继承自 `Base`，并包含了环境变量（`envs`）和与环境变量相关的秘密对象（`envVarSecrets`）。
//    该结构体的主要作用是通过结合环境变量和配置文件内容，创建适用于应用服务的 ConfigMap 对象，并将其挂载到相应的卷路径中。

// 2. `CreateVolume` 方法：该方法用于根据给定的定义对象（`Define`）创建配置文件卷。它首先从数据库中获取配置文件内容，
//    然后将其与环境变量进行结合，通过替换变量的方式生成最终的配置文件内容。接着，创建一个 Kubernetes 的 ConfigMap 对象，并将配置文件内容写入其中，
//    最后将该 ConfigMap 对象挂载到定义对象的卷路径中。

// 3. `CreateDependVolume` 方法：该方法与 `CreateVolume` 方法类似，主要用于创建依赖服务的配置文件卷。它从依赖服务的数据库记录中获取相关的卷信息和配置文件内容，
//    然后生成并挂载 ConfigMap 对象到指定路径。

// 通过这些结构体和方法，Rainbond 平台可以在应用服务启动时动态生成并配置相应的配置文件卷，
// 使得服务能够根据环境变量和配置文件的内容自动进行初始化配置，从而增强了平台的灵活性和可配置性。

package volume

import (
	"fmt"
	"path"

	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigFileVolume config file volume struct
type ConfigFileVolume struct {
	Base
	envs          []corev1.EnvVar
	envVarSecrets []*corev1.Secret
}

// CreateVolume config file volume create volume
func (v *ConfigFileVolume) CreateVolume(define *Define) error {
	// environment variables
	configs := make(map[string]string)
	for _, sec := range v.envVarSecrets {
		for k, v := range sec.Data {
			// The priority of component environment variable is higher than the one of the application.
			if val := configs[k]; val == string(v) {
				continue
			}
			configs[k] = string(v)
		}
	}
	// component env priority over the app configuration group
	for _, env := range v.envs {
		configs[env.Name] = env.Value
	}
	cf, err := v.dbmanager.TenantServiceConfigFileDao().GetByVolumeName(v.as.ServiceID, v.svm.VolumeName)
	if err != nil {
		logrus.Errorf("error getting config file by volume name(%s): %v", v.svm.VolumeName, err)
		return fmt.Errorf("error getting config file by volume name(%s): %v", v.svm.VolumeName, err)
	}
	cmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      util.NewUUID(),
			Namespace: v.as.GetNamespace(),
			Labels:    v.as.GetCommonLabels(),
		},
		Data: make(map[string]string),
	}
	cmap.Data[path.Base(v.svm.VolumePath)] = util.ParseVariable(cf.FileContent, configs)
	v.as.SetConfigMap(cmap)
	define.SetVolumeCMap(cmap, path.Base(v.svm.VolumePath), v.svm.VolumePath, false, v.svm.Mode)
	return nil
}

// CreateDependVolume config file volume create depend volume
func (v *ConfigFileVolume) CreateDependVolume(define *Define) error {
	configs := make(map[string]string)
	for _, env := range v.envs {
		configs[env.Name] = env.Value
	}
	depVol, err := v.dbmanager.TenantServiceVolumeDao().GetVolumeByServiceIDAndName(v.smr.DependServiceID, v.smr.VolumeName)
	if err != nil {
		return fmt.Errorf("error getting TenantServiceVolume according to serviceID(%s) and volumeName(%s): %v",
			v.smr.DependServiceID, v.smr.VolumeName, err)
	}
	cf, err := v.dbmanager.TenantServiceConfigFileDao().GetByVolumeName(v.smr.DependServiceID, v.smr.VolumeName)
	if err != nil {
		return fmt.Errorf("error getting TenantServiceConfigFile according to volumeName(%s): %v", v.smr.VolumeName, err)
	}

	cmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      util.NewUUID(),
			Namespace: v.as.GetNamespace(),
			Labels:    v.as.GetCommonLabels(),
		},
		Data: make(map[string]string),
	}
	cmap.Data[path.Base(v.smr.VolumePath)] = util.ParseVariable(cf.FileContent, configs)
	v.as.SetConfigMap(cmap)

	define.SetVolumeCMap(cmap, path.Base(v.smr.VolumePath), v.smr.VolumePath, false, depVol.Mode)
	return nil
}

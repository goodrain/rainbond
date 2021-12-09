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

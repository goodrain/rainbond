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

package conversion

import (
	"fmt"
	"strings"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	typesv1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/jinzhu/gorm"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

//TenantServicePlugin conv service all plugin
func TenantServicePlugin(as *typesv1.AppService, dbmanager db.Manager) error {
	initContainers, pluginContainers, err := createPluginsContainer(as, dbmanager)
	if err != nil {
		return err
	}
	podtemplate := as.GetPodTemplate()
	podtemplate.Spec.Containers = append(podtemplate.Spec.Containers, pluginContainers...)
	podtemplate.Spec.InitContainers = initContainers
	return nil
}

func createPluginsContainer(as *typesv1.AppService, dbmanager db.Manager) ([]v1.Container, []v1.Container, error) {
	var containers []v1.Container
	var initContainers []v1.Container
	appPlugins, err := dbmanager.TenantServicePluginRelationDao().GetALLRelationByServiceID(as.ServiceID)
	if err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
		return nil, nil, fmt.Errorf("find plugins error. %v", err.Error())
	}
	if len(appPlugins) == 0 && !as.NeedProxy {
		return nil, nil, nil
	}
	netPlugin := false
	for _, pluginR := range appPlugins {
		//if plugin not enable,ignore it
		if pluginR.Switch == false {
			continue
		}
		versionInfo, err := dbmanager.TenantPluginBuildVersionDao().GetLastBuildVersionByVersionID(pluginR.PluginID, pluginR.VersionID)
		if err != nil {
			return nil, nil, err
		}
		envs, err := createPluginEnvs(pluginR.PluginID, as.TenantID, as.ServiceAlias, as.GetPodTemplate().Spec.Containers[0].Env, pluginR.VersionID, as.ServiceID, dbmanager)
		if err != nil {
			return nil, nil, err
		}
		args, err := createPluginArgs(versionInfo.ContainerCMD)
		if err != nil {
			return nil, nil, err
		}
		pc := v1.Container{
			Name:                   "plugin-" + pluginR.PluginID,
			Image:                  versionInfo.BuildLocalImage,
			Env:                    *envs,
			Resources:              createPluginResources(pluginR.ContainerMemory, pluginR.ContainerCPU),
			TerminationMessagePath: "",
			Args:                   args,
			VolumeMounts:           as.GetPodTemplate().Spec.Containers[0].VolumeMounts,
		}
		pluginModel, err := getPluginModel(pluginR.PluginID, as.TenantID, dbmanager)
		if err != nil {
			return nil, nil, err
		}
		if pluginModel == model.InitPlugin {
			initContainers = append(initContainers, pc)
			continue
		}
		if pluginModel == model.DownNetPlugin {
			netPlugin = true
		}
		containers = append(containers, pc)
	}
	//if need proxy but not install net plugin
	if as.NeedProxy && !netPlugin {
		c2 := v1.Container{
			Name: "adapter-" + as.ServiceID[len(as.ServiceID)-20:],
			VolumeMounts: []v1.VolumeMount{v1.VolumeMount{
				MountPath: "/etc/kubernetes",
				Name:      "kube-config",
				ReadOnly:  true,
			}},
			TerminationMessagePath: "",
			Image:                  "goodrain.me/adapter",
			Resources:              createAdapterResources(50, 20),
		}
		containers = append(containers, c2)
	}
	return initContainers, containers, nil
}

func getPluginModel(pluginID, tenantID string, dbmanager db.Manager) (string, error) {
	plugin, err := dbmanager.TenantPluginDao().GetPluginByID(pluginID, tenantID)
	if err != nil {
		return "", err
	}
	return plugin.PluginModel, nil
}

func createPluginArgs(cmd string) ([]string, error) {
	if cmd == "" {
		return nil, nil
	}
	return strings.Split(cmd, " "), nil
}

//container envs
func createPluginEnvs(pluginID, tenantID, serviceAlias string, mainEnvs []v1.EnvVar, versionID, serviceID string, dbmanager db.Manager) (*[]v1.EnvVar, error) {
	versionEnvs, err := dbmanager.TenantPluginVersionENVDao().GetVersionEnvByServiceID(serviceID, pluginID)
	if err != nil {
		return nil, err
	}
	var envs []v1.EnvVar
	//first set main service env
	for _, e := range mainEnvs {
		envs = append(envs, e)
	}
	for _, e := range versionEnvs {
		envs = append(envs, v1.EnvVar{Name: e.EnvName, Value: e.EnvValue})
	}
	discoverURL := fmt.Sprintf(
		"/v1/resources/%s/%s/%s",
		tenantID,
		serviceAlias,
		pluginID)
	envs = append(envs, v1.EnvVar{Name: "DISCOVER_URL", Value: discoverURL})
	envs = append(envs, v1.EnvVar{Name: "PLUGIN_ID", Value: pluginID})
	return &envs, nil
}

func pluginWeight(pluginModel string) int {
	switch pluginModel {
	case model.UpNetPlugin:
		return 9
	case model.DownNetPlugin:
		return 8
	case model.GeneralPlugin:
		return 1
	default:
		return 0
	}
}
func createPluginResources(memory int, cpu int) v1.ResourceRequirements {
	limits := v1.ResourceList{}
	// limits[v1.ResourceCPU] = *resource.NewMilliQuantity(
	// 	int64(cpu*3),
	// 	resource.DecimalSI)
	limits[v1.ResourceMemory] = *resource.NewQuantity(
		int64(memory*1024*1024),
		resource.BinarySI)
	request := v1.ResourceList{}
	// request[v1.ResourceCPU] = *resource.NewMilliQuantity(
	// 	int64(cpu*2),
	// 	resource.DecimalSI)
	request[v1.ResourceMemory] = *resource.NewQuantity(
		int64(memory*1024*1024),
		resource.BinarySI)
	return v1.ResourceRequirements{
		Limits:   limits,
		Requests: request,
	}
}

func createAdapterResources(memory int, cpu int) v1.ResourceRequirements {
	limits := v1.ResourceList{}
	limits[v1.ResourceCPU] = *resource.NewMilliQuantity(
		int64(cpu*3),
		resource.DecimalSI)
	//limits[v1.ResourceMemory] = *resource.NewQuantity(
	//	int64(memory*1024*1024),
	//	resource.BinarySI)
	request := v1.ResourceList{}
	request[v1.ResourceCPU] = *resource.NewMilliQuantity(
		int64(cpu*2),
		resource.DecimalSI)
	//request[v1.ResourceMemory] = *resource.NewQuantity(
	//	int64(memory*1024*1024),
	//	resource.BinarySI)
	return v1.ResourceRequirements{
		Limits:   limits,
		Requests: request,
	}
}

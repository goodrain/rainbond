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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"

	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	typesv1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/jinzhu/gorm"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//TenantServicePlugin conv service all plugin
func TenantServicePlugin(as *typesv1.AppService, dbmanager db.Manager) error {
	initContainers, pluginContainers, err := conversionServicePlugin(as, dbmanager)
	if err != nil {
		return err
	}
	podtemplate := as.GetPodTemplate()
	if podtemplate != nil {
		podtemplate.Spec.Containers = append(podtemplate.Spec.Containers, pluginContainers...)
		podtemplate.Spec.InitContainers = initContainers
		return nil
	}
	return fmt.Errorf("pod templete is nil before define plugin")
}

func conversionServicePlugin(as *typesv1.AppService, dbmanager db.Manager) ([]v1.Container, []v1.Container, error) {
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
	var meshPluginID string
	var mainContainer v1.Container
	if as.GetPodTemplate() != nil && len(as.GetPodTemplate().Spec.Containers) > 0 {
		mainContainer = as.GetPodTemplate().Spec.Containers[0]
	}
	for _, pluginR := range appPlugins {
		//if plugin not enable,ignore it
		if pluginR.Switch == false {
			continue
		}
		//apply plugin dynamic config
		ApplyPluginConfig(as, pluginR, dbmanager)
		versionInfo, err := dbmanager.TenantPluginBuildVersionDao().GetLastBuildVersionByVersionID(pluginR.PluginID, pluginR.VersionID)
		if err != nil {
			return nil, nil, fmt.Errorf("do not found available plugin versions")
		}
		podTmpl := as.GetPodTemplate()
		if podTmpl == nil {
			logrus.Warnf("Can't not get pod for plugin(plugin_id=%s)", pluginR.PluginID)
			continue
		}
		envs, err := createPluginEnvs(pluginR.PluginID, as.TenantID, as.ServiceAlias, mainContainer.Env, pluginR.VersionID, as.ServiceID, dbmanager)
		if err != nil {
			return nil, nil, err
		}
		args, err := createPluginArgs(versionInfo.ContainerCMD, *envs)
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
			VolumeMounts:           mainContainer.VolumeMounts,
		}
		pluginModel, err := getPluginModel(pluginR.PluginID, as.TenantID, dbmanager)
		if err != nil {
			return nil, nil, fmt.Errorf("get plugin model info failure %s", err.Error())
		}
		if pluginModel == model.DownNetPlugin {
			netPlugin = true
			meshPluginID = pluginR.PluginID
		}
		if pluginModel == model.InitPlugin {
			initContainers = append(initContainers, pc)
		} else {
			containers = append(containers, pc)
		}
	}
	var udpDep bool
	//if need proxy but not install net plugin
	if as.NeedProxy && !netPlugin {
		depUDPPort, _ := dbmanager.TenantServicesPortDao().GetDepUDPPort(as.ServiceID)
		if len(depUDPPort) > 0 {
			c2 := createUDPDefaultPluginContainer(as.ServiceID, mainContainer.Env)
			containers = append(containers, c2)
			udpDep = true
		} else {
			pluginID, err := applyDefaultMeshPluginConfig(as, dbmanager)
			if err != nil {
				logrus.Errorf("apply default mesh plugin config failure %s", err.Error())
			}
			c2 := createTCPDefaultPluginContainer(as.ServiceID, pluginID, mainContainer.Env)
			containers = append(containers, c2)
			meshPluginID = pluginID
		}
	}
	if as.NeedProxy && !udpDep && strings.ToLower(as.ExtensionSet["startup_sequence"]) == "true" {
		initContainers = append(initContainers, createProbeMeshInitContainer(as.ServiceID, meshPluginID, as.ServiceAlias, mainContainer.Env))
	}
	return initContainers, containers, nil
}

func createUDPDefaultPluginContainer(serviceID string, envs []v1.EnvVar) v1.Container {
	return v1.Container{
		Name: "default-udpmesh-" + serviceID[len(serviceID)-20:],
		VolumeMounts: []v1.VolumeMount{v1.VolumeMount{
			MountPath: "/etc/kubernetes",
			Name:      "kube-config",
			ReadOnly:  true,
		}},
		Env:                    envs,
		TerminationMessagePath: "",
		Image:                  "goodrain.me/adapter",
		Resources:              createAdapterResources(128, 500),
	}
}

func getTCPMeshImageName() string {
	if d := os.Getenv("TCPMESH_DEFAULT_IMAGE_NAME"); d != "" {
		return d
	}
	return "goodrain.me/mesh_plugin"
}
func getProbeMeshImageName() string {
	if d := os.Getenv("PROBE_MESH_IMAGE_NAME"); d != "" {
		return d
	}
	return "goodrain.me/rbd-init-probe"
}

func createTCPDefaultPluginContainer(serviceID, pluginID string, envs []v1.EnvVar) v1.Container {
	envs = append(envs, v1.EnvVar{Name: "PLUGIN_ID", Value: pluginID})
	dockerBridgeIP, xdsHostPort := getXDSHostIPAndPort()
	envs = append(envs, v1.EnvVar{Name: "XDS_HOST_IP", Value: dockerBridgeIP})
	envs = append(envs, v1.EnvVar{Name: "XDS_HOST_PORT", Value: xdsHostPort})
	return v1.Container{
		Name:      "default-tcpmesh-" + serviceID[len(serviceID)-20:],
		Env:       envs,
		Image:     getTCPMeshImageName(),
		Resources: createAdapterResources(128, 500),
	}
}

func createProbeMeshInitContainer(serviceID, pluginID, serviceAlias string, envs []v1.EnvVar) v1.Container {
	envs = append(envs, v1.EnvVar{Name: "PLUGIN_ID", Value: pluginID})
	dockerBridgeIP, xdsHostPort := getXDSHostIPAndPort()
	envs = append(envs, v1.EnvVar{Name: "XDS_HOST_IP", Value: dockerBridgeIP})
	envs = append(envs, v1.EnvVar{Name: "XDS_HOST_PORT", Value: xdsHostPort})
	return v1.Container{
		Name:      "probe-mesh-" + serviceID[len(serviceID)-20:],
		Env:       envs,
		Image:     getProbeMeshImageName(),
		Resources: createAdapterResources(128, 500),
	}
}

//ApplyPluginConfig applyPluginConfig
func ApplyPluginConfig(as *typesv1.AppService, servicePluginRelation *model.TenantServicePluginRelation, dbmanager db.Manager) {
	config, err := dbmanager.TenantPluginVersionConfigDao().GetPluginConfig(servicePluginRelation.ServiceID, servicePluginRelation.PluginID)
	if err != nil && err != gorm.ErrRecordNotFound {
		logrus.Errorf("get service plugin config from db failure %s", err.Error())
	}
	if config != nil {
		cm := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("config-%s-%s", config.ServiceID, config.PluginID),
				Labels: as.GetCommonLabels(map[string]string{
					"plugin_id":     servicePluginRelation.PluginID,
					"service_alias": as.ServiceAlias,
				}),
			},
			Data: map[string]string{
				"plugin-config": config.ConfigStr,
				"plugin-model":  servicePluginRelation.PluginModel,
			},
		}
		as.SetConfigMap(cm)
	}
}

//applyDefaultMeshPluginConfig applyDefaultMeshPluginConfig
func applyDefaultMeshPluginConfig(as *typesv1.AppService, dbmanager db.Manager) (string, error) {
	var baseServices []*api_model.BaseService
	deps, err := dbmanager.TenantServiceRelationDao().GetTenantServiceRelations(as.ServiceID)
	if err != nil {
		logrus.Errorf("get service depend service info failure %s", err.Error())
	}
	for _, dep := range deps {
		ports, err := dbmanager.TenantServicesPortDao().GetPortsByServiceID(dep.DependServiceID)
		if err != nil {
			logrus.Errorf("get service depend service port info failure %s", err.Error())
		}
		depService, err := dbmanager.TenantServiceDao().GetServiceByID(dep.DependServiceID)
		if err != nil {
			logrus.Errorf("get service depend service info failure %s", err.Error())
		}
		for _, port := range ports {
			depService := &api_model.BaseService{
				ServiceAlias:       as.ServiceAlias,
				ServiceID:          as.ServiceID,
				DependServiceAlias: depService.ServiceAlias,
				DependServiceID:    depService.ServiceID,
				Port:               port.ContainerPort,
				Protocol:           "tcp",
			}
			baseServices = append(baseServices, depService)
		}
	}
	var res = &api_model.ResourceSpec{
		BaseServices: baseServices,
	}
	resJSON, err := json.Marshal(res)
	if err != nil {
		return "", err
	}
	pluginID := "tcpmesh" + util.NewUUID()
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("config-%s-%s", as.ServiceID, pluginID),
			Labels: as.GetCommonLabels(map[string]string{
				"plugin_id":     pluginID,
				"service_alias": as.ServiceAlias,
			}),
		},
		Data: map[string]string{
			"plugin-config": string(resJSON),
			"plugin-model":  model.DownNetPlugin,
		},
	}
	as.SetConfigMap(cm)
	return pluginID, nil
}

func getPluginModel(pluginID, tenantID string, dbmanager db.Manager) (string, error) {
	plugin, err := dbmanager.TenantPluginDao().GetPluginByID(pluginID, tenantID)
	if err != nil {
		return "", err
	}
	return plugin.PluginModel, nil
}

func createPluginArgs(cmd string, envs []v1.EnvVar) ([]string, error) {
	if cmd == "" {
		return nil, nil
	}
	configs := make(map[string]string, len(envs))
	for _, env := range envs {
		configs[env.Name] = env.Value
	}
	return strings.Split(util.ParseVariable(cmd, configs), " "), nil
}
func getXDSHostIPAndPort() (string, string) {
	dockerBridgeIP := "172.30.42.1"
	xdsHostPort := "6101"
	if os.Getenv("DOCKER_BRIDGE_IP") != "" {
		dockerBridgeIP = os.Getenv("DOCKER_BRIDGE_IP")
	}
	if os.Getenv("XDS_HOST_IP") != "" {
		dockerBridgeIP = os.Getenv("XDS_HOST_IP")
	}
	if os.Getenv("XDS_HOST_PORT") != "" {
		xdsHostPort = os.Getenv("XDS_HOST_PORT")
	}
	return dockerBridgeIP, xdsHostPort
}

//container envs
func createPluginEnvs(pluginID, tenantID, serviceAlias string, mainEnvs []v1.EnvVar, versionID, serviceID string, dbmanager db.Manager) (*[]v1.EnvVar, error) {
	versionEnvs, err := dbmanager.TenantPluginVersionENVDao().GetVersionEnvByServiceID(serviceID, pluginID)
	if err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
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
	dockerBridgeIP, xdsHostPort := getXDSHostIPAndPort()
	envs = append(envs, v1.EnvVar{Name: "XDS_HOST_IP", Value: dockerBridgeIP})
	envs = append(envs, v1.EnvVar{Name: "XDS_HOST_PORT", Value: xdsHostPort})
	discoverURL := fmt.Sprintf(
		"http://%s:6100/v1/resources/%s/%s/%s",
		dockerBridgeIP,
		tenantID,
		serviceAlias,
		pluginID)
	envs = append(envs, v1.EnvVar{Name: "DISCOVER_URL", Value: discoverURL})
	envs = append(envs, v1.EnvVar{Name: "DISCOVER_URL_NOHOST", Value: fmt.Sprintf(
		"/v1/resources/%s/%s/%s",
		tenantID,
		serviceAlias,
		pluginID)})
	envs = append(envs, v1.EnvVar{Name: "PLUGIN_ID", Value: pluginID})
	var config = make(map[string]string, len(envs))
	for _, env := range envs {
		config[env.Name] = env.Value
	}
	for i, env := range envs {
		envs[i].Value = util.ParseVariable(env.Value, config)
	}
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

//createAdapterResources current no limit
func createAdapterResources(memory int, cpu int) v1.ResourceRequirements {
	limits := v1.ResourceList{}
	// limits[v1.ResourceCPU] = *resource.NewMilliQuantity(
	// 	int64(cpu*3),
	// 	resource.DecimalSI)
	//limits[v1.ResourceMemory] = *resource.NewQuantity(
	//	int64(memory*1024*1024),
	//	resource.BinarySI)
	request := v1.ResourceList{}
	// request[v1.ResourceCPU] = *resource.NewMilliQuantity(
	// 	int64(cpu*2),
	// 	resource.DecimalSI)
	//request[v1.ResourceMemory] = *resource.NewQuantity(
	//	int64(memory*1024*1024),
	//	resource.BinarySI)
	return v1.ResourceRequirements{
		Limits:   limits,
		Requests: request,
	}
}

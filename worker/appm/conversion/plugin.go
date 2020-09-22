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
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	typesv1 "github.com/goodrain/rainbond/worker/appm/types/v1"
)

//TenantServicePlugin conv service all plugin
func TenantServicePlugin(as *typesv1.AppService, dbmanager db.Manager) error {
	initContainers, pluginContainers, bootSeqContainer, err := conversionServicePlugin(as, dbmanager)
	if err != nil {
		return err
	}
	as.BootSeqContainer = bootSeqContainer
	podtemplate := as.GetPodTemplate()
	if podtemplate != nil {
		podtemplate.Spec.Containers = append(podtemplate.Spec.Containers, pluginContainers...)
		podtemplate.Spec.InitContainers = initContainers
		return nil
	}
	return fmt.Errorf("pod templete is nil before define plugin")
}

func conversionServicePlugin(as *typesv1.AppService, dbmanager db.Manager) ([]v1.Container, []v1.Container, *v1.Container, error) {
	var containers []v1.Container
	var initContainers []v1.Container

	appPlugins, err := dbmanager.TenantServicePluginRelationDao().GetALLRelationByServiceID(as.ServiceID)
	if err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
		return nil, nil, nil, fmt.Errorf("find plugins error. %v", err.Error())
	}
	if len(appPlugins) == 0 && !as.NeedProxy {
		return nil, nil, nil, nil
	}

	netPlugin := false
	var meshPluginID string
	var mainContainer v1.Container
	if as.GetPodTemplate() != nil && len(as.GetPodTemplate().Spec.Containers) > 0 {
		mainContainer = as.GetPodTemplate().Spec.Containers[0]
	}

	var inBoundPlugin *model.TenantServicePluginRelation
	for _, pluginR := range appPlugins {
		//if plugin not enable,ignore it
		if pluginR.Switch == false {
			continue
		}
		versionInfo, err := dbmanager.TenantPluginBuildVersionDao().GetLastBuildVersionByVersionID(pluginR.PluginID, pluginR.VersionID)
		if err != nil {
			logrus.Errorf("do not found available plugin versions %s", pluginR.PluginID)
			continue
		}
		podTmpl := as.GetPodTemplate()
		if podTmpl == nil {
			logrus.Warnf("Can't not get pod for plugin(plugin_id=%s)", pluginR.PluginID)
			continue
		}
		envs, err := createPluginEnvs(pluginR.PluginID, as.TenantID, as.ServiceAlias, mainContainer.Env, pluginR.VersionID, as.ServiceID, dbmanager)
		if err != nil {
			return nil, nil, nil, err
		}
		args, err := createPluginArgs(versionInfo.ContainerCMD, *envs)
		if err != nil {
			return nil, nil, nil, err
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
			return nil, nil, nil, fmt.Errorf("get plugin model info failure %s", err.Error())
		}
		if pluginModel == model.InBoundAndOutBoundNetPlugin || pluginModel == model.InBoundNetPlugin {
			inBoundPlugin = pluginR
		}
		if pluginModel == model.OutBoundNetPlugin || pluginModel == model.InBoundAndOutBoundNetPlugin {
			netPlugin = true
			meshPluginID = pluginR.PluginID
		}
		if pluginModel == model.InitPlugin {
			initContainers = append(initContainers, pc)
		} else {
			containers = append(containers, pc)
		}
	}

	var inboundPluginConfig *api_model.ResourceSpec
	//apply plugin dynamic config
	if inBoundPlugin != nil {
		config, err := dbmanager.TenantPluginVersionConfigDao().GetPluginConfig(inBoundPlugin.ServiceID,
			inBoundPlugin.PluginID)
		if err != nil && err != gorm.ErrRecordNotFound {
			logrus.Errorf("get service plugin config from db failure %s", err.Error())
		}
		if config != nil {
			var resourceConfig api_model.ResourceSpec
			if err := json.Unmarshal([]byte(config.ConfigStr), &resourceConfig); err == nil {
				inboundPluginConfig = &resourceConfig
			}
		}
	}

	//create plugin config to configmap
	for i := range appPlugins {
		ApplyPluginConfig(as, appPlugins[i], dbmanager, inboundPluginConfig)
	}

	//if need proxy but not install net plugin
	if as.NeedProxy && !netPlugin {
		pluginID, err := applyDefaultMeshPluginConfig(as, dbmanager)
		if err != nil {
			logrus.Errorf("apply default mesh plugin config failure %s", err.Error())
		}
		c2 := createTCPDefaultPluginContainer(as, pluginID, mainContainer.Env)
		containers = append(containers, c2)
		meshPluginID = pluginID
	}

	var bootSequence v1.Container
	if needStartupSequence := as.ExtensionSet["needStartupSequence"]; needStartupSequence == "true" {
		startupSequenceDetector := newStartupSequenceDetector(as.ServiceID, dbmanager)
		dependServices, dependServiceNum, err := startupSequenceDetector.dependServices()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("detect depend services: %v", err)
		}

		if dependServices != "" {
			envs := mainContainer.Env
			envs = append(envs, corev1.EnvVar{Name: "DEPEND_SERVICE", Value: dependServices})
			envs = append(envs, corev1.EnvVar{Name: "DEPEND_SERVICE_COUNT", Value: strconv.Itoa(dependServiceNum)})
			bootSequence = createProbeMeshInitContainer(as, meshPluginID, as.ServiceAlias, envs)
			initContainers = append(initContainers, bootSequence)
		}
	}
	return initContainers, containers, &bootSequence, nil
}

func createTCPDefaultPluginContainer(as *typesv1.AppService, pluginID string, envs []v1.EnvVar) v1.Container {
	envs = append(envs, v1.EnvVar{Name: "PLUGIN_ID", Value: pluginID})
	xdsHost, xdsHostPort, apiHostPort := getXDSHostIPAndPort()
	envs = append(envs, xdsHostIPEnv(xdsHost))
	envs = append(envs, v1.EnvVar{Name: "API_HOST_PORT", Value: apiHostPort})
	envs = append(envs, v1.EnvVar{Name: "XDS_HOST_PORT", Value: xdsHostPort})

	return v1.Container{
		Name:      "default-tcpmesh-" + as.ServiceID[len(as.ServiceID)-20:],
		Env:       envs,
		Image:     typesv1.GetTCPMeshImageName(),
		Resources: createTCPUDPMeshRecources(as),
	}
}

func createProbeMeshInitContainer(as *typesv1.AppService, pluginID, serviceAlias string, envs []v1.EnvVar) v1.Container {
	envs = append(envs, v1.EnvVar{Name: "PLUGIN_ID", Value: pluginID})
	xdsHost, xdsHostPort, apiHostPort := getXDSHostIPAndPort()
	envs = append(envs, xdsHostIPEnv(xdsHost))
	envs = append(envs, v1.EnvVar{Name: "API_HOST_PORT", Value: apiHostPort})
	envs = append(envs, v1.EnvVar{Name: "XDS_HOST_PORT", Value: xdsHostPort})
	return v1.Container{
		Name:      "probe-mesh-" + as.ServiceID[len(as.ServiceID)-20:],
		Env:       envs,
		Image:     typesv1.GetProbeMeshImageName(),
		Resources: createTCPUDPMeshRecources(as),
	}
}

//ApplyPluginConfig applyPluginConfig
func ApplyPluginConfig(as *typesv1.AppService, servicePluginRelation *model.TenantServicePluginRelation,
	dbmanager db.Manager, inboundPluginConfig *api_model.ResourceSpec) {
	config, err := dbmanager.TenantPluginVersionConfigDao().GetPluginConfig(servicePluginRelation.ServiceID,
		servicePluginRelation.PluginID)
	if err != nil && err != gorm.ErrRecordNotFound {
		logrus.Errorf("get service plugin config from db failure %s", err.Error())
	}
	if config != nil {
		configStr := config.ConfigStr
		//if have inbound plugin,will Propagate its listner port to other plug-ins
		if inboundPluginConfig != nil {
			var oldConfig api_model.ResourceSpec
			if err := json.Unmarshal([]byte(configStr), &oldConfig); err == nil {
				for i := range oldConfig.BasePorts {
					for j := range inboundPluginConfig.BasePorts {
						if oldConfig.BasePorts[i].Port == inboundPluginConfig.BasePorts[j].Port {
							oldConfig.BasePorts[i].ListenPort = inboundPluginConfig.BasePorts[j].ListenPort
						}
					}
				}
				if newConfig, err := json.Marshal(&oldConfig); err == nil {
					configStr = string(newConfig)
				}
			}
		}
		cm := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("config-%s-%s", config.ServiceID, config.PluginID),
				Labels: as.GetCommonLabels(map[string]string{
					"plugin_id":     servicePluginRelation.PluginID,
					"service_alias": as.ServiceAlias,
				}),
			},
			Data: map[string]string{
				"plugin-config": configStr,
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
			if *port.IsInnerService {
				depService := &api_model.BaseService{
					ServiceAlias:       as.ServiceAlias,
					ServiceID:          as.ServiceID,
					DependServiceAlias: depService.ServiceAlias,
					DependServiceID:    depService.ServiceID,
					Port:               port.ContainerPort,
					Protocol:           port.Protocol,
				}
				baseServices = append(baseServices, depService)
			}
		}
	}
	var res = &api_model.ResourceSpec{
		BaseServices: baseServices,
	}
	resJSON, err := json.Marshal(res)
	if err != nil {
		return "", err
	}
	pluginID := "def-mesh" + as.ServiceID
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
			"plugin-model":  model.OutBoundNetPlugin,
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
func getXDSHostIPAndPort() (string, string, string) {
	xdsHost := ""
	xdsHostPort := "6101"
	apiHostPort := "6100"
	if os.Getenv("XDS_HOST_IP") != "" {
		xdsHost = os.Getenv("XDS_HOST_IP")
	}
	if os.Getenv("XDS_HOST_PORT") != "" {
		xdsHostPort = os.Getenv("XDS_HOST_PORT")
	}
	if os.Getenv("API_HOST_PORT") != "" {
		apiHostPort = os.Getenv("API_HOST_PORT")
	}
	return xdsHost, xdsHostPort, apiHostPort
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
	xdsHost, xdsHostPort, apiHostPort := getXDSHostIPAndPort()
	envs = append(envs, xdsHostIPEnv(xdsHost))
	envs = append(envs, v1.EnvVar{Name: "API_HOST_PORT", Value: apiHostPort})
	envs = append(envs, v1.EnvVar{Name: "XDS_HOST_PORT", Value: xdsHostPort})
	discoverURL := fmt.Sprintf(
		"http://%s:6100/v1/resources/%s/%s/%s",
		"${XDS_HOST_IP}",
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
	case model.InBoundNetPlugin:
		return 9
	case model.OutBoundNetPlugin:
		return 8
	case model.GeneralPlugin:
		return 1
	default:
		return 0
	}
}

func createPluginResources(memory int, cpu int) v1.ResourceRequirements {
	return createResourcesByDefaultCPU(memory, int64(cpu), int64(cpu))
}

func createTCPUDPMeshRecources(as *typesv1.AppService) v1.ResourceRequirements {
	var memory = 128
	var cpu int64
	if limit, ok := as.ExtensionSet["tcpudp_mesh_cpu"]; ok {
		limitint, _ := strconv.Atoi(limit)
		if limitint > 0 {
			cpu = int64(limitint)
		}
	}
	if request, ok := as.ExtensionSet["tcpudp_mesh_memory"]; ok {
		requestint, _ := strconv.Atoi(request)
		if requestint > 0 {
			memory = requestint
		}
	}
	return createResourcesByDefaultCPU(memory, cpu, func() int64 {
		if cpu < 120 {
			return 120
		}
		return cpu
	}())
}

func xdsHostIPEnv(xdsHost string) corev1.EnvVar {
	if xdsHost == "" {
		return v1.EnvVar{Name: "XDS_HOST_IP", ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "status.hostIP",
			},
		}}
	}
	return v1.EnvVar{Name: "XDS_HOST_IP", Value: xdsHost}
}

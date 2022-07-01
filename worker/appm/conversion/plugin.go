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
	"github.com/goodrain/rainbond/worker/appm/volume"
	"os"
	"strconv"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
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
	initContainers, preContainers, postContainers, err := conversionServicePlugin(as, dbmanager)
	if err != nil {
		logrus.Errorf("create plugin containers for component %s failure: %s", as.ServiceID, err.Error())
		return err
	}
	podtemplate := as.GetPodTemplate()
	if podtemplate != nil {
		if len(preContainers) > 0 {
			podtemplate.Spec.Containers = append(preContainers, podtemplate.Spec.Containers...)
		}
		if len(postContainers) > 0 {
			podtemplate.Spec.Containers = append(podtemplate.Spec.Containers, postContainers...)
		}
		podtemplate.Spec.InitContainers = initContainers
		return nil
	}
	return fmt.Errorf("pod templete is nil before define plugin")
}

func conversionServicePlugin(as *typesv1.AppService, dbmanager db.Manager) ([]v1.Container, []v1.Container, []v1.Container, error) {
	var precontainers, postcontainers []v1.Container
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
		if !pluginR.Switch {
			logrus.Debugf("plugin %s is disable in component %s", pluginR.PluginID, as.ServiceID)
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
		envs, err := createPluginEnvs(pluginR.PluginID, as.GetNamespace(), as.ServiceAlias, mainContainer.Env, pluginR.VersionID, as.ServiceID, dbmanager)
		if err != nil {
			return nil, nil, nil, err
		}
		var envFromSecrets []corev1.EnvFromSource
		envVarSecrets := as.GetEnvVarSecrets(true)
		for _, secret := range envVarSecrets {
			envFromSecrets = append(envFromSecrets, corev1.EnvFromSource{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secret.Name,
					},
				},
			})
		}

		pc := v1.Container{
			Name:                   "plugin-" + pluginR.PluginID,
			Image:                  versionInfo.BuildLocalImage,
			Env:                    *envs,
			EnvFrom:                envFromSecrets,
			Resources:              createPluginResources(pluginR.ContainerMemory, pluginR.ContainerCPU),
			TerminationMessagePath: "",
			VolumeMounts:           mainContainer.VolumeMounts,
		}

		if len(versionInfo.ContainerCMD) > 0 {
			pc.Command = []string{"/bin/sh", "-c"}
			pc.Args = []string{versionInfo.ContainerCMD}
		}

		pluginModel, err := getPluginModel(pluginR.PluginID, as.TenantID, dbmanager)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("get plugin model info failure %s", err.Error())
		}
		var preconatiner = false
		if pluginModel == model.InBoundAndOutBoundNetPlugin || pluginModel == model.InBoundNetPlugin {
			inBoundPlugin = pluginR
			preconatiner = true
		}
		if pluginModel == model.OutBoundNetPlugin || pluginModel == model.InBoundAndOutBoundNetPlugin {
			netPlugin = true
			meshPluginID = pluginR.PluginID
			preconatiner = true
		}
		if netPlugin {
			config, err := dbmanager.TenantPluginVersionConfigDao().GetPluginConfig(as.ServiceID, pluginR.PluginID)
			if err != nil && err != gorm.ErrRecordNotFound {
				logrus.Errorf("get service plugin config from db failure %s", err.Error())
			}
			if config != nil {
				var resourceConfig api_model.ResourceSpec
				if err := json.Unmarshal([]byte(config.ConfigStr), &resourceConfig); err != nil {
					logrus.Warningf("load mesh plugin %s config of componet %s failure %v", pluginR.PluginID, as.ServiceID, err.Error())
				}
				if len(resourceConfig.BaseServices) > 0 {
					setSidecarContainerLifecycle(as, &pc, &resourceConfig)
				}
			}
		}
		if pluginModel == model.InitPlugin {
			if strings.ToLower(os.Getenv("DISABLE_INIT_CONTAINER_ENABLE_SECURITY")) != "true" {
				//init container default open security
				pc.SecurityContext = &corev1.SecurityContext{Privileged: util.Bool(true)}
			}
			initContainers = append(initContainers, pc)
		} else if preconatiner {
			precontainers = append(precontainers, pc)
		} else {
			postcontainers = append(postcontainers, pc)
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
		config, err := dbmanager.TenantPluginVersionConfigDao().GetPluginConfig(appPlugins[i].ServiceID,
			appPlugins[i].PluginID)
		if err != nil && err != gorm.ErrRecordNotFound {
			logrus.Errorf("get service plugin config from db failure %s", err.Error())
		}
		if config != nil {
			configStr := config.ConfigStr
			var oldConfig api_model.ResourceSpec
			if err := json.Unmarshal([]byte(configStr), &oldConfig); err == nil {
				for key, plugin := range oldConfig.BaseNormal.Options {
					var pluginStorage api_model.PluginStorage
					jsonValue, ok := plugin.(string)
					if !ok {
						continue
					}
					if err := json.Unmarshal([]byte(jsonValue), &pluginStorage); err != nil {
						continue
					}
					if pluginStorage.VolumeName != "" || pluginStorage.VolumePath != "" {
						IsContainMount(&mainContainer.VolumeMounts, as, pluginStorage, config.PluginID)
						delete(oldConfig.BaseNormal.Options, key)
					}
				}
			}
		}
		ApplyPluginConfig(as, appPlugins[i], dbmanager, inboundPluginConfig)
	}
	//if need proxy but not install net plugin
	if as.NeedProxy && !netPlugin {
		pluginID, pluginConfig, err := applyDefaultMeshPluginConfig(as, dbmanager)
		if err != nil {
			logrus.Errorf("apply default mesh plugin config failure %s", err.Error())
		}
		defaultSidecarContainer := createTCPDefaultPluginContainer(as, pluginID, mainContainer.Env, pluginConfig)
		precontainers = append(precontainers, defaultSidecarContainer)
		meshPluginID = pluginID
	}

	bootSequence := createProbeMeshInitContainer(as, meshPluginID, as.ServiceAlias, mainContainer.Env)
	if bootSeqDepServiceIds := as.ExtensionSet["boot_seq_dep_service_ids"]; as.NeedProxy && bootSeqDepServiceIds != "" {
		initContainers = append(initContainers, bootSequence)
	}
	as.BootSeqContainer = &bootSequence
	if len(as.GetPodTemplate().Spec.Containers) != 0 {
		pluginMountVolume(initContainers, as.GetPodTemplate().Spec.Containers[0].VolumeMounts)
		pluginMountVolume(precontainers, as.GetPodTemplate().Spec.Containers[0].VolumeMounts)
		pluginMountVolume(postcontainers, as.GetPodTemplate().Spec.Containers[0].VolumeMounts)
	}
	return initContainers, precontainers, postcontainers, nil
}

func createTCPDefaultPluginContainer(as *typesv1.AppService, pluginID string, envs []v1.EnvVar, pluginConfig *api_model.ResourceSpec) v1.Container {
	envs = append(envs, v1.EnvVar{Name: "PLUGIN_ID", Value: pluginID})
	xdsHost, xdsHostPort, apiHostPort := getXDSHostIPAndPort()
	envs = append(envs, xdsHostIPEnv(xdsHost))
	envs = append(envs, v1.EnvVar{Name: "API_HOST_PORT", Value: apiHostPort})
	envs = append(envs, v1.EnvVar{Name: "XDS_HOST_PORT", Value: xdsHostPort})

	container := v1.Container{
		Name:      "default-tcpmesh-" + as.GetK8sWorkloadName(),
		Env:       envs,
		Image:     typesv1.GetOnlineTCPMeshImageName(),
		Resources: createTCPUDPMeshRecources(as),
	}

	setSidecarContainerLifecycle(as, &container, pluginConfig)
	return container
}

func setSidecarContainerLifecycle(as *typesv1.AppService, con *corev1.Container, pluginConfig *api_model.ResourceSpec) {
	if strings.ToLower(as.ExtensionSet["disable_sidecar_check"]) != "true" {
		var port int
		if as.ExtensionSet["sidecar_check_port"] != "" {
			cport, _ := strconv.Atoi(as.ExtensionSet["sidecar_check_port"])
			if cport != 0 {
				port = cport
			}
		}
		if port == 0 {
			for _, dep := range pluginConfig.BaseServices {
				if strings.ToLower(dep.Protocol) != "udp" {
					port = dep.Port
					break
				}
			}
			if port == 0 {
				for _, bport := range pluginConfig.BasePorts {
					if strings.ToLower(bport.Protocol) != "udp" {
						port = bport.Port
						break
					}
				}
			}
		}
		con.Lifecycle = &corev1.Lifecycle{
			PostStart: &corev1.Handler{
				Exec: &v1.ExecAction{
					Command: []string{"/root/rainbond-mesh-data-panel", "wait", strconv.Itoa(port)},
				},
			},
		}
	}
}

func createProbeMeshInitContainer(as *typesv1.AppService, pluginID, serviceAlias string, envs []v1.EnvVar) v1.Container {
	envs = append(envs, v1.EnvVar{Name: "PLUGIN_ID", Value: pluginID})
	xdsHost, xdsHostPort, apiHostPort := getXDSHostIPAndPort()
	envs = append(envs, xdsHostIPEnv(xdsHost))
	envs = append(envs, v1.EnvVar{Name: "API_HOST_PORT", Value: apiHostPort})
	envs = append(envs, v1.EnvVar{Name: "XDS_HOST_PORT", Value: xdsHostPort})

	return v1.Container{
		Name:      "probe-mesh-" + as.GetK8sWorkloadName(),
		Env:       envs,
		Image:     typesv1.GetOnlineProbeMeshImageName(),
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
func applyDefaultMeshPluginConfig(as *typesv1.AppService, dbmanager db.Manager) (string, *api_model.ResourceSpec, error) {
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
		return "", nil, err
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
	return pluginID, res, nil
}

func getPluginModel(pluginID, tenantID string, dbmanager db.Manager) (string, error) {
	plugin, err := dbmanager.TenantPluginDao().GetPluginByID(pluginID, tenantID)
	if err != nil {
		return "", err
	}
	return plugin.PluginModel, nil
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

func createPluginResources(memory int, cpu int) v1.ResourceRequirements {
	return createResourcesBySetting(memory, int64(cpu), int64(cpu), 0)
}

func createTCPUDPMeshRecources(as *typesv1.AppService) v1.ResourceRequirements {
	var memory int
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
	return createResourcesBySetting(memory, cpu, func() int64 {
		if 0 < cpu && cpu < 120 {
			return 120
		}
		return cpu
	}(), 0)
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

//IsContainMount 判断存储路径是否冲突，以及进一步实现创建存储或配置文件
func IsContainMount(volumeMounts *[]v1.VolumeMount, as *typesv1.AppService, plugin api_model.PluginStorage, pluginID string) bool {

	for _, mountValue := range *volumeMounts {
		if mountValue.MountPath == plugin.VolumePath {
			return false
		}
	}
	volumeMount := v1.VolumeMount{Name: plugin.VolumeName, MountPath: plugin.VolumePath}
	*volumeMounts = append(*volumeMounts, volumeMount)
	var define = &volume.Define{}
	v := new(volume.PluginStorageVolume)
	v.Plugin = plugin
	v.AS = as
	v.PluginID = pluginID
	if err := v.CreateVolume(define); err != nil {
		return false
	}
	as.GetPodTemplate().Spec.Volumes = append(as.GetPodTemplate().Spec.Volumes, define.GetVolumes()...)
	if len(as.GetPodTemplate().Spec.Containers) != 0 {
		as.GetPodTemplate().Spec.Containers[0].VolumeMounts = append(as.GetPodTemplate().Spec.Containers[0].VolumeMounts, define.GetVolumeMounts()...)
	}
	return true
}

func pluginMountVolume(containers []v1.Container, volumeMount []v1.VolumeMount) {
	for i := range containers {
		containers[i].VolumeMounts = volumeMount
	}
}

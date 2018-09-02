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

package appm

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"

	"github.com/Sirupsen/logrus"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/pkg/api/v1"
)

//PodTemplateSpecBuild pod build
type PodTemplateSpecBuild struct {
	serviceID, eventID string
	needProxy          bool
	hostName           string
	service            *model.TenantServices
	tenant             *model.Tenants
	pluginsRelation    []*model.TenantServicePluginRelation
	dbmanager          db.Manager
	logger             event.Logger
	versionInfo        *model.VersionInfo
	localScheduler     bool
	volumeMount        map[string]string
	NodeAPI            string
}

//PodTemplateSpecBuilder pod builder
func PodTemplateSpecBuilder(serviceID string, logger event.Logger, nodeAPI string) (*PodTemplateSpecBuild, error) {
	dbmanager := db.GetManager()
	service, err := dbmanager.TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		return nil, fmt.Errorf("find service error. %v", err.Error())
	}
	tenant, err := dbmanager.TenantDao().GetTenantByUUID(service.TenantID)
	if err != nil {
		return nil, fmt.Errorf("find tenant error. %v", err.Error())
	}
	pluginRelations, err := dbmanager.TenantServicePluginRelationDao().GetALLRelationByServiceID(serviceID)
	if err != nil {
		return nil, fmt.Errorf("find plugins error. %v", err.Error())
	}
	versionInfo, err := dbmanager.VersionInfoDao().GetVersionByDeployVersion(service.DeployVersion, serviceID)
	if err != nil {
		logrus.Warnf("error get versioninfo table by key %s,prepare use default", service.DeployVersion)
		var buildType = "image"
		path := service.ImageName
		if strings.HasPrefix(service.ImageName, builder.RUNNERIMAGENAME) {
			buildType = "slug"
			path = fmt.Sprintf("/grdata/build/tenant/%s/slug/%s/%s.tgz", service.TenantID, service.ServiceID, service.DeployVersion)
		}
		versionInfo = &model.VersionInfo{
			DeliveredType: buildType,
			DeliveredPath: path,
		}
	}
	return &PodTemplateSpecBuild{
		serviceID:       serviceID,
		eventID:         logger.Event(),
		dbmanager:       dbmanager,
		pluginsRelation: pluginRelations,
		service:         service,
		tenant:          tenant,
		versionInfo:     versionInfo,
		logger:          logger,
		volumeMount:     make(map[string]string),
		NodeAPI:         nodeAPI,
	}, nil
}

//GetTenant get tenant
func (p *PodTemplateSpecBuild) GetTenant() *model.Tenants {
	return p.tenant
}

//GetService get service
func (p *PodTemplateSpecBuild) GetService() *model.TenantServices {
	return p.service
}

//Build 通过service 构建pod template
func (p *PodTemplateSpecBuild) Build() (*v1.PodTemplateSpec, error) {

	//step1:构建环境变量定义
	envs, err := p.createEnv()
	if err != nil {
		return nil, fmt.Errorf("create envs in pod template error :%v", err.Error())
	}
	//step2:构建挂载定义
	volumes, volumeMounts, err := p.createVolumes(envs)
	if err != nil {
		return nil, fmt.Errorf("create volume in pod template error :%v", err.Error())
	}
	//step3.0:构建initContainer
	initContainers, plugincontainers, err := p.createPluginsContainer(volumeMounts, envs)
	if err != nil {
		return nil, fmt.Errorf("create plugin container error. %v", err.Error())
	}
	//step3.1:构建容器定义
	containers := p.createContainer(volumeMounts, envs)
	if len(plugincontainers) != 0 {
		for _, plugin := range plugincontainers {
			containers = append(containers, plugin)
		}
	}
	//step4:Node selector
	nodeSelector := p.createNodeSelector()
	//step5:pod 亲和性

	podSpec := v1.PodSpec{
		Volumes:      volumes,
		Containers:   containers,
		NodeSelector: nodeSelector,
		Affinity:     p.createAffinity(),
	}
	if len(initContainers) != 0 {
		podSpec.InitContainers = initContainers
	}

	//step6:构建pod label
	labels := map[string]string{
		"name":        p.service.ServiceAlias,
		"version":     p.service.DeployVersion,
		"tenant_name": p.tenant.Name,
		"event_id":    p.eventID,
	}
	//step7:插件启动排序
	pid, err := p.sortPlugins()
	if err != nil {
		return nil, fmt.Errorf("sort plugin errro. %v", err.Error())
	}
	for k, v := range pid {
		labels[fmt.Sprintf("f%d", k)] = v
	}
	//设置为本地调度，调度器会识别此label
	//只要statefulset应用支持本地调度
	if p.localScheduler {
		serviceType, err := p.dbmanager.TenantServiceLabelDao().GetTenantServiceTypeLabel(p.serviceID)
		if err != nil {
			return nil, fmt.Errorf("get service type error.don't support local scheduler")
		}
		if serviceType.LabelValue == util.StatefulServiceType {
			labels["local-scheduler"] = "true"
			ls, err := p.dbmanager.LocalSchedulerDao().GetLocalScheduler(p.serviceID)
			if err != nil {
				logrus.Error("get local scheduler info error.", err.Error())
				return nil, err
			}
			if ls != nil {
				for _, l := range ls {
					labels["scheduler-"+l.PodName] = l.NodeIP
				}
			}
		}
	}
	outPorts, err := p.dbmanager.TenantServicesPortDao().GetOuterPorts(p.serviceID)
	if err != nil {
		return nil, fmt.Errorf("find outer ports error. %v", err.Error())
	}
	if outPorts != nil && len(outPorts) > 0 {
		crt, err := p.checkUpstreamPluginRelation()
		if err != nil {
			return nil, fmt.Errorf("get service upstream plugin relation error, %s", err.Error())
		}
		if crt {
			pluginPorts, err := p.dbmanager.TenantServicesStreamPluginPortDao().GetPluginMappingPorts(
				p.serviceID,
				model.UpNetPlugin,
			)
			if err != nil {
				return nil, fmt.Errorf("find upstream plugin mapping port error, %s", err.Error())
			}
			outPorts, err = p.CreateUpstreamPluginMappingPort(outPorts, pluginPorts)
		}
		labels["service_type"] = "outer"
		var pStr string
		for _, p := range outPorts {
			if pStr != "" {
				pStr += "-.-"
			}
			pStr += fmt.Sprintf("%d_._%s", p.ContainerPort, p.Protocol)
		}
		labels["protocols"] = pStr
	}
	//step7: set hostname
	if p.hostName != "" {
		logrus.Infof("set pod name is %s", p.hostName)
		podSpec.Hostname = p.hostName
	}
	//step8: 构建PodTemplateSpec
	temp := v1.PodTemplateSpec{
		Spec: podSpec,
	}
	temp.Annotations = p.createPodAnnotations()
	temp.Labels = labels
	return &temp, nil
}

//createPodAnnotations create pod annotation
func (p *PodTemplateSpecBuild) createPodAnnotations() map[string]string {
	var annotations = make(map[string]string)
	if p.service.Replicas <= 1 {
		annotations["rainbond.com/tolerate-unready-endpoints"] = "true"
	}
	return annotations
}

//TODO:
//节点亲和性 应用亲和性
func (p *PodTemplateSpecBuild) createAffinity() *v1.Affinity {
	var affinity v1.Affinity
	labels, err := p.dbmanager.TenantServiceLabelDao().GetTenantServiceAffinityLabel(p.serviceID)
	if err == nil && labels != nil && len(labels) > 0 {
		nsr := make([]v1.NodeSelectorRequirement, 0)
		podAffinity := make([]v1.PodAffinityTerm, 0)
		podAntAffinity := make([]v1.PodAffinityTerm, 0)
		for _, l := range labels {
			if l.LabelKey == model.LabelKeyNodeAffinity {
				nsr = append(nsr, v1.NodeSelectorRequirement{
					Key:      l.LabelKey,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{l.LabelValue},
				})
			}
			if l.LabelKey == model.LabelKeyServiceAffinity {
				podAffinity = append(podAffinity, v1.PodAffinityTerm{
					LabelSelector: metav1.SetAsLabelSelector(map[string]string{
						"name": l.LabelValue,
					}),
				})
			}
			if l.LabelKey == model.LabelKeyServiceAntyAffinity {
				podAntAffinity = append(
					podAntAffinity, v1.PodAffinityTerm{
						LabelSelector: metav1.SetAsLabelSelector(map[string]string{
							"name": l.LabelValue,
						}),
					})
			}
		}
		affinity.NodeAffinity = &v1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					v1.NodeSelectorTerm{MatchExpressions: nsr},
				},
			},
		}
		affinity.PodAffinity = &v1.PodAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: podAffinity,
		}
		affinity.PodAntiAffinity = &v1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: podAntAffinity,
		}
	}
	return &affinity
}
func (p *PodTemplateSpecBuild) createNodeSelector() map[string]string {
	selector := make(map[string]string)
	labels, err := p.dbmanager.TenantServiceLabelDao().GetTenantServiceNodeSelectorLabel(p.serviceID)
	if err == nil && labels != nil && len(labels) > 0 {
		for _, l := range labels {
			//应用机器选择标签的值作为机器标签的key,值都使用node
			selector[l.LabelValue] = model.LabelKeyNodeSelector
		}
	}
	return selector
}

func (p *PodTemplateSpecBuild) checkUpstreamPluginRelation() (bool, error) {
	return p.dbmanager.TenantServicePluginRelationDao().CheckSomeModelPluginByServiceID(
		p.serviceID,
		model.UpNetPlugin)
}

//CreateUpstreamPluginMappingPort 检查是否存在upstream插件，接管入口网络
func (p *PodTemplateSpecBuild) CreateUpstreamPluginMappingPort(
	ports []*model.TenantServicesPort,
	pluginPorts []*model.TenantServicesStreamPluginPort,
) (
	[]*model.TenantServicesPort,
	error) {
	//start from 65301
	for i := range ports {
		port := ports[i]
		for _, pport := range pluginPorts {
			if pport.ContainerPort == port.ContainerPort {
				port.ContainerPort = pport.PluginPort
				port.MappingPort = pport.PluginPort
			}
		}
	}
	return ports, nil
}

func (p *PodTemplateSpecBuild) createContainer(volumeMounts []v1.VolumeMount, envs *[]v1.EnvVar) []v1.Container {
	var containers []v1.Container
	//create app container
	var containerName string
	for _, e := range *envs {
		if e.Name == "CONTAINERNAME" {
			if e.Value != "" {
				containerName = e.Value
			}
			break
		}
	}
	if containerName == "" {
		containerName = p.serviceID
	}
	c1 := v1.Container{
		Name:                   containerName,
		Image:                  p.service.ImageName,
		Env:                    *envs,
		Ports:                  p.createPorts(),
		Resources:              p.createResources(),
		TerminationMessagePath: "",
		ReadinessProbe:         p.createProbe("readiness"),
		LivenessProbe:          p.createProbe("liveness"),
		VolumeMounts:           volumeMounts,
		Args:                   p.createArgs(*envs),
	}
	if p.versionInfo.DeliveredType == "slug" {
		c1.Image = builder.RUNNERIMAGENAME
	}
	if p.versionInfo.DeliveredType == "image" {
		c1.Image = p.versionInfo.DeliveredPath
	}
	containers = append(containers, c1)
	return containers
}
func (p *PodTemplateSpecBuild) createArgs(envs []v1.EnvVar) (args []string) {
	if p.service.ContainerCMD == "" {
		return
	}
	cmd := p.service.ContainerCMD
	var reg = regexp.MustCompile(`(?U)\$\{.*\}`)
	resultKey := reg.FindAllString(cmd, -1)
	for _, rk := range resultKey {
		value := getenv(GetConfigKey(rk), envs)
		cmd = strings.Replace(cmd, rk, value, -1)
	}
	args = strings.Split(cmd, " ")
	args = util.RemoveSpaces(args)
	return args
}

//GetConfigKey 获取配置key
func GetConfigKey(rk string) string {
	if len(rk) < 4 {
		return ""
	}
	left := strings.Index(rk, "{")
	right := strings.Index(rk, "}")
	return rk[left+1 : right]
}

func getenv(key string, envs []v1.EnvVar) string {
	for _, env := range envs {
		if env.Name == key {
			return env.Value
		}
	}
	return ""
}

func (p *PodTemplateSpecBuild) createProbe(mode string) *v1.Probe {
	//TODO:应用创建时如果有开端口，创建默认探针
	probe, err := p.dbmanager.ServiceProbeDao().GetServiceUsedProbe(p.serviceID, mode)
	if err == nil && probe != nil {
		if mode == "liveness" && probe.SuccessThreshold < 1 {
			probe.SuccessThreshold = 1
		}
		if mode == "readiness" && probe.FailureThreshold < 1 {
			probe.FailureThreshold = 3
		}
		p := &v1.Probe{
			FailureThreshold:    int32(probe.FailureThreshold),
			SuccessThreshold:    int32(probe.SuccessThreshold),
			InitialDelaySeconds: int32(probe.InitialDelaySecond),
			TimeoutSeconds:      int32(probe.TimeoutSecond),
			PeriodSeconds:       int32(probe.PeriodSecond),
		}
		if probe.Scheme == "tcp" {
			tcp := &v1.TCPSocketAction{
				Port: intstr.FromInt(probe.Port),
			}
			p.TCPSocket = tcp
			return p
		} else if probe.Scheme == "http" {
			action := v1.HTTPGetAction{Path: probe.Path, Port: intstr.FromInt(probe.Port)}
			if probe.HTTPHeader != "" {
				hds := strings.Split(probe.HTTPHeader, ",")
				var headers []v1.HTTPHeader
				for _, hd := range hds {
					kv := strings.Split(hd, "=")
					if len(kv) == 1 {
						header := v1.HTTPHeader{
							Name:  kv[0],
							Value: "",
						}
						headers = append(headers, header)
					} else if len(kv) == 2 {
						header := v1.HTTPHeader{
							Name:  kv[0],
							Value: kv[1],
						}
						headers = append(headers, header)
					}
				}
				action.HTTPHeaders = headers
			}
			p.HTTPGet = &action
			return p
		}
		return nil
	}
	if err != nil {
		logrus.Error("query probe error:", err.Error())
	}
	//TODO:使用默认探针
	return nil
}

//createAdapterResources
//memory Mb
//cpu (core*1000)
//TODO:内存暂时不限制
func (p *PodTemplateSpecBuild) createAdapterResources(memory int, cpu int) v1.ResourceRequirements {
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

//createPluginResources
//memory Mb
//cpu (core*1000)
//TODO:插件的资源限制，CPU暂时不限制
func (p *PodTemplateSpecBuild) createPluginResources(memory int, cpu int) v1.ResourceRequirements {
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

func (p *PodTemplateSpecBuild) createResources() v1.ResourceRequirements {
	var cpuRequest, cpuLimit int64
	memory := p.service.ContainerMemory
	if memory < 512 {
		cpuRequest, cpuLimit = int64(memory)/128*30, int64(memory)/128*80
	} else if memory <= 1024 {
		cpuRequest, cpuLimit = int64(memory)/128*30, int64(memory)/128*160
	} else {
		cpuRequest, cpuLimit = int64(memory)/128*30, ((int64(memory)-1024)/1024*500 + 1280)
	}
	limits := v1.ResourceList{}
	limits[v1.ResourceCPU] = *resource.NewMilliQuantity(
		cpuLimit,
		resource.DecimalSI)
	limits[v1.ResourceMemory] = *resource.NewQuantity(
		int64(p.service.ContainerMemory*1024*1024),
		resource.BinarySI)
	request := v1.ResourceList{}
	request[v1.ResourceCPU] = *resource.NewMilliQuantity(
		cpuRequest,
		resource.DecimalSI)
	request[v1.ResourceMemory] = *resource.NewQuantity(
		int64(p.service.ContainerMemory*1024*1024),
		resource.BinarySI)
	return v1.ResourceRequirements{
		Limits:   limits,
		Requests: request,
	}
}

func (p *PodTemplateSpecBuild) createPorts() (ports []v1.ContainerPort) {
	ps, err := p.dbmanager.TenantServicesPortDao().GetPortsByServiceID(p.serviceID)
	if err == nil && ps != nil && len(ps) > 0 {
		crt, err := p.checkUpstreamPluginRelation()
		if err != nil {
			//return nil, fmt.Errorf("get service upstream plugin relation error, %s", err.Error())
			return
		}
		if crt {
			pluginPorts, err := p.dbmanager.TenantServicesStreamPluginPortDao().GetPluginMappingPorts(
				p.serviceID,
				model.UpNetPlugin,
			)
			if err != nil {
				//return nil, fmt.Errorf("find upstream plugin mapping port error, %s", err.Error())
				return
			}
			ps, err = p.CreateUpstreamPluginMappingPort(ps, pluginPorts)
		}
		for i := range ps {
			p := ps[i]
			var hostPort int32
			if p.IsOuterService && os.Getenv("CUR_NET") == "midonet" {
				hostPort = 1
			}
			ports = append(ports, v1.ContainerPort{
				HostPort:      hostPort,
				ContainerPort: int32(p.ContainerPort),
			})
		}
	}
	return
}

func (p *PodTemplateSpecBuild) createVolumes(envs *[]v1.EnvVar) ([]v1.Volume, []v1.VolumeMount, error) {
	var volumes []v1.Volume
	var volumeMounts []v1.VolumeMount
	//应用自定义挂载
	vs, err := p.dbmanager.TenantServiceVolumeDao().GetTenantServiceVolumesByServiceID(p.serviceID)
	if err != nil {
		return nil, nil, err
	}
	if vs != nil && len(vs) > 0 {
		for i := range vs {
			v := vs[i]
			if v.VolumeType != model.MemoryFSVolumeType.String() {
				err := util.CheckAndCreateDir(v.HostPath)
				if err != nil {
					return nil, nil, fmt.Errorf("create host path %s error,%s", v.HostPath, err.Error())
				}
				os.Chmod(v.HostPath, 0777)
			}
			//应用含有本地存储，设置调度类型为本地调度
			if v.VolumeType == model.LocalVolumeType.String() {
				p.localScheduler = true
			}
			p.createVolumeObj(model.VolumeType(v.VolumeType), fmt.Sprintf("manual%d", v.ID), v.VolumePath, v.HostPath, v.IsReadOnly, &volumeMounts, &volumes)
		}
	}
	//应用本身定义挂载 TODO:确定本身挂载是否一定生效在自定义挂载有数据的情况下
	if p.service.VolumeMountPath != "" && p.service.VolumePath != "" {
		err := util.CheckAndCreateDir(p.service.HostPath)
		if err != nil {
			return nil, nil, fmt.Errorf("create host path %s error,%s", p.service.HostPath, err.Error())
		}
		os.Chmod(p.service.HostPath, 0777)
		p.createVolumeObj(model.VolumeType(p.service.VolumeType), p.service.VolumePath, p.service.VolumeMountPath, p.service.HostPath, false, &volumeMounts, &volumes)
	}
	//依赖挂载
	tsmr, err := p.dbmanager.TenantServiceMountRelationDao().GetTenantServiceMountRelationsByService(p.serviceID)
	if err != nil {
		return nil, nil, err
	}
	if vs != nil && len(tsmr) > 0 {
		for i := range tsmr {
			t := tsmr[i]
			err := util.CheckAndCreateDir(t.HostPath)
			if err != nil {
				return nil, nil, fmt.Errorf("create host path %s error,%s", t.HostPath, err.Error())
			}
			p.createVolumeObj(model.ShareFileVolumeType, fmt.Sprintf("mnt%d", t.ID), t.VolumePath, t.HostPath, false, &volumeMounts, &volumes)
		}
	}
	//处理slug挂载
	if p.versionInfo.DeliveredType == "slug" {
		var slugPath string
		for _, e := range *envs {
			if e.Name == "SLUG_PATH" {
				slugPath = e.Value
				break
			}
		}
		if slugPath != "" {
			slugPath = "/grdata/build/tenant/" + slugPath
		} else {
			slugPath = p.versionInfo.DeliveredPath
		}
		p.createVolumeObj(model.ShareFileVolumeType, "slug", "/tmp/slug/slug.tgz", slugPath, true, &volumeMounts, &volumes)
	}
	//有依赖的服务需要启动grproxy,挂载kubeconfig
	if p.needProxy {
		p.createVolumeObj(model.ShareFileVolumeType, "kube-config", "/etc/kubernetes", "/grdata/kubernetes", true, nil, &volumes)
	}
	return volumes, volumeMounts, nil
}
func (p *PodTemplateSpecBuild) createVolumeObj(VolumeType model.VolumeType, name, mountPath, hostPath string, readOnly bool, volumeMounts *[]v1.VolumeMount, volumes *[]v1.Volume) {
	//Ensure mount directory is unique.
	if _, ok := p.volumeMount[mountPath]; ok {
		return
	}
	p.volumeMount[mountPath] = mountPath
	if volumeMounts != nil {
		vm := v1.VolumeMount{
			MountPath: mountPath,
			Name:      name,
			ReadOnly:  readOnly,
			SubPath:   "",
		}
		*volumeMounts = append(*volumeMounts, vm)
	}
	if VolumeType != model.MemoryFSVolumeType {
		vo := v1.Volume{
			Name: name,
		}
		vo.HostPath = &v1.HostPathVolumeSource{
			Path: hostPath,
		}
		*volumes = append(*volumes, vo)
	} else {
		vo := v1.Volume{Name: name}
		vo.EmptyDir = &v1.EmptyDirVolumeSource{
			Medium: v1.StorageMediumMemory,
		}
		*volumes = append(*volumes, vo)
	}
	//TODO:handle other volume type
}

//createEnv create app env
func (p *PodTemplateSpecBuild) createEnv() (*[]v1.EnvVar, error) {
	var envs []v1.EnvVar
	//set app history env
	if p.service.ContainerEnv != "" {
		vs := strings.Split(p.service.ContainerEnv, ",")
		for _, s := range vs {
			kv := strings.Split(s, "=")
			if len(kv) == 2 {
				envs = append(envs, v1.EnvVar{
					Name:  kv[0],
					Value: kv[1],
				})
			}
		}
	}
	//set default env
	envs = append(envs, v1.EnvVar{Name: "TENANT_ID", Value: p.service.TenantID})
	envs = append(envs, v1.EnvVar{Name: "SERVICE_ID", Value: p.service.ServiceID})
	envs = append(envs, v1.EnvVar{Name: "SERVICE_VERSION", Value: p.service.ServiceVersion})
	envs = append(envs, v1.EnvVar{Name: "MEMORY_SIZE", Value: p.getMemoryType()})
	envs = append(envs, v1.EnvVar{Name: "SERVICE_NAME", Value: p.service.ServiceAlias})
	envs = append(envs, v1.EnvVar{Name: "SERVICE_EXTEND_METHOD", Value: p.service.ExtendMethod})
	envs = append(envs, v1.EnvVar{Name: "SERVICE_POD_NUM", Value: fmt.Sprintf("%d", p.service.Replicas)})
	envs = append(envs, v1.EnvVar{Name: "EVENT_ID", Value: p.eventID})

	var envsAll []*model.TenantServiceEnvVar
	//set relation app outer env
	relations, err := p.dbmanager.TenantServiceRelationDao().GetTenantServiceRelations(p.serviceID)
	if err != nil {
		return nil, err
	}
	if relations != nil && len(relations) > 0 {
		var relationIDs []string
		for _, r := range relations {
			relationIDs = append(relationIDs, r.DependServiceID)
		}
		if len(relationIDs) > 0 {
			es, err := p.dbmanager.TenantServiceEnvVarDao().GetDependServiceEnvs(relationIDs, []string{"outer", "both"})
			if err != nil {
				return nil, err
			}
			if es != nil {
				envsAll = append(envsAll, es...)
			}
			serviceAliass, err := p.dbmanager.TenantServiceDao().GetServiceAliasByIDs(relationIDs)
			if err != nil {
				return nil, err
			}
			var Depend string
			for _, sa := range serviceAliass {
				if Depend != "" {
					Depend += ","
				}
				Depend += fmt.Sprintf("%s:%s", sa.ServiceAlias, sa.ServiceID)
			}
			envs = append(envs, v1.EnvVar{Name: "DEPEND_SERVICE", Value: Depend})
			p.needProxy = true
		}
	}

	//set app relation env
	relations, err = p.dbmanager.TenantServiceRelationDao().GetTenantServiceRelationsByDependServiceID(p.serviceID)
	if err != nil {
		return nil, err
	}
	if relations != nil && len(relations) > 0 {
		var relationIDs []string
		for _, r := range relations {
			relationIDs = append(relationIDs, r.ServiceID)
		}
		if len(relationIDs) > 0 {
			serviceAliass, err := p.dbmanager.TenantServiceDao().GetServiceAliasByIDs(relationIDs)
			if err != nil {
				return nil, err
			}
			var Depend string
			for _, sa := range serviceAliass {
				if Depend != "" {
					Depend += ","
				}
				Depend += fmt.Sprintf("%s:%s", sa.ServiceAlias, sa.ServiceID)
			}
			envs = append(envs, v1.EnvVar{Name: "REVERSE_DEPEND_SERVICE", Value: Depend})
		}
	}
	//set app port and net env
	ports, err := p.dbmanager.TenantServicesPortDao().GetPortsByServiceID(p.serviceID)
	if err != nil {
		return nil, err
	}
	if ports != nil && len(ports) > 0 {
		var portStr string
		for i, port := range ports {
			if i == 0 {
				envs = append(envs, v1.EnvVar{Name: "PORT", Value: strconv.Itoa(ports[0].ContainerPort)})
				envs = append(envs, v1.EnvVar{Name: "PROTOCOL", Value: ports[0].Protocol})
			}
			if portStr != "" {
				portStr += ":"
			}
			portStr += fmt.Sprintf("%d", port.ContainerPort)
			if port.IsOuterService && (port.Protocol == "http" || port.Protocol == "https") {
				envs = append(envs, v1.EnvVar{Name: "DEFAULT_DOMAIN", Value: p.service.Autodomain(p.tenant.Name, port.ContainerPort)})
			}
		}
		envs = append(envs, v1.EnvVar{Name: "MONITOR_PORT", Value: portStr})
	}
	//set net mode env by get from system
	envs = append(envs, v1.EnvVar{Name: "CUR_NET", Value: os.Getenv("CUR_NET")})

	//set app custom envs
	es, err := p.dbmanager.TenantServiceEnvVarDao().GetServiceEnvs(p.serviceID, []string{"inner", "both", "outer"})
	if err != nil {
		return nil, err
	}
	if len(es) > 0 {
		envsAll = append(envsAll, es...)
	}

	for _, e := range envsAll {
		if e.AttrName == "HOSTNAME" {
			p.hostName = e.AttrValue
		}
		envs = append(envs, v1.EnvVar{Name: e.AttrName, Value: e.AttrValue})
	}
	return &envs, nil
}

func (p *PodTemplateSpecBuild) createPluginsContainer(volumeMounts []v1.VolumeMount, mainEnvs *[]v1.EnvVar) ([]v1.Container, []v1.Container, error) {
	var containers []v1.Container
	var initContainers []v1.Container
	if len(p.pluginsRelation) == 0 && !p.needProxy {
		return nil, containers, nil
	}
	netPlugin := false
	for _, pluginR := range p.pluginsRelation {
		//if plugin not enable,ignore it
		if pluginR.Switch == false {
			continue
		}
		versionInfo, err := p.dbmanager.TenantPluginBuildVersionDao().GetLastBuildVersionByVersionID(pluginR.PluginID, pluginR.VersionID)
		if err != nil {
			return nil, nil, err
		}
		envs, err := p.createPluginEnvs(pluginR.PluginID, mainEnvs, pluginR.VersionID)
		if err != nil {
			return nil, nil, err
		}
		args, err := p.createPluginArgs(versionInfo.ContainerCMD)
		if err != nil {
			return nil, nil, err
		}
		pc := v1.Container{
			Name:                   "plugin-" + pluginR.PluginID,
			Image:                  versionInfo.BuildLocalImage,
			Env:                    *envs,
			Resources:              p.createPluginResources(pluginR.ContainerMemory, pluginR.ContainerCPU),
			TerminationMessagePath: "",
			Args:                   args,
			VolumeMounts:           volumeMounts,
		}
		pluginModel, err := p.getPluginModel(pluginR.PluginID)
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
	if p.needProxy && !netPlugin {
		c2 := v1.Container{
			Name: "adapter-" + p.serviceID[len(p.serviceID)-20:],
			VolumeMounts: []v1.VolumeMount{v1.VolumeMount{
				MountPath: "/etc/kubernetes",
				Name:      "kube-config",
				ReadOnly:  true,
			}},
			TerminationMessagePath: "",
			Env:                    *mainEnvs,
			Image:                  "goodrain.me/adapter",
			Resources:              p.createAdapterResources(50, 20),
		}
		containers = append(containers, c2)
	}
	return initContainers, containers, nil
}

func (p *PodTemplateSpecBuild) getPluginModel(pluginID string) (string, error) {
	plugin, err := p.dbmanager.TenantPluginDao().GetPluginByID(pluginID, p.tenant.UUID)
	if err != nil {
		return "", err
	}
	return plugin.PluginModel, nil
}

func (p *PodTemplateSpecBuild) createPluginArgs(cmd string) ([]string, error) {
	if cmd == "" {
		return nil, nil
	}
	return strings.Split(cmd, " "), nil
}

//container envs
func (p *PodTemplateSpecBuild) createPluginEnvs(pluginID string, mainEnvs *[]v1.EnvVar, versionID string) (*[]v1.EnvVar, error) {
	versionEnvs, err := p.dbmanager.TenantPluginVersionENVDao().GetVersionEnvByServiceID(p.serviceID, pluginID)
	if err != nil {
		return nil, err
	}
	var envs []v1.EnvVar
	for _, e := range versionEnvs {
		envs = append(envs, v1.EnvVar{Name: e.EnvName, Value: e.EnvValue})
	}
	for _, pluginRelation := range p.pluginsRelation {
		if strings.Contains(pluginRelation.PluginModel, "net-plugin") {
			envs = append(envs, v1.EnvVar{Name: "PLUGIN_MOEL", Value: pluginRelation.PluginModel})
		}
	}
	discoverURL := fmt.Sprintf(
		"%s/v1/resources/%s/%s/%s",
		p.NodeAPI,
		p.tenant.UUID,
		p.service.ServiceAlias,
		pluginID)
	envs = append(envs, v1.EnvVar{Name: "DISCOVER_URL", Value: discoverURL})
	envs = append(envs, v1.EnvVar{Name: "PLUGIN_ID", Value: pluginID})
	for _, e := range *mainEnvs {
		envs = append(envs, e)
	}
	//TODO: 在哪些情况下需要注入主容器的环境变量
	logrus.Debugf("plugin env is %v", envs)
	return &envs, nil
}

func (p *PodTemplateSpecBuild) sortPlugins() ([]string, error) {
	var pid []string
	var mid []int
	//one app could have one plugin of same mode
	for _, plugin := range p.pluginsRelation {
		pi, err := p.dbmanager.TenantPluginDao().GetPluginByID(plugin.PluginID, p.tenant.UUID)
		if err != nil {
			return nil, err
		}
		pid = append(pid, plugin.PluginID)
		mid = append(mid, p.pluginWeight(pi.PluginModel))
	}
	for i := 0; i < len(p.pluginsRelation); i++ {
		for j := i + 1; j < len(p.pluginsRelation); j++ {
			if mid[i] < mid[j] {
				tmpM := mid[i]
				mid[i] = mid[j]
				mid[j] = tmpM
				tmpP := pid[i]
				pid[i] = pid[j]
				pid[j] = tmpP
			}
		}
	}
	return pid, nil
}

func (p *PodTemplateSpecBuild) pluginWeight(pluginModel string) int {
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

var memoryLabels = map[int]string{
	128:   "micro",
	256:   "small",
	512:   "medium",
	1024:  "large",
	2048:  "2xlarge",
	4096:  "4xlarge",
	8192:  "8xlarge",
	16384: "16xlarge",
	32768: "32xlarge",
	65536: "64xlarge",
}

func (p *PodTemplateSpecBuild) getMemoryType() string {
	memorySize := p.service.ContainerMemory
	memoryType := "small"
	if v, ok := memoryLabels[memorySize]; ok {
		memoryType = v
	}
	return memoryType
}

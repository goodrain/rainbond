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
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

//TenantServiceVersion service deploy version conv. define pod spec
func TenantServiceVersion(as *v1.AppService, dbmanager db.Manager) error {
	version, err := dbmanager.VersionInfoDao().GetVersionByDeployVersion(as.DeployVersion, as.ServiceID)
	if err != nil {
		return fmt.Errorf("get service deploy version failure %s", err.Error())
	}
	dv, err := createVolumes(as, version, dbmanager)
	if err != nil {
		return fmt.Errorf("create volume in pod template error :%s", err.Error())
	}
	container, err := getMainContainer(as, version, dv, dbmanager)
	if err != nil {
		return fmt.Errorf("conv service main container failure %s", err.Error())
	}
	//need service mesh sidecar, volume kubeconfig
	if as.NeedProxy {
		dv.SetVolume(dbmodel.ShareFileVolumeType, "kube-config", "/etc/kubernetes", "/grdata/kubernetes", corev1.HostPathDirectoryOrCreate, true)
	}
	podtmpSpec := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: as.GetCommonLabels(map[string]string{
				"name":    as.ServiceAlias,
				"version": as.DeployVersion,
			}),
			Annotations: createPodAnnotations(as),
			Name:        as.ServiceID + "-pod-spec",
		},
		Spec: corev1.PodSpec{
			Volumes:      dv.GetVolumes(),
			Containers:   []corev1.Container{*container},
			NodeSelector: createNodeSelector(as, dbmanager),
			Affinity:     createAffinity(as, dbmanager),
			Hostname: func() string {
				if nodeID, ok := as.ExtensionSet["hostname"]; ok {
					return nodeID
				}
				return ""
			}(),
			NodeName: func() string {
				if nodeID, ok := as.ExtensionSet["selectnode"]; ok {
					return nodeID
				}
				return ""
			}(),
			HostNetwork: func() bool {
				if _, ok := as.ExtensionSet["hostnetwork"]; ok {
					return true
				}
				return false
			}(),
			SchedulerName: func() string {
				if name, ok := as.ExtensionSet["shcedulername"]; ok {
					return name
				}
				return ""
			}(),
		},
	}
	//set to deployment or statefulset
	as.SetPodTemplate(podtmpSpec)
	return nil
}

func getMainContainer(as *v1.AppService, version *dbmodel.VersionInfo, dv *volumeDefine, dbmanager db.Manager) (*corev1.Container, error) {
	envs, err := createEnv(as, dbmanager)
	if err != nil {
		return nil, fmt.Errorf("conv service envs failure %s", err.Error())
	}
	args := createArgs(version, *envs)
	resources := createResources(as)
	ports := createPorts(as, dbmanager)
	imagename := version.ImageName
	if imagename == "" {
		if version.DeliveredType == "slug" {
			imagename = builder.RUNNERIMAGENAME
		} else {
			imagename = version.DeliveredPath
		}
	}
	return &corev1.Container{
		Name:           as.ServiceID,
		Image:          imagename,
		Args:           args,
		Ports:          ports,
		Env:            *envs,
		VolumeMounts:   dv.GetVolumeMounts(),
		LivenessProbe:  createProbe(as, dbmanager, "liveness"),
		ReadinessProbe: createProbe(as, dbmanager, "readiness"),
		Resources:      resources,
	}, nil
}

func getenv(key string, envs []corev1.EnvVar) string {
	for _, env := range envs {
		if env.Name == key {
			return env.Value
		}
	}
	return ""
}

func createArgs(version *dbmodel.VersionInfo, envs []corev1.EnvVar) (args []string) {
	if version.Cmd == "" {
		return
	}
	configs := make(map[string]string, len(envs))
	for _, env := range envs {
		configs[env.Name] = env.Value
	}
	cmd := util.ParseVariable(version.Cmd, configs)
	args = strings.Split(cmd, " ")
	args = util.RemoveSpaces(args)
	return args
}

//createEnv create service container env
func createEnv(as *v1.AppService, dbmanager db.Manager) (*[]corev1.EnvVar, error) {
	var envs []corev1.EnvVar
	var envsAll []*dbmodel.TenantServiceEnvVar
	//set logger env
	//todo: user define and set logger config
	envs = append(envs, corev1.EnvVar{
		Name:  "LOGGER_DRIVER_NAME",
		Value: "streamlog",
	})

	//set relation app outer env
	relations, err := dbmanager.TenantServiceRelationDao().GetTenantServiceRelations(as.ServiceID)
	if err != nil {
		return nil, err
	}
	if relations != nil && len(relations) > 0 {
		var relationIDs []string
		for _, r := range relations {
			relationIDs = append(relationIDs, r.DependServiceID)
		}
		//set service all dependces ids
		as.Dependces = relationIDs
		if len(relationIDs) > 0 {
			es, err := dbmanager.TenantServiceEnvVarDao().GetDependServiceEnvs(relationIDs, []string{"outer", "both"})
			if err != nil {
				return nil, err
			}
			if es != nil {
				envsAll = append(envsAll, es...)
			}
			serviceAliass, err := dbmanager.TenantServiceDao().GetServiceAliasByIDs(relationIDs)
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
			envs = append(envs, corev1.EnvVar{Name: "DEPEND_SERVICE", Value: Depend})
			as.NeedProxy = true
		}
	}
	//set app relation env
	relations, err = dbmanager.TenantServiceRelationDao().GetTenantServiceRelationsByDependServiceID(as.ServiceID)
	if err != nil {
		return nil, err
	}
	if relations != nil && len(relations) > 0 {
		var relationIDs []string
		for _, r := range relations {
			relationIDs = append(relationIDs, r.ServiceID)
		}
		if len(relationIDs) > 0 {
			serviceAliass, err := dbmanager.TenantServiceDao().GetServiceAliasByIDs(relationIDs)
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
			envs = append(envs, corev1.EnvVar{Name: "REVERSE_DEPEND_SERVICE", Value: Depend})
		}
	}
	//set app port and net env
	ports, err := dbmanager.TenantServicesPortDao().GetPortsByServiceID(as.ServiceID)
	if err != nil {
		return nil, err
	}
	if ports != nil && len(ports) > 0 {
		var portStr string
		for i, port := range ports {
			if i == 0 {
				envs = append(envs, corev1.EnvVar{Name: "PORT", Value: strconv.Itoa(ports[0].ContainerPort)})
				envs = append(envs, corev1.EnvVar{Name: "PROTOCOL", Value: ports[0].Protocol})
			}
			if portStr != "" {
				portStr += ":"
			}
			portStr += fmt.Sprintf("%d", port.ContainerPort)
			if port.IsOuterService && (port.Protocol == "http" || port.Protocol == "https") {
				envs = append(envs, corev1.EnvVar{Name: "DEFAULT_DOMAIN", Value: createDefaultDomain(as.TenantName, as.ServiceAlias, port.ContainerPort)})
			}
		}
		envs = append(envs, corev1.EnvVar{Name: "MONITOR_PORT", Value: portStr})
	}
	//set net mode env by get from system
	envs = append(envs, corev1.EnvVar{Name: "CUR_NET", Value: os.Getenv("CUR_NET")})
	//set app custom envs
	es, err := dbmanager.TenantServiceEnvVarDao().GetServiceEnvs(as.ServiceID, []string{"inner", "both", "outer"})
	if err != nil {
		return nil, err
	}
	if len(es) > 0 {
		envsAll = append(envsAll, es...)
	}
	for _, e := range envsAll {
		envs = append(envs, corev1.EnvVar{Name: strings.TrimSpace(e.AttrName), Value: e.AttrValue})
		if strings.HasSuffix(e.AttrName, "ES_") {
			as.ExtensionSet[strings.ToLower(e.AttrName[3:])] = e.AttrValue
		}
	}
	svc, err := dbmanager.TenantServiceDao().GetServiceByID(as.ServiceID)
	if err != nil {
		return nil, err
	}
	//set default env
	envs = append(envs, corev1.EnvVar{Name: "TENANT_ID", Value: as.TenantID})
	envs = append(envs, corev1.EnvVar{Name: "SERVICE_ID", Value: as.ServiceID})
	envs = append(envs, corev1.EnvVar{Name: "MEMORY_SIZE", Value: getMemoryType(as.ContainerMemory)})
	envs = append(envs, corev1.EnvVar{Name: "SERVICE_NAME", Value: as.ServiceAlias})
	envs = append(envs, corev1.EnvVar{Name: "SERVICE_EXTEND_METHOD", Value: svc.ExtendMethod})
	envs = append(envs, corev1.EnvVar{Name: "SERVICE_POD_NUM", Value: strconv.Itoa(as.Replicas)})
	envs = append(envs, corev1.EnvVar{Name: "HOST_IP", ValueFrom: &corev1.EnvVarSource{
		FieldRef: &corev1.ObjectFieldSelector{
			FieldPath: "status.hostIP",
		},
	}})
	envs = append(envs, corev1.EnvVar{Name: "POD_IP", ValueFrom: &corev1.EnvVarSource{
		FieldRef: &corev1.ObjectFieldSelector{
			FieldPath: "status.podIP",
		},
	}})
	var config = make(map[string]string, len(envs))
	for _, env := range envs {
		config[env.Name] = env.Value
	}
	for i, env := range envs {
		envs[i].Value = util.ParseVariable(env.Value, config)
	}
	return &envs, nil
}

func getMemoryType(memorySize int) string {
	memoryType := "small"
	if v, ok := memoryLabels[memorySize]; ok {
		memoryType = v
	}
	return memoryType
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

func createVolumes(as *v1.AppService, version *dbmodel.VersionInfo, dbmanager db.Manager) (*volumeDefine, error) {
	logrus.Infof("begin creating volumes.")
	var vd = &volumeDefine{
		as: as,
	}
	vs, err := dbmanager.TenantServiceVolumeDao().GetTenantServiceVolumesByServiceID(version.ServiceID)
	if err != nil {
		return nil, err
	}

	// environment variables
	configs := make(map[string]string)
	envs, err := createEnv(as, dbmanager)
	if err != nil {
		logrus.Warningf("error creating environment variables: %v", err)
	} else {
		for _, env := range *envs {
			configs[env.Name] = env.Value
		}
	}

	if vs != nil && len(vs) > 0 {
		for i := range vs {
			v := vs[i]
			if v.VolumeType == dbmodel.ShareFileVolumeType.String() {
				err := util.CheckAndCreateDir(v.HostPath)
				if err != nil {
					return nil, fmt.Errorf("create host path %s error,%s", v.HostPath, err.Error())
				}
				os.Chmod(v.HostPath, 0777)
			}
			// create a configMap which will be mounted as a volume
			var cmap *corev1.ConfigMap
			if v.VolumeType == dbmodel.ConfigFileVolumeType.String() {
				cf, err := dbmanager.TenantServiceConfigFileDao().GetByVolumeName(v.VolumeName)
				if err != nil {
					logrus.Errorf("error getting config file by volume name(%s): %v", v.VolumeName, err)
					return nil, fmt.Errorf("error getting config file by volume name(%s): %v", v.VolumeName, err)
				}
				name := fmt.Sprintf("manual%s%s", as.ServiceID, v.VolumePath)
				name = strings.Replace(name, "/", "slash", -1)
				name = strings.Replace(name, ".", "dot", -1)
				cmap = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: as.TenantID,
						Labels:    as.GetCommonLabels(),
					},
					Data: make(map[string]string),
				}
				cmap.Data[path.Base(v.VolumePath)] = util.ParseVariable(cf.FileContent, configs)
				as.SetConfigMap(cmap)
				vd.SetVolumeCMap(cmap, path.Base(v.VolumePath), v.VolumePath, v.IsReadOnly)
				continue // pass codes below
			}
			if as.GetStatefulSet() != nil {
				vd.SetPV(dbmodel.VolumeType(v.VolumeType), fmt.Sprintf("manual%d", v.ID), v.VolumePath, v.IsReadOnly)
			} else {
				hostPath := v.HostPath
				if as.IsWindowsService {
					hostPath = RewriteHostPathInWindows(hostPath)
				}
				vd.SetVolume(dbmodel.VolumeType(v.VolumeType), fmt.Sprintf("manual%d", v.ID), v.VolumePath, hostPath, corev1.HostPathDirectoryOrCreate, v.IsReadOnly)
			}
		}
	}
	//handle Shared storage
	logrus.Infof("begin handling Shared storage")
	tsmr, err := dbmanager.TenantServiceMountRelationDao().GetTenantServiceMountRelationsByService(version.ServiceID)
	if err != nil {
		return nil, err
	}
	if vs != nil && len(tsmr) > 0 {
		for i := range tsmr {
			t := tsmr[i]
			switch t.VolumeType {
			case dbmodel.ShareFileVolumeType.String():
				err := util.CheckAndCreateDir(t.HostPath)
				if err != nil {
					return nil, fmt.Errorf("create host path %s error,%s", t.HostPath, err.Error())
				}
				hostPath := t.HostPath
				if as.IsWindowsService {
					hostPath = RewriteHostPathInWindows(hostPath)
				}
				vd.SetVolume(dbmodel.ShareFileVolumeType, fmt.Sprintf("mnt%d", t.ID), t.VolumePath, hostPath, corev1.HostPathDirectoryOrCreate, false)
			case dbmodel.ConfigFileVolumeType.String():
				tsv, err := dbmanager.TenantServiceVolumeDao().GetVolumeByServiceIDAndName(t.DependServiceID, t.VolumeName)
				if err != nil {
					return nil, fmt.Errorf("error getting TenantServiceVolume according to serviceID(%s) and volumeName(%s): %v",
						t.DependServiceID, t.VolumeName, err)
				}
				cf, err := dbmanager.TenantServiceConfigFileDao().GetByVolumeName(t.VolumeName)
				if err != nil {
					return nil, fmt.Errorf("error getting TenantServiceConfigFileDao according to volumeName(%s): %v", t.VolumeName, err)
				}

				name := fmt.Sprintf("manual%s%s", as.ServiceID, t.VolumePath)
				name = strings.Replace(name, "/", "slash", -1)
				name = strings.Replace(name, ".", "dot", -1)
				cmap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: as.TenantID,
						Labels:    as.GetCommonLabels(),
					},
					Data: make(map[string]string),
				}
				cmap.Data[path.Base(t.VolumePath)] = util.ParseVariable(cf.FileContent, configs)
				as.SetConfigMap(cmap)

				vd.SetVolumeCMap(cmap, path.Base(t.VolumePath), t.VolumePath, tsv.IsReadOnly)
			}
		}
	}
	//handle slug file volume
	if version.DeliveredType == "slug" {
		//slug host path already is windows style
		vd.SetVolume(dbmodel.ShareFileVolumeType, "slug", "/tmp/slug/slug.tgz", version.DeliveredPath, corev1.HostPathFile, true)
	}
	return vd, nil
}

//RewriteHostPathInWindows rewrite host path
func RewriteHostPathInWindows(hostPath string) string {
	localPath := os.Getenv("LOCAL_DATA_PATH")
	sharePath := os.Getenv("SHARE_DATA_PATH")
	if localPath == "" {
		localPath = "/grlocaldata"
	}
	if sharePath == "" {
		sharePath = "/grdata"
	}
	hostPath = strings.Replace(hostPath, "/grdata", `z:`, 1)
	hostPath = strings.Replace(hostPath, "/", `\`, -1)
	return hostPath
}

//RewriteContainerPathInWindows mount path in windows
func RewriteContainerPathInWindows(mountPath string) string {
	if mountPath == "" {
		return ""
	}
	if mountPath[0] == '/' {
		mountPath = `c:\` + mountPath[1:]
	}
	mountPath = strings.Replace(mountPath, "/", `\`, -1)
	return mountPath
}

type volumeDefine struct {
	as           *v1.AppService
	volumeMounts []corev1.VolumeMount
	volumes      []corev1.Volume
}

func (v *volumeDefine) GetVolumes() []corev1.Volume {
	return v.volumes
}

func (v *volumeDefine) GetVolumeMounts() []corev1.VolumeMount {
	return v.volumeMounts
}

func (v *volumeDefine) SetPV(VolumeType dbmodel.VolumeType, name, mountPath string, readOnly bool) {
	logrus.Info("Set persistence volume for statefuleset.")
	switch VolumeType {
	case dbmodel.ShareFileVolumeType:
		logrus.Infof("VolumeType is share-file")
		if statefulset := v.as.GetStatefulSet(); statefulset != nil {
			//do not limit
			resourceStorage, _ := resource.ParseQuantity("500Gi")
			statefulset.Spec.VolumeClaimTemplates = append(
				statefulset.Spec.VolumeClaimTemplates,
				corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: name,
						Labels: v.as.GetCommonLabels(map[string]string{
							"tenant_id": v.as.TenantID,
						}),
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
						StorageClassName: &v1.RainbondStatefuleShareStorageClass,
						Resources: corev1.ResourceRequirements{
							Requests: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceStorage: resourceStorage,
							},
						},
					},
				},
			)
			v.volumeMounts = append(v.volumeMounts, corev1.VolumeMount{
				Name:      name,
				MountPath: mountPath,
				ReadOnly:  readOnly,
			})
		}
	case dbmodel.LocalVolumeType:
		logrus.Infof("VolumeType is local")
		if statefulset := v.as.GetStatefulSet(); statefulset != nil {
			//do not limit
			resourceStorage, _ := resource.ParseQuantity("500Gi")
			statefulset.Spec.VolumeClaimTemplates = append(
				statefulset.Spec.VolumeClaimTemplates,
				corev1.PersistentVolumeClaim{

					ObjectMeta: metav1.ObjectMeta{
						Name: name,
						Annotations: map[string]string{
							client.LabelOS: func() string {
								if v.as.IsWindowsService {
									return "windows"
								}
								return "linux"
							}(),
						},
						Labels: v.as.GetCommonLabels(map[string]string{
							"tenant_id": v.as.TenantID,
						}),
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
						StorageClassName: &v1.RainbondStatefuleLocalStorageClass,
						Resources: corev1.ResourceRequirements{
							Requests: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceStorage: resourceStorage,
							},
						},
					},
				},
			)
			v.volumeMounts = append(v.volumeMounts, corev1.VolumeMount{
				Name:      name,
				MountPath: mountPath,
				ReadOnly:  readOnly,
			})
		}
	}
}

func (v *volumeDefine) SetVolume(VolumeType dbmodel.VolumeType, name, mountPath, hostPath string, hostPathType corev1.HostPathType, readOnly bool) {
	logrus.Info("Set volume for deployment.")
	for _, m := range v.volumeMounts {
		if m.MountPath == mountPath {
			return
		}
	}
	switch VolumeType {
	case dbmodel.MemoryFSVolumeType:
		logrus.Infof("VolumeType is memoryfs")
		vo := corev1.Volume{Name: name}
		vo.EmptyDir = &corev1.EmptyDirVolumeSource{
			Medium: corev1.StorageMediumMemory,
		}
		v.volumes = append(v.volumes, vo)
		if mountPath != "" {
			vm := corev1.VolumeMount{
				MountPath: mountPath,
				Name:      name,
				ReadOnly:  readOnly,
				SubPath:   "",
			}
			v.volumeMounts = append(v.volumeMounts, vm)
		}
	case dbmodel.ShareFileVolumeType:
		logrus.Infof("VolumeType is share-file")
		if hostPath != "" {
			vo := corev1.Volume{
				Name: name,
			}
			vo.HostPath = &corev1.HostPathVolumeSource{
				Path: hostPath,
				Type: &hostPathType,
			}
			v.volumes = append(v.volumes, vo)
			if mountPath != "" {
				vm := corev1.VolumeMount{
					MountPath: mountPath,
					Name:      name,
					ReadOnly:  readOnly,
					SubPath:   "",
				}
				v.volumeMounts = append(v.volumeMounts, vm)
			}
		}
	case dbmodel.LocalVolumeType:
		//no support
		return
	}
}

// SetVolumeCMap sets volumes and volumeMounts. The type of volumes is configMap.
func (v *volumeDefine) SetVolumeCMap(cmap *corev1.ConfigMap, k, p string, isReadOnly bool) {
	vm := corev1.VolumeMount{
		MountPath: p,
		Name:      cmap.Name,
		ReadOnly:  isReadOnly,
		SubPath:   path.Base(p),
	}
	v.volumeMounts = append(v.volumeMounts, vm)

	vo := corev1.Volume{
		Name: cmap.Name,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cmap.Name,
				},
				Items: []corev1.KeyToPath{
					{
						Key:  k,
						Path: path.Base(p), // subpath
					},
				},
			},
		},
	}
	v.volumes = append(v.volumes, vo)
}

func createResources(as *v1.AppService) corev1.ResourceRequirements {
	var cpuRequest, cpuLimit int64
	memory := as.ContainerMemory
	if memory < 512 {
		cpuRequest, cpuLimit = int64(memory)/128*30, int64(memory)/128*80
	} else if memory <= 1024 {
		cpuRequest, cpuLimit = int64(memory)/128*30, int64(memory)/128*160
	} else {
		cpuRequest, cpuLimit = int64(memory)/128*30, ((int64(memory)-1024)/1024*500 + 1280)
	}
	limits := corev1.ResourceList{}
	limits[corev1.ResourceCPU] = *resource.NewMilliQuantity(
		cpuLimit,
		resource.DecimalSI)
	limits[corev1.ResourceMemory] = *resource.NewQuantity(
		int64(as.ContainerMemory*1024*1024),
		resource.BinarySI)
	request := corev1.ResourceList{}
	request[corev1.ResourceCPU] = *resource.NewMilliQuantity(
		cpuRequest,
		resource.DecimalSI)
	request[corev1.ResourceMemory] = *resource.NewQuantity(
		int64(as.ContainerMemory*1024*1024),
		resource.BinarySI)
	return corev1.ResourceRequirements{
		Limits:   limits,
		Requests: request,
	}
}

func checkUpstreamPluginRelation(serviceID string, dbmanager db.Manager) (bool, error) {
	return dbmanager.TenantServicePluginRelationDao().CheckSomeModelPluginByServiceID(
		serviceID,
		dbmodel.UpNetPlugin)
}
func createUpstreamPluginMappingPort(
	ports []*dbmodel.TenantServicesPort,
	pluginPorts []*dbmodel.TenantServicesStreamPluginPort,
) (
	[]*dbmodel.TenantServicesPort,
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
func createPorts(as *v1.AppService, dbmanager db.Manager) (ports []corev1.ContainerPort) {
	ps, err := dbmanager.TenantServicesPortDao().GetPortsByServiceID(as.ServiceID)
	if err == nil && ps != nil && len(ps) > 0 {
		crt, err := checkUpstreamPluginRelation(as.ServiceID, dbmanager)
		if err != nil {
			logrus.Warningf("error getting service upstream plugin relation, %s", err.Error())
			return
		}
		if crt {
			pluginPorts, err := dbmanager.TenantServicesStreamPluginPortDao().GetPluginMappingPorts(
				as.ServiceID,
				dbmodel.UpNetPlugin,
			)
			if err != nil {
				logrus.Warningf("find upstream plugin mapping port error, %s", err.Error())
				return
			}
			ps, err = createUpstreamPluginMappingPort(ps, pluginPorts)
		}
		for i := range ps {
			p := ps[i]
			ports = append(ports, corev1.ContainerPort{
				ContainerPort: int32(p.ContainerPort),
				// Must be UDP, TCP, or SCTP.
				Protocol: conversionPortProtocol(p.Protocol),
			})
		}
	}
	return
}

func createProbe(as *v1.AppService, dbmanager db.Manager, mode string) *corev1.Probe {
	probe, err := dbmanager.ServiceProbeDao().GetServiceUsedProbe(as.ServiceID, mode)
	if err == nil && probe != nil {
		if mode == "liveness" && probe.SuccessThreshold < 1 {
			probe.SuccessThreshold = 1
		}
		if mode == "readiness" && probe.FailureThreshold < 1 {
			probe.FailureThreshold = 3
		}
		p := &corev1.Probe{
			FailureThreshold:    int32(probe.FailureThreshold),
			SuccessThreshold:    int32(probe.SuccessThreshold),
			InitialDelaySeconds: int32(probe.InitialDelaySecond),
			TimeoutSeconds:      int32(probe.TimeoutSecond),
			PeriodSeconds:       int32(probe.PeriodSecond),
		}
		if probe.Scheme == "tcp" {
			tcp := &corev1.TCPSocketAction{
				Port: intstr.FromInt(probe.Port),
			}
			p.TCPSocket = tcp
			return p
		} else if probe.Scheme == "http" {
			action := corev1.HTTPGetAction{Path: probe.Path, Port: intstr.FromInt(probe.Port)}
			if probe.HTTPHeader != "" {
				hds := strings.Split(probe.HTTPHeader, ",")
				var headers []corev1.HTTPHeader
				for _, hd := range hds {
					kv := strings.Split(hd, "=")
					if len(kv) == 1 {
						header := corev1.HTTPHeader{
							Name:  kv[0],
							Value: "",
						}
						headers = append(headers, header)
					} else if len(kv) == 2 {
						header := corev1.HTTPHeader{
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
	//TODO:create default probe
	return nil
}

func createNodeSelector(as *v1.AppService, dbmanager db.Manager) map[string]string {
	selector := make(map[string]string)
	labels, err := dbmanager.TenantServiceLabelDao().GetTenantServiceNodeSelectorLabel(as.ServiceID)
	if err == nil && labels != nil && len(labels) > 0 {
		for _, l := range labels {
			if l.LabelValue == "windows" || l.LabelValue == "linux" {
				selector[client.LabelOS] = l.LabelValue
				continue
			}
			if strings.Contains(l.LabelValue, "=") {
				kv := strings.SplitN(l.LabelValue, "=", 1)
				selector[kv[0]] = kv[1]
			} else {
				selector["rainbond_node_lable_"+l.LabelValue] = "true"
			}
		}
	}
	return selector
}
func createAffinity(as *v1.AppService, dbmanager db.Manager) *corev1.Affinity {
	var affinity corev1.Affinity
	nsr := make([]corev1.NodeSelectorRequirement, 0)
	podAffinity := make([]corev1.PodAffinityTerm, 0)
	podAntAffinity := make([]corev1.PodAffinityTerm, 0)
	osWindowsSelect := false
	labels, err := dbmanager.TenantServiceLabelDao().GetTenantServiceAffinityLabel(as.ServiceID)
	if err == nil && labels != nil && len(labels) > 0 {
		for _, l := range labels {
			if l.LabelKey == dbmodel.LabelKeyNodeSelector {
				if l.LabelValue == "windows" {
					osWindowsSelect = true
					continue
				}
			}
			if l.LabelKey == dbmodel.LabelKeyNodeAffinity {
				if l.LabelValue == "windows" {
					nsr = append(nsr, corev1.NodeSelectorRequirement{
						Key:      client.LabelOS,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{l.LabelValue},
					})
					osWindowsSelect = true
					continue
				}
				if strings.Contains(l.LabelValue, "=") {
					kv := strings.SplitN(l.LabelValue, "=", 1)
					nsr = append(nsr, corev1.NodeSelectorRequirement{
						Key:      kv[0],
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{kv[1]},
					})
				} else {
					nsr = append(nsr, corev1.NodeSelectorRequirement{
						Key:      "rainbond_node_lable_" + l.LabelValue,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"true"},
					})
				}
			}
			if l.LabelKey == dbmodel.LabelKeyServiceAffinity {
				podAffinity = append(podAffinity, corev1.PodAffinityTerm{
					LabelSelector: metav1.SetAsLabelSelector(map[string]string{
						"name": l.LabelValue,
					}),
					Namespaces: []string{as.TenantID},
				})
			}
			if l.LabelKey == dbmodel.LabelKeyServiceAntyAffinity {
				podAntAffinity = append(
					podAntAffinity, corev1.PodAffinityTerm{
						LabelSelector: metav1.SetAsLabelSelector(map[string]string{
							"name": l.LabelValue,
						}),
						Namespaces: []string{as.TenantID},
					})
			}
		}
	}
	if !osWindowsSelect {
		nsr = append(nsr, corev1.NodeSelectorRequirement{
			Key:      client.LabelOS,
			Operator: corev1.NodeSelectorOpNotIn,
			Values:   []string{"windows"},
		})
	}
	if len(nsr) > 0 {
		affinity.NodeAffinity = &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					corev1.NodeSelectorTerm{MatchExpressions: nsr},
				},
			},
		}
	}
	if len(podAffinity) > 0 {
		affinity.PodAffinity = &corev1.PodAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: podAffinity,
		}
	}
	if len(podAntAffinity) > 0 {
		affinity.PodAntiAffinity = &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: podAntAffinity,
		}
	}
	return &affinity
}

func createPodAnnotations(as *v1.AppService) map[string]string {
	var annotations = make(map[string]string)
	if as.Replicas <= 1 {
		annotations["rainbond.com/tolerate-unready-endpoints"] = "true"
	}
	return annotations
}

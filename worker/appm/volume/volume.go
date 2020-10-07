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
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Volume volume function interface
type Volume interface {
	CreateVolume(define *Define) error       // use serviceVolume
	CreateDependVolume(define *Define) error // use serviceMountR
	setBaseInfo(as *v1.AppService, serviceVolume *model.TenantServiceVolume, serviceMountR *model.TenantServiceMountRelation, version *dbmodel.VersionInfo, dbmanager db.Manager)
}

// NewVolumeManager create volume
func NewVolumeManager(as *v1.AppService, serviceVolume *model.TenantServiceVolume, serviceMountR *model.TenantServiceMountRelation, version *dbmodel.VersionInfo, dbmanager db.Manager) Volume {
	var v Volume
	volumeType := ""
	if serviceVolume != nil {
		volumeType = serviceVolume.VolumeType
	}
	if serviceMountR != nil {
		volumeType = serviceMountR.VolumeType
	}
	if volumeType == "" {
		logrus.Warn("unknown volume Type, can't create volume")
		return nil
	}
	switch volumeType {
	case dbmodel.ShareFileVolumeType.String():
		v = new(ShareFileVolume)
	case dbmodel.ConfigFileVolumeType.String():
		v = new(ConfigFileVolume)
	case dbmodel.MemoryFSVolumeType.String():
		v = new(MemoryFSVolume)
	case dbmodel.LocalVolumeType.String():
		v = new(LocalVolume)
	default:
		logrus.Warnf("other volume type[%s]", volumeType)
		v = new(OtherVolume)
	}
	v.setBaseInfo(as, serviceVolume, serviceMountR, version, dbmanager)
	return v
}

// Base volume base
type Base struct {
	as        *v1.AppService
	svm       *model.TenantServiceVolume
	smr       *model.TenantServiceMountRelation
	version   *dbmodel.VersionInfo
	dbmanager db.Manager
}

func (b *Base) setBaseInfo(as *v1.AppService, serviceVolume *model.TenantServiceVolume, serviceMountR *model.TenantServiceMountRelation, version *dbmodel.VersionInfo, dbmanager db.Manager) {
	b.as = as
	b.svm = serviceVolume
	b.smr = serviceMountR
	b.version = version
	b.dbmanager = dbmanager
}

func prepare() {
	// TODO prepare volume info, create volume just create volume and return volumeMount, do not process anything else
}

func newVolumeClaim(name, volumePath, accessMode, storageClassName string, capacity int64, labels, annotations map[string]string) *corev1.PersistentVolumeClaim {
	logrus.Debugf("volume annotaion is %+v", annotations)
	if capacity == 0 {
		logrus.Warnf("claim[%s] capacity is 0, set 500G default", name)
		capacity = 500
	}
	resourceStorage, _ := resource.ParseQuantity(fmt.Sprintf("%dGi", capacity)) // 统一单位使用G
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
			Namespace:   "string",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{parseAccessMode(accessMode)},
			StorageClassName: &storageClassName,
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: resourceStorage,
				},
			},
		},
	}
}

/*
	RWO - ReadWriteOnce
	ROX - ReadOnlyMany
	RWX - ReadWriteMany
*/
func parseAccessMode(accessMode string) corev1.PersistentVolumeAccessMode {
	accessMode = strings.ToUpper(accessMode)
	switch accessMode {
	case "RWO":
		return corev1.ReadWriteOnce
	case "ROX":
		return corev1.ReadOnlyMany
	case "RWX":
		return corev1.ReadWriteMany
	default:
		return corev1.ReadWriteOnce
	}
}

// Define define volume
type Define struct {
	as           *v1.AppService
	volumeMounts []corev1.VolumeMount
	volumes      []corev1.Volume
}

// GetVolumes get define volumes
func (v *Define) GetVolumes() []corev1.Volume {
	return v.volumes
}

// GetVolumeMounts get define volume mounts
func (v *Define) GetVolumeMounts() []corev1.VolumeMount {
	return v.volumeMounts
}

// SetVolume define set volume
func (v *Define) SetVolume(VolumeType dbmodel.VolumeType, name, mountPath, hostPath string, hostPathType corev1.HostPathType, readOnly bool) {
	for _, m := range v.volumeMounts {
		if m.MountPath == mountPath {
			return
		}
	}
	switch VolumeType {
	case dbmodel.MemoryFSVolumeType:
		vo := corev1.Volume{Name: name}
		// V5.2 do not use memory as medium of emptyDir
		vo.EmptyDir = &corev1.EmptyDirVolumeSource{}
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
func (v *Define) SetVolumeCMap(cmap *corev1.ConfigMap, k, p string, isReadOnly bool) {
	var configFileMode int32 = 0777
	vm := corev1.VolumeMount{
		MountPath: p,
		Name:      cmap.Name,
		ReadOnly:  false,
		SubPath:   path.Base(p),
	}
	v.volumeMounts = append(v.volumeMounts, vm)
	var defaultMode int32 = 0777
	vo := corev1.Volume{
		Name: cmap.Name,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cmap.Name,
				},
				DefaultMode: &defaultMode,
				Items: []corev1.KeyToPath{
					corev1.KeyToPath{
						Key:  k,
						Path: path.Base(p), // subpath
						Mode: &configFileMode,
					},
				},
			},
		},
	}
	v.volumes = append(v.volumes, vo)
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
			envs = append(envs, corev1.EnvVar{Name: "DEPEND_SERVICE_COUNT", Value: strconv.Itoa(len(serviceAliass))})
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
		}
		menvs := convertRulesToEnvs(as, dbmanager, ports)
		if envs != nil && len(envs) > 0 {
			envs = append(envs, menvs...)
		}
		envs = append(envs, corev1.EnvVar{Name: "MONITOR_PORT", Value: portStr})
	}
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
		if strings.HasPrefix(e.AttrName, "ES_") {
			as.ExtensionSet[strings.ToLower(e.AttrName[3:])] = e.AttrValue
		}
	}
	//set default env
	envs = append(envs, corev1.EnvVar{Name: "TENANT_ID", Value: as.TenantID})
	envs = append(envs, corev1.EnvVar{Name: "SERVICE_ID", Value: as.ServiceID})
	envs = append(envs, corev1.EnvVar{Name: "MEMORY_SIZE", Value: getMemoryType(as.ContainerMemory)})
	envs = append(envs, corev1.EnvVar{Name: "SERVICE_NAME", Value: as.ServiceAlias})
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

func convertRulesToEnvs(as *v1.AppService, dbmanager db.Manager, ports []*dbmodel.TenantServicesPort) (re []corev1.EnvVar) {
	defDomain := fmt.Sprintf(".%s.%s.", as.ServiceAlias, as.TenantName)
	httpRules, _ := dbmanager.HTTPRuleDao().ListByServiceID(as.ServiceID)
	portDomainEnv := make(map[int][]corev1.EnvVar)
	portProtocolEnv := make(map[int][]corev1.EnvVar)
	for i := range httpRules {
		rule := httpRules[i]
		portDomainEnv[rule.ContainerPort] = append(portDomainEnv[rule.ContainerPort], corev1.EnvVar{
			Name:  fmt.Sprintf("DOMAIN_%d", rule.ContainerPort),
			Value: rule.Domain,
		})
		portProtocolEnv[rule.ContainerPort] = append(portProtocolEnv[rule.ContainerPort], corev1.EnvVar{
			Name: fmt.Sprintf("DOMAIN_PROTOCOL_%d", rule.ContainerPort),
			Value: func() string {
				if rule.CertificateID != "" {
					return "https"
				}
				return "http"
			}(),
		})
	}
	var portInts []int
	for _, port := range ports {
		if *port.IsOuterService {
			portInts = append(portInts, port.ContainerPort)
		}
	}
	sort.Ints(portInts)
	var gloalDomain, gloalDomainProcotol string
	var firstDomain, firstDomainProcotol string
	for _, p := range portInts {
		if len(portDomainEnv[p]) == 0 {
			continue
		}
		var portDomain, portDomainProcotol string
		for i, renv := range portDomainEnv[p] {
			//custom http rule
			if !strings.Contains(renv.Value, defDomain) {
				if gloalDomain == "" {
					gloalDomain = renv.Value
					gloalDomainProcotol = portProtocolEnv[p][i].Value
				}
				portDomain = renv.Value
				portDomainProcotol = portProtocolEnv[p][i].Value
				break
			}
			if firstDomain == "" {
				firstDomain = renv.Value
				firstDomainProcotol = portProtocolEnv[p][i].Value
			}
		}
		if portDomain == "" {
			portDomain = portDomainEnv[p][0].Value
			portDomainProcotol = portProtocolEnv[p][0].Value
		}
		re = append(re, corev1.EnvVar{
			Name:  fmt.Sprintf("DOMAIN_%d", p),
			Value: portDomain,
		})
		re = append(re, corev1.EnvVar{
			Name:  fmt.Sprintf("DOMAIN_PROTOCOL_%d", p),
			Value: portDomainProcotol,
		})
	}
	if gloalDomain == "" {
		gloalDomain = firstDomain
		gloalDomainProcotol = firstDomainProcotol
	}
	if gloalDomain != "" {
		re = append(re, corev1.EnvVar{
			Name:  "DOMAIN",
			Value: gloalDomain,
		})
		re = append(re, corev1.EnvVar{
			Name:  "DOMAIN_PROTOCOL",
			Value: gloalDomainProcotol,
		})
	}
	return
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

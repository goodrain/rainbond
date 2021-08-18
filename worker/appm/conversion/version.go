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
	"net"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/util/envutil"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/goodrain/rainbond/worker/appm/volume"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

//TenantServiceVersion service deploy version conv. define pod spec
func TenantServiceVersion(as *v1.AppService, dbmanager db.Manager) error {
	version, err := dbmanager.VersionInfoDao().GetVersionByDeployVersion(as.DeployVersion, as.ServiceID)
	if err != nil {
		return fmt.Errorf("get service deploy version %s failure %s", as.DeployVersion, err.Error())
	}
	envVarSecrets := as.GetEnvVarSecrets(true)
	logrus.Debugf("[getMainContainer] %d secrets as envs were found.", len(envVarSecrets))

	envs, err := createEnv(as, dbmanager, envVarSecrets)
	if err != nil {
		return fmt.Errorf("conv service envs failure %s", err.Error())
	}

	dv, err := createVolumes(as, version, envs, envVarSecrets, dbmanager)
	if err != nil {
		return fmt.Errorf("create volume in pod template error :%s", err.Error())
	}
	container, err := getMainContainer(as, version, dv, envs, envVarSecrets, dbmanager)
	if err != nil {
		return fmt.Errorf("conv service main container failure %s", err.Error())
	}
	//need service mesh sidecar, volume kubeconfig
	if as.NeedProxy {
		dv.SetVolume(dbmodel.ShareFileVolumeType, "kube-config", "/etc/kubernetes", "/grdata/kubernetes", corev1.HostPathDirectoryOrCreate, true)
	}
	nodeSelector := createNodeSelector(as, dbmanager)
	tolerations := createToleration(nodeSelector)
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
			ImagePullSecrets: setImagePullSecrets(),
			Volumes:          dv.GetVolumes(),
			Containers:       []corev1.Container{*container},
			NodeSelector:     nodeSelector,
			Tolerations:      tolerations,
			Affinity:         createAffinity(as, dbmanager),
			HostAliases:      createHostAliases(as),
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
	var terminationGracePeriodSeconds int64 = 10
	if as.GetDeployment() != nil {
		podtmpSpec.Spec.TerminationGracePeriodSeconds = &terminationGracePeriodSeconds
	}
	//set to deployment or statefulset
	as.SetPodTemplate(podtmpSpec)
	return nil
}

func getMainContainer(as *v1.AppService, version *dbmodel.VersionInfo, dv *volume.Define, envs []corev1.EnvVar, envVarSecrets []*corev1.Secret, dbmanager db.Manager) (*corev1.Container, error) {
	// secret as container environment variables
	var envFromSecrets []corev1.EnvFromSource
	for _, secret := range envVarSecrets {
		envFromSecrets = append(envFromSecrets, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secret.Name,
				},
			},
		})
	}
	args := createArgs(version, envs)
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

	c := &corev1.Container{
		Name:           as.ServiceID,
		Image:          imagename,
		Args:           args,
		Ports:          ports,
		Env:            envs,
		EnvFrom:        envFromSecrets,
		VolumeMounts:   dv.GetVolumeMounts(),
		LivenessProbe:  createProbe(as, dbmanager, "liveness"),
		ReadinessProbe: createProbe(as, dbmanager, "readiness"),
		Resources:      resources,
	}

	label, err := dbmanager.TenantServiceLabelDao().GetPrivilegedLabel(as.ServiceID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("get privileged label: %v", err)
	}
	if label != nil {
		logrus.Infof("service id: %s; enable privileged.", as.ServiceID)
		c.SecurityContext = &corev1.SecurityContext{Privileged: util.Bool(true)}
	}

	return c, nil
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
func createEnv(as *v1.AppService, dbmanager db.Manager, envVarSecrets []*corev1.Secret) ([]corev1.EnvVar, error) {
	var envs []corev1.EnvVar
	var envsAll []*dbmodel.TenantServiceEnvVar
	//set logger env
	//todo: user define and set logger config
	envs = append(envs, corev1.EnvVar{
		Name:  "LOGGER_DRIVER_NAME",
		Value: "streamlog",
	})

	bootSeqDepServiceIDs := as.ExtensionSet["boot_seq_dep_service_ids"]
	logrus.Infof("boot sequence dep service ids: %s", bootSeqDepServiceIDs)

	//set relation app outer env
	var relationIDs []string
	relations, err := dbmanager.TenantServiceRelationDao().GetTenantServiceRelations(as.ServiceID)
	if err != nil {
		return nil, err
	}
	if len(relations) > 0 {
		for _, r := range relations {
			relationIDs = append(relationIDs, r.DependServiceID)
		}
		//set service all dependces ids
		as.Dependces = relationIDs
		es, err := dbmanager.TenantServiceEnvVarDao().GetDependServiceEnvs(relationIDs, []string{"outer", "both"})
		if err != nil {
			return nil, err
		}
		if es != nil {
			envsAll = append(envsAll, es...)
		}

		serviceAliases, err := dbmanager.TenantServiceDao().GetServiceAliasByIDs(relationIDs)
		if err != nil {
			return nil, err
		}
		var Depend string
		var startupSequenceDependencies []string
		for _, sa := range serviceAliases {
			if Depend != "" {
				Depend += ","
			}
			Depend += fmt.Sprintf("%s:%s", sa.ServiceAlias, sa.ServiceID)

			if bootSeqDepServiceIDs != "" && strings.Contains(bootSeqDepServiceIDs, sa.ServiceID) {
				startupSequenceDependencies = append(startupSequenceDependencies, sa.ServiceAlias)
			}
		}
		envs = append(envs, corev1.EnvVar{Name: "DEPEND_SERVICE", Value: Depend})
		envs = append(envs, corev1.EnvVar{Name: "DEPEND_SERVICE_COUNT", Value: strconv.Itoa(len(serviceAliases))})
		envs = append(envs, corev1.EnvVar{Name: "STARTUP_SEQUENCE_DEPENDENCIES", Value: strings.Join(startupSequenceDependencies, ",")})

		if as.GovernanceMode == model.GovernanceModeBuildInServiceMesh {
			as.NeedProxy = true
		}
	}

	//set app relation env
	relations, err = dbmanager.TenantServiceRelationDao().GetTenantServiceRelationsByDependServiceID(as.ServiceID)
	if err != nil {
		return nil, err
	}
	if len(relations) > 0 {
		var relationIDs []string
		for _, r := range relations {
			relationIDs = append(relationIDs, r.ServiceID)
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
		envs = append(envs, corev1.EnvVar{Name: "REVERSE_DEPEND_SERVICE", Value: Depend})
	}

	//set app port and net env
	ports, err := dbmanager.TenantServicesPortDao().GetPortsByServiceID(as.ServiceID)
	if err != nil {
		return nil, err
	}
	if len(ports) > 0 {
		var portStr string
		var minPort int
		var protocol string
		for _, port := range ports {
			if minPort == 0 || minPort > port.ContainerPort {
				minPort = port.ContainerPort
				protocol = port.Protocol
			}
			if portStr != "" {
				portStr += ":"
			}
			portStr += fmt.Sprintf("%d", port.ContainerPort)
		}
		envs = append(envs, corev1.EnvVar{Name: "PORT", Value: strconv.Itoa(minPort)})
		envs = append(envs, corev1.EnvVar{Name: "PROTOCOL", Value: protocol})
		menvs := convertRulesToEnvs(as, dbmanager, ports)
		if len(envs) > 0 {
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
	}

	//set default env
	envs = append(envs, corev1.EnvVar{Name: "TENANT_ID", Value: as.TenantID})
	envs = append(envs, corev1.EnvVar{Name: "SERVICE_ID", Value: as.ServiceID})
	envs = append(envs, corev1.EnvVar{Name: "MEMORY_SIZE", Value: envutil.GetMemoryType(as.ContainerMemory)})
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
	for _, sec := range envVarSecrets {
		for k, v := range sec.Data {
			// The priority of component environment variable is higher than the one of the application.
			if val := config[k]; val == string(v) {
				continue
			}
			config[k] = string(v)
		}
	}
	// component env priority over the app configuration group
	for _, env := range envs {
		config[env.Name] = env.Value
	}

	for i, env := range envs {
		envs[i].Value = util.ParseVariable(env.Value, config)
	}
	for _, sec := range envVarSecrets {
		for i, data := range sec.Data {
			sec.Data[i] = []byte(util.ParseVariable(string(data), config))
		}
	}
	// set Extension set config item
	for k, v := range config {
		if strings.HasPrefix(k, "ES_") {
			as.ExtensionSet[strings.ToLower(k[3:])] = v
		}
	}
	return envs, nil
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

func createVolumes(as *v1.AppService, version *dbmodel.VersionInfo, envs []corev1.EnvVar, secrets []*corev1.Secret, dbmanager db.Manager) (*volume.Define, error) {
	var define = &volume.Define{}
	vs, err := dbmanager.TenantServiceVolumeDao().GetTenantServiceVolumesByServiceID(version.ServiceID)
	if err != nil {
		return nil, err
	}
	for _, v := range vs {
		vol := volume.NewVolumeManager(as, v, nil, version, envs, secrets, dbmanager)
		if vol != nil {
			if err = vol.CreateVolume(define); err != nil {
				logrus.Warningf("service: %s, create volume: %s, error: %+v \n skip it", version.ServiceID, v.VolumeName, err.Error())
				continue
			}
		}
	}

	//handle Shared storage
	tsmr, err := dbmanager.TenantServiceMountRelationDao().GetTenantServiceMountRelationsByService(version.ServiceID)
	if err != nil {
		return nil, err
	}

	if vs != nil && len(tsmr) > 0 {
		for _, t := range tsmr {
			sv, err := dbmanager.TenantServiceVolumeDao().GetVolumeByServiceIDAndName(t.DependServiceID, t.VolumeName)
			if err != nil {
				return nil, fmt.Errorf("service id: %s; volume name: %s; get dep volume: %v", t.DependServiceID, t.VolumeName, err)
			}
			vol := volume.NewVolumeManager(as, sv, t, version, envs, secrets, dbmanager)
			if vol != nil {
				if err = vol.CreateDependVolume(define); err != nil {
					logrus.Warningf("service: %s, create volume: %s, error: %+v \n skip it", version.ServiceID, t.VolumeName, err.Error())
					continue
				}
			}
		}
	}

	//handle slug file volume
	if version.DeliveredType == "slug" {
		//slug host path already is windows style
		define.SetVolume(dbmodel.ShareFileVolumeType, "slug", "/tmp/slug/slug.tgz", version.DeliveredPath, corev1.HostPathFile, true)
	}
	return define, nil
}

func createResources(as *v1.AppService) corev1.ResourceRequirements {
	var cpuRequest, cpuLimit int64
	if limit, ok := as.ExtensionSet["cpulimit"]; ok {
		limitint, _ := strconv.Atoi(limit)
		if limitint > 0 {
			cpuLimit = int64(limitint)
		}
	}
	if request, ok := as.ExtensionSet["cpurequest"]; ok {
		requestint, _ := strconv.Atoi(request)
		if requestint > 0 {
			cpuRequest = int64(requestint)
		}
	}
	if as.ContainerCPU > 0 && cpuRequest == 0 && cpuLimit == 0{
		cpuLimit = int64(as.ContainerCPU)
		cpuRequest = int64(as.ContainerCPU)
	}
	rr := createResourcesBySetting(as.ContainerMemory, cpuRequest, cpuLimit, int64(as.ContainerGPU))
	return rr
}

func getGPULableKey() corev1.ResourceName {
	if os.Getenv("GPU_LABLE_KEY") != "" {
		return corev1.ResourceName(os.Getenv("GPU_LABLE_KEY"))
	}
	return "rainbond.com/gpu-mem"
}

func checkUpstreamPluginRelation(serviceID string, dbmanager db.Manager) (bool, error) {
	inBoundOK, err := dbmanager.TenantServicePluginRelationDao().CheckSomeModelPluginByServiceID(
		serviceID,
		model.InBoundNetPlugin)
	if err != nil {
		return false, err
	}
	if inBoundOK {
		return inBoundOK, nil
	}
	return dbmanager.TenantServicePluginRelationDao().CheckSomeModelPluginByServiceID(
		serviceID,
		model.InBoundAndOutBoundNetPlugin)
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
				as.ServiceID)
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
		if mode == "liveness" {
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
			if l.LabelValue == model.LabelKeyServicePrivileged {
				continue
			}
			if strings.Contains(l.LabelValue, "=") {
				kv := strings.SplitN(l.LabelValue, "=", 2)
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
	enableGPU := as.ContainerGPU > 0
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
	if !enableGPU {
		nsr = append(nsr, corev1.NodeSelectorRequirement{
			Key:      client.LabelGPU,
			Values:   []string{"true"},
			Operator: corev1.NodeSelectorOpNotIn,
		})
	} else {
		nsr = append(nsr, corev1.NodeSelectorRequirement{
			Key:      client.LabelGPU,
			Values:   []string{"true"},
			Operator: corev1.NodeSelectorOpIn,
		})
	}
	if hostname, ok := as.ExtensionSet["selecthost"]; ok {
		nsr = append(nsr, corev1.NodeSelectorRequirement{
			Key:      "kubernetes.io/hostname",
			Values:   []string{hostname},
			Operator: corev1.NodeSelectorOpIn,
		})
	}
	if len(nsr) > 0 {
		affinity.NodeAffinity = &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{MatchExpressions: nsr},
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
	if as.Replicas == 1 && as.ExtensionSet["pod_ip"] != "" {
		logrus.Infof("custom set pod ip for calico, service %s, ip: %s", as.ServiceID, as.ExtensionSet["pod_ip"])
		annotations["cni.projectcalico.org/ipAddrs"] = fmt.Sprintf("[\"%s\"]", as.ExtensionSet["pod_ip"])
	}
	return annotations
}

func setImagePullSecrets() []corev1.LocalObjectReference {
	imagePullSecretName := os.Getenv("IMAGE_PULL_SECRET")
	if imagePullSecretName == "" {
		return nil
	}

	return []corev1.LocalObjectReference{
		{Name: imagePullSecretName},
	}
}

func createToleration(nodeSelector map[string]string) []corev1.Toleration {
	var tolerations []corev1.Toleration
	if value, exist := nodeSelector["type"]; exist && value == "virtual-kubelet" {
		tolerations = append(tolerations, corev1.Toleration{
			Key:      "virtual-kubelet.io/provider",
			Operator: corev1.TolerationOpExists,
		})
	}
	return tolerations
}

func createHostAliases(as *v1.AppService) []corev1.HostAlias {
	cache := make(map[string]*corev1.HostAlias)
	for k, v := range as.ExtensionSet {
		if strings.HasPrefix(k, "host_") {
			if net.ParseIP(v) != nil {
				if cache[v] != nil {
					cache[v].Hostnames = append(cache[v].Hostnames, k[5:])
				} else {
					cache[v] = &corev1.HostAlias{IP: v, Hostnames: []string{k[5:]}}
				}
			}
		}
	}
	var re []corev1.HostAlias
	for k := range cache {
		re = append(re, *cache[k])
	}
	return re
}

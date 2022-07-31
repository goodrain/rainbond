package handler

import (
	"encoding/json"
	"fmt"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	v1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path"
	"strconv"
	"strings"
)

//ConvertResource 处理资源
func (c *clusterAction) ConvertResource(ctx context.Context, namespace string, lr map[string]model.LabelResource) (map[string]model.ApplicationResource, *util.APIHandleError) {
	logrus.Infof("ConvertResource function begin")
	appsServices := make(map[string]model.ApplicationResource)
	for label, resource := range lr {
		c.workloadHandle(ctx, appsServices, resource, namespace, label)
	}
	logrus.Infof("ConvertResource function end")
	return appsServices, nil
}

func (c *clusterAction) workloadHandle(ctx context.Context, cr map[string]model.ApplicationResource, lr model.LabelResource, namespace string, label string) {
	app := label
	deployResource := c.workloadDeployments(lr.Workloads.Deployments, namespace)
	sfsResource := c.workloadStateFulSets(lr.Workloads.StateFulSets, namespace)
	jobResource := c.workloadJobs(lr.Workloads.Jobs, namespace)
	cjResource := c.workloadCronJobs(lr.Workloads.CronJobs, namespace)
	convertResource := append(deployResource, append(sfsResource, append(jobResource, append(cjResource)...)...)...)
	k8sResources := c.getAppKubernetesResources(ctx, lr.Others, namespace)
	cr[app] = model.ApplicationResource{
		ConvertResource:     convertResource,
		KubernetesResources: k8sResources,
	}
}

func (c *clusterAction) workloadDeployments(dmNames []string, namespace string) []model.ConvertResource {
	var componentsCR []model.ConvertResource
	for _, dmName := range dmNames {
		resources, err := c.clientset.AppsV1().Deployments(namespace).Get(context.Background(), dmName, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("Failed to get Deployment %v:%v", dmName, err)
			return nil
		}
		//BasicManagement
		basic := model.BasicManagement{
			ResourceType: model.Deployment,
			Replicas:     resources.Spec.Replicas,
			Memory:       resources.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().Value() / 1024 / 1024,
			CPU:          resources.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().Value(),
			Image:        resources.Spec.Template.Spec.Containers[0].Image,
			Cmd:          strings.Join(append(resources.Spec.Template.Spec.Containers[0].Command, resources.Spec.Template.Spec.Containers[0].Args...), " "),
		}
		c.podTemplateSpecResource(&componentsCR, basic, resources.Spec.Template, namespace, dmName, resources.Labels)
	}
	return componentsCR
}

func (c *clusterAction) workloadStateFulSets(sfsNames []string, namespace string) []model.ConvertResource {
	var componentsCR []model.ConvertResource
	for _, sfsName := range sfsNames {
		resources, err := c.clientset.AppsV1().StatefulSets(namespace).Get(context.Background(), sfsName, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("Failed to get Deployment %v:%v", sfsName, err)
			return nil
		}

		//BasicManagement
		basic := model.BasicManagement{
			ResourceType: model.StateFulSet,
			Replicas:     resources.Spec.Replicas,
			Memory:       resources.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().Value() / 1024 / 1024,
			CPU:          resources.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().Value(),
			Image:        resources.Spec.Template.Spec.Containers[0].Image,
			Cmd:          strings.Join(append(resources.Spec.Template.Spec.Containers[0].Command, resources.Spec.Template.Spec.Containers[0].Args...), " "),
		}
		c.podTemplateSpecResource(&componentsCR, basic, resources.Spec.Template, namespace, sfsName, resources.Labels)
	}
	return componentsCR
}

func (c *clusterAction) workloadJobs(jobNames []string, namespace string) []model.ConvertResource {
	var componentsCR []model.ConvertResource
	for _, jobName := range jobNames {
		resources, err := c.clientset.BatchV1().Jobs(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("Failed to get Deployment %v:%v", jobName, err)
			return nil
		}
		var BackoffLimit, Parallelism, ActiveDeadlineSeconds, Completions string
		if resources.Spec.BackoffLimit != nil {
			BackoffLimit = fmt.Sprintf("%v", *resources.Spec.BackoffLimit)
		}
		if resources.Spec.Parallelism != nil {
			Parallelism = fmt.Sprintf("%v", *resources.Spec.Parallelism)
		}
		if resources.Spec.ActiveDeadlineSeconds != nil {
			ActiveDeadlineSeconds = fmt.Sprintf("%v", *resources.Spec.ActiveDeadlineSeconds)
		}
		if resources.Spec.Completions != nil {
			Completions = fmt.Sprintf("%v", *resources.Spec.Completions)
		}
		job := model.JobStrategy{
			Schedule:              resources.Spec.Template.Spec.SchedulerName,
			BackoffLimit:          BackoffLimit,
			Parallelism:           Parallelism,
			ActiveDeadlineSeconds: ActiveDeadlineSeconds,
			Completions:           Completions,
		}
		//BasicManagement
		basic := model.BasicManagement{
			ResourceType: model.Job,
			Replicas:     resources.Spec.Completions,
			Memory:       resources.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().Value() / 1024 / 1024,
			CPU:          resources.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().Value(),
			Image:        resources.Spec.Template.Spec.Containers[0].Image,
			Cmd:          strings.Join(append(resources.Spec.Template.Spec.Containers[0].Command, resources.Spec.Template.Spec.Containers[0].Args...), " "),
			JobStrategy:  job,
		}
		c.podTemplateSpecResource(&componentsCR, basic, resources.Spec.Template, namespace, jobName, resources.Labels)
	}
	return componentsCR
}

func (c *clusterAction) workloadCronJobs(cjNames []string, namespace string) []model.ConvertResource {
	var componentsCR []model.ConvertResource
	for _, cjName := range cjNames {
		resources, err := c.clientset.BatchV1beta1().CronJobs(namespace).Get(context.Background(), cjName, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("Failed to get Deployment %v:%v", cjName, err)
			return nil
		}
		BackoffLimit, Parallelism, ActiveDeadlineSeconds, Completions := "", "", "", ""
		if resources.Spec.JobTemplate.Spec.BackoffLimit != nil {
			BackoffLimit = fmt.Sprintf("%v", *resources.Spec.JobTemplate.Spec.BackoffLimit)
		}
		if resources.Spec.JobTemplate.Spec.Parallelism != nil {
			Parallelism = fmt.Sprintf("%v", *resources.Spec.JobTemplate.Spec.Parallelism)
		}
		if resources.Spec.JobTemplate.Spec.ActiveDeadlineSeconds != nil {
			ActiveDeadlineSeconds = fmt.Sprintf("%v", *resources.Spec.JobTemplate.Spec.ActiveDeadlineSeconds)
		}
		if resources.Spec.JobTemplate.Spec.Completions != nil {
			Completions = fmt.Sprintf("%v", *resources.Spec.JobTemplate.Spec.Completions)
		}
		job := model.JobStrategy{
			Schedule:              resources.Spec.Schedule,
			BackoffLimit:          BackoffLimit,
			Parallelism:           Parallelism,
			ActiveDeadlineSeconds: ActiveDeadlineSeconds,
			Completions:           Completions,
		}
		//BasicManagement
		basic := model.BasicManagement{
			ResourceType: model.CronJob,
			Replicas:     resources.Spec.JobTemplate.Spec.Completions,
			Memory:       resources.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().Value() / 1024 / 1024,
			CPU:          resources.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().Value(),
			Image:        resources.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image,
			Cmd:          strings.Join(append(resources.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Command, resources.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Args...), " "),
			JobStrategy:  job,
		}
		c.podTemplateSpecResource(&componentsCR, basic, resources.Spec.JobTemplate.Spec.Template, namespace, cjName, resources.Labels)
	}
	return componentsCR
}

func (c *clusterAction) podTemplateSpecResource(componentsCR *[]model.ConvertResource, basic model.BasicManagement, template corev1.PodTemplateSpec, namespace, name string, rsLabel map[string]string) {
	//Port
	var ps []model.PortManagement
	for _, port := range template.Spec.Containers[0].Ports {
		if string(port.Protocol) == "UDP" {
			ps = append(ps, model.PortManagement{
				Port:     port.ContainerPort,
				Protocol: "UDP",
				Inner:    false,
				Outer:    false,
			})
			continue
		}
		if string(port.Protocol) == "TCP" {
			ps = append(ps, model.PortManagement{
				Port:     port.ContainerPort,
				Protocol: "TCP",
				Inner:    false,
				Outer:    false,
			})
			continue
		}
		logrus.Warningf("Transport protocol type not recognized%v", port.Protocol)
	}

	//ENV
	var envs []model.ENVManagement
	for i := 0; i < len(template.Spec.Containers[0].Env); i++ {
		if cm := template.Spec.Containers[0].Env[i].ValueFrom; cm == nil {
			envs = append(envs, model.ENVManagement{
				ENVKey:     template.Spec.Containers[0].Env[i].Name,
				ENVValue:   template.Spec.Containers[0].Env[i].Value,
				ENVExplain: "",
			})
			template.Spec.Containers[0].Env = append(template.Spec.Containers[0].Env[:i], template.Spec.Containers[0].Env[i+1:]...)
		}
	}

	//Configs
	var configs []model.ConfigManagement
	//这一块是处理配置文件
	//配置文件的名字最终都是configmap里面的key值。
	//volume在被挂载后存在四种情况
	//第一种是volume存在items，volumeMount的SubPath不等于空。路径直接是volumeMount里面的mountPath。
	//第二种是volume存在items，volumeMount的SubPath等于空。路径则变成volumeMount里面的mountPath拼接上items里面每一个元素的key值。
	//第三种是volume不存在items，volumeMount的SubPath不等于空。路径直接是volumeMount里面的mountPath。
	//第四种是volume不存在items，volumeMount的SubPath等于空。路径则变成volumeMount里面的mountPath拼接上configmap资源里面每一个元素的key值
	cmMap := make(map[string]corev1.ConfigMap)
	cmList, err := c.clientset.CoreV1().ConfigMaps(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to get ConfigMap%v", err)
	}
	for _, volume := range template.Spec.Volumes {
		for _, cm := range cmList.Items {
			cmMap[cm.Name] = cm
		}
		if volume.ConfigMap != nil && err == nil {
			cm, _ := cmMap[volume.ConfigMap.Name]
			cmData := cm.Data
			isLog := true
			for _, volumeMount := range template.Spec.Containers[0].VolumeMounts {
				if volume.Name != volumeMount.Name {
					continue
				}
				isLog = false
				if volume.ConfigMap.Items != nil {
					if volumeMount.SubPath != "" {
						configName := ""
						var mode int32
						for _, item := range volume.ConfigMap.Items {
							if item.Path == volumeMount.SubPath {
								configName = item.Key
								mode = *item.Mode
							}
						}
						configs = append(configs, model.ConfigManagement{
							ConfigName:  configName,
							ConfigPath:  volumeMount.MountPath,
							ConfigValue: cmData[configName],
							Mode:        mode,
						})
						continue
					}
					p := volumeMount.MountPath
					for _, item := range volume.ConfigMap.Items {
						p := path.Join(p, item.Path)
						var mode int32
						if item.Mode != nil {
							mode = *item.Mode
						}
						configs = append(configs, model.ConfigManagement{
							ConfigName:  item.Key,
							ConfigPath:  p,
							ConfigValue: cmData[item.Key],
							Mode:        mode,
						})
					}
				} else {
					if volumeMount.SubPath != "" {
						configs = append(configs, model.ConfigManagement{
							ConfigName:  volumeMount.SubPath,
							ConfigPath:  volumeMount.MountPath,
							ConfigValue: cmData[volumeMount.SubPath],
							Mode:        *volume.ConfigMap.DefaultMode,
						})
						continue
					}
					mountPath := volumeMount.MountPath
					for key, val := range cmData {
						mountPath = path.Join(mountPath, key)
						configs = append(configs, model.ConfigManagement{
							ConfigName:  key,
							ConfigPath:  mountPath,
							ConfigValue: val,
							Mode:        *volume.ConfigMap.DefaultMode,
						})
					}
				}
			}
			if isLog {
				logrus.Warningf("configmap type resource %v is not mounted in volumemount", volume.ConfigMap.Name)
			}
		}
	}

	//TelescopicManagement
	HPAResource, err := c.clientset.AutoscalingV1().HorizontalPodAutoscalers(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to get HorizontalPodAutoscalers list:%v", err)
	}
	var t model.TelescopicManagement
	//这一块就是自动伸缩的对应解析，
	//需要注意的一点是hpa的cpu和memory的阈值设置是通过Annotations["autoscaling.alpha.kubernetes.io/metrics"]字段设置
	//而且它的返回值是个json字符串所以设置了一个结构体进行解析。
	for _, hpa := range HPAResource.Items {
		if hpa.Spec.ScaleTargetRef.Kind != model.Deployment || hpa.Spec.ScaleTargetRef.Name != name {
			t.Enable = false
			continue
		}
		t.Enable = true
		t.MinReplicas = *hpa.Spec.MinReplicas
		t.MaxReplicas = hpa.Spec.MaxReplicas
		var cpuormemorys []*dbmodel.TenantServiceAutoscalerRuleMetrics
		cpuUsage := hpa.Spec.TargetCPUUtilizationPercentage
		if cpuUsage != nil {
			cpuormemorys = append(cpuormemorys, &dbmodel.TenantServiceAutoscalerRuleMetrics{
				MetricsType:       "resource_metrics",
				MetricsName:       "cpu",
				MetricTargetType:  "utilization",
				MetricTargetValue: int(*cpuUsage),
			})
		}
		CPUAndMemoryJSON, ok := hpa.Annotations["autoscaling.alpha.kubernetes.io/metrics"]
		if ok {
			type com struct {
				T        string `json:"type"`
				Resource map[string]interface{}
			}
			var c []com
			err := json.Unmarshal([]byte(CPUAndMemoryJSON), &c)
			if err != nil {
				logrus.Errorf("autoscaling.alpha.kubernetes.io/metrics parsing failed：%v", err)
			}

			for _, cpuormemory := range c {
				switch cpuormemory.Resource["name"] {
				case "cpu":
					cpu := fmt.Sprint(cpuormemory.Resource["targetAverageValue"])
					cpuUnit := cpu[len(cpu)-1:]
					var cpuUsage int
					if cpuUnit == "m" {
						cpuUsage, _ = strconv.Atoi(cpu[:len(cpu)-1])
					}
					if cpuUnit == "g" || cpuUnit == "G" {
						cpuUsage, _ = strconv.Atoi(cpu[:len(cpu)-1])
						cpuUsage = cpuUsage * 1024
					}
					cpuormemorys = append(cpuormemorys, &dbmodel.TenantServiceAutoscalerRuleMetrics{
						MetricsType:       "resource_metrics",
						MetricsName:       "cpu",
						MetricTargetType:  "average_value",
						MetricTargetValue: cpuUsage,
					})
				case "memory":
					memory := fmt.Sprint(cpuormemory.Resource["targetAverageValue"])
					memoryUnit := memory[:len(memory)-1]
					var MemoryUsage int
					if memoryUnit == "m" {
						MemoryUsage, _ = strconv.Atoi(memory[:len(memory)-1])
					}
					if memoryUnit == "g" || memoryUnit == "G" {
						MemoryUsage, _ = strconv.Atoi(memory[:len(memory)-1])
						MemoryUsage = MemoryUsage * 1024
					}
					cpuormemorys = append(cpuormemorys, &dbmodel.TenantServiceAutoscalerRuleMetrics{
						MetricsType:       "resource_metrics",
						MetricsName:       "cpu",
						MetricTargetType:  "average_value",
						MetricTargetValue: MemoryUsage,
					})
				}

			}
		}
		t.CPUOrMemory = cpuormemorys
	}

	//HealthyCheckManagement
	var hcm model.HealthyCheckManagement
	livenessProbe := template.Spec.Containers[0].LivenessProbe
	if livenessProbe != nil {
		var httpHeaders []string
		for _, httpHeader := range livenessProbe.HTTPGet.HTTPHeaders {
			nv := httpHeader.Name + "=" + httpHeader.Value
			httpHeaders = append(httpHeaders, nv)
		}
		hcm.Status = 1
		hcm.DetectionMethod = strings.ToLower(string(livenessProbe.HTTPGet.Scheme))
		hcm.Port = int(livenessProbe.HTTPGet.Port.IntVal)
		hcm.Path = livenessProbe.HTTPGet.Path
		if livenessProbe.Exec != nil {
			hcm.Command = strings.Join(livenessProbe.Exec.Command, " ")
		}
		hcm.HTTPHeader = strings.Join(httpHeaders, ",")
		hcm.Mode = "liveness"
		hcm.InitialDelaySecond = int(livenessProbe.InitialDelaySeconds)
		hcm.PeriodSecond = int(livenessProbe.PeriodSeconds)
		hcm.TimeoutSecond = int(livenessProbe.TimeoutSeconds)
		hcm.FailureThreshold = int(livenessProbe.FailureThreshold)
		hcm.SuccessThreshold = int(livenessProbe.SuccessThreshold)
	} else {
		readinessProbe := template.Spec.Containers[0].ReadinessProbe
		if readinessProbe != nil {
			var httpHeaders []string
			for _, httpHeader := range readinessProbe.HTTPGet.HTTPHeaders {
				nv := httpHeader.Name + "=" + httpHeader.Value
				httpHeaders = append(httpHeaders, nv)
			}
			hcm.Status = 1
			hcm.DetectionMethod = strings.ToLower(string(readinessProbe.HTTPGet.Scheme))
			hcm.Mode = "readiness"
			hcm.Port = int(readinessProbe.HTTPGet.Port.IntVal)
			hcm.Path = readinessProbe.HTTPGet.Path
			if readinessProbe.Exec != nil {
				hcm.Command = strings.Join(readinessProbe.Exec.Command, " ")
			}
			hcm.HTTPHeader = strings.Join(httpHeaders, ",")
			hcm.InitialDelaySecond = int(readinessProbe.InitialDelaySeconds)
			hcm.PeriodSecond = int(readinessProbe.PeriodSeconds)
			hcm.TimeoutSecond = int(readinessProbe.TimeoutSeconds)
			hcm.FailureThreshold = int(readinessProbe.FailureThreshold)
			hcm.SuccessThreshold = int(readinessProbe.SuccessThreshold)
		}
	}

	var attributes []*dbmodel.ComponentK8sAttributes
	if template.Spec.Containers[0].Env != nil && len(template.Spec.Containers[0].Env) > 0 {
		envYaml, err := ObjectToJSONORYaml("yaml", template.Spec.Containers[0].Env)
		if err != nil {
			logrus.Errorf("deployment:%v env %v", name, err)
		}
		envAttributes := &dbmodel.ComponentK8sAttributes{
			Name:           dbmodel.K8sAttributeNameENV,
			SaveType:       "yaml",
			AttributeValue: envYaml,
		}
		attributes = append(attributes, envAttributes)
	}
	if template.Spec.Volumes != nil {
		volumesYaml, err := ObjectToJSONORYaml("yaml", template.Spec.Volumes)
		if err != nil {
			logrus.Errorf("deployment:%v volumes %v", name, err)
		}
		volumesAttributes := &dbmodel.ComponentK8sAttributes{
			Name:           dbmodel.K8sAttributeNameVolumes,
			SaveType:       "yaml",
			AttributeValue: volumesYaml,
		}
		attributes = append(attributes, volumesAttributes)

	}
	if template.Spec.Containers[0].VolumeMounts != nil {
		volumeMountsYaml, err := ObjectToJSONORYaml("yaml", template.Spec.Containers[0].VolumeMounts)
		if err != nil {
			logrus.Errorf("deployment:%v volumeMounts %v", name, err)
		}
		volumeMountsAttributes := &dbmodel.ComponentK8sAttributes{
			Name:           dbmodel.K8sAttributeNameVolumeMounts,
			SaveType:       "yaml",
			AttributeValue: volumeMountsYaml,
		}
		attributes = append(attributes, volumeMountsAttributes)
	}
	if template.Spec.ServiceAccountName != "" {
		serviceAccountAttributes := &dbmodel.ComponentK8sAttributes{
			Name:           dbmodel.K8sAttributeNameServiceAccountName,
			SaveType:       "string",
			AttributeValue: template.Spec.ServiceAccountName,
		}
		attributes = append(attributes, serviceAccountAttributes)
	}
	if rsLabel != nil {
		labelsJSON, err := ObjectToJSONORYaml("json", rsLabel)
		if err != nil {
			logrus.Errorf("deployment:%v labels %v", name, err)
		}
		labelsAttributes := &dbmodel.ComponentK8sAttributes{
			Name:           dbmodel.K8sAttributeNameLabels,
			SaveType:       "json",
			AttributeValue: labelsJSON,
		}
		attributes = append(attributes, labelsAttributes)
	}

	if template.Spec.NodeSelector != nil {
		NodeSelectorJSON, err := ObjectToJSONORYaml("json", template.Spec.NodeSelector)
		if err != nil {
			logrus.Errorf("deployment:%v nodeSelector %v", name, err)
		}
		nodeSelectorAttributes := &dbmodel.ComponentK8sAttributes{
			Name:           dbmodel.K8sAttributeNameNodeSelector,
			SaveType:       "json",
			AttributeValue: NodeSelectorJSON,
		}
		attributes = append(attributes, nodeSelectorAttributes)
	}
	if template.Spec.Tolerations != nil {
		tolerationsYaml, err := ObjectToJSONORYaml("yaml", template.Spec.Tolerations)
		if err != nil {
			logrus.Errorf("deployment:%v tolerations %v", name, err)
		}
		tolerationsAttributes := &dbmodel.ComponentK8sAttributes{
			Name:           dbmodel.K8sAttributeNameTolerations,
			SaveType:       "yaml",
			AttributeValue: tolerationsYaml,
		}
		attributes = append(attributes, tolerationsAttributes)
	}
	if template.Spec.Affinity != nil {
		affinityYaml, err := ObjectToJSONORYaml("yaml", template.Spec.Affinity)
		if err != nil {
			logrus.Errorf("deployment:%v affinity %v", name, err)
		}
		affinityAttributes := &dbmodel.ComponentK8sAttributes{
			Name:           dbmodel.K8sAttributeNameAffinity,
			SaveType:       "yaml",
			AttributeValue: affinityYaml,
		}
		attributes = append(attributes, affinityAttributes)
	}
	if securityContext := template.Spec.Containers[0].SecurityContext; securityContext != nil && securityContext.Privileged != nil {
		privilegedAttributes := &dbmodel.ComponentK8sAttributes{
			Name:           dbmodel.K8sAttributeNamePrivileged,
			SaveType:       "string",
			AttributeValue: strconv.FormatBool(*securityContext.Privileged),
		}
		attributes = append(attributes, privilegedAttributes)
	}

	*componentsCR = append(*componentsCR, model.ConvertResource{
		ComponentsName:                   name,
		BasicManagement:                  basic,
		PortManagement:                   ps,
		ENVManagement:                    envs,
		ConfigManagement:                 configs,
		TelescopicManagement:             t,
		HealthyCheckManagement:           hcm,
		ComponentK8sAttributesManagement: attributes,
	})
}

func (c *clusterAction) getAppKubernetesResources(ctx context.Context, others model.OtherResource, namespace string) []dbmodel.K8sResource {
	var k8sResources []dbmodel.K8sResource
	servicesMap := make(map[string]corev1.Service)
	servicesList, err := c.clientset.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("namespace:%v get services error:%v", namespace, err)
	}
	if len(others.Services) != 0 && err == nil {
		for _, services := range servicesList.Items {
			servicesMap[services.Name] = services
		}
		for _, servicesName := range others.Services {
			services, _ := servicesMap[servicesName]
			services.Kind = model.Service
			services.Status = corev1.ServiceStatus{}
			services.APIVersion = "v1"
			services.ManagedFields = []metav1.ManagedFieldsEntry{}
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", services)
			if err != nil {
				logrus.Errorf("namespace:%v service:%v error: %v", namespace, services.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:    services.Name,
				Kind:    model.Service,
				Content: kubernetesResourcesYAML,
				Success: 1,
				Status:  "创建成功",
			})
		}
	}

	pvcMap := make(map[string]corev1.PersistentVolumeClaim)
	pvcList, err := c.clientset.CoreV1().PersistentVolumeClaims(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("namespace:%v get pvc error:%v", namespace, err)
	}
	if len(others.PVC) != 0 && err == nil {
		for _, pvc := range pvcList.Items {
			pvcMap[pvc.Name] = pvc
		}
		for _, pvcName := range others.PVC {
			pvc, _ := pvcMap[pvcName]
			pvc.Status = corev1.PersistentVolumeClaimStatus{}
			pvc.ManagedFields = []metav1.ManagedFieldsEntry{}
			pvc.Kind = model.PVC
			pvc.APIVersion = "v1"
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", pvc)
			if err != nil {
				logrus.Errorf("namespace:%v pvc:%v error: %v", namespace, pvc.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:    pvc.Name,
				Kind:    model.PVC,
				Content: kubernetesResourcesYAML,
				Success: 1,
				Status:  "创建成功",
			})
		}
	}

	ingressMap := make(map[string]networkingv1.Ingress)
	ingressList, err := c.clientset.NetworkingV1().Ingresses(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("namespace:%v get ingresses error:%v", namespace, err)
	}
	if len(others.Ingresses) != 0 && err == nil {
		for _, ingress := range ingressList.Items {
			ingressMap[ingress.Name] = ingress
		}
		for _, ingressName := range others.Ingresses {
			ingresses, _ := ingressMap[ingressName]
			ingresses.Status = networkingv1.IngressStatus{}
			ingresses.ManagedFields = []metav1.ManagedFieldsEntry{}
			ingresses.Kind = model.Ingress
			ingresses.APIVersion = "networking.k8s.io/v1"
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", ingresses)
			if err != nil {
				logrus.Errorf("namespace:%v ingresses:%v error: %v", namespace, ingresses.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:    ingresses.Name,
				Kind:    model.Ingress,
				Content: kubernetesResourcesYAML,
				Success: 1,
				Status:  "创建成功",
			})
		}
	}

	networkPoliciesMap := make(map[string]networkingv1.NetworkPolicy)
	networkPoliciesList, err := c.clientset.NetworkingV1().NetworkPolicies(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("namespace:%v get NetworkPolicies error:%v", namespace, err)
	}
	if len(others.NetworkPolicies) != 0 && err == nil {
		for _, networkPolicies := range networkPoliciesList.Items {
			networkPoliciesMap[networkPolicies.Name] = networkPolicies
		}
		for _, networkPoliciesName := range others.NetworkPolicies {
			networkPolicies, _ := networkPoliciesMap[networkPoliciesName]
			networkPolicies.ManagedFields = []metav1.ManagedFieldsEntry{}
			networkPolicies.Kind = model.NetworkPolicy
			networkPolicies.APIVersion = "networking.k8s.io/v1"
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", networkPolicies)
			if err != nil {
				logrus.Errorf("namespace:%v NetworkPolicies:%v error: %v", namespace, networkPolicies.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:    networkPolicies.Name,
				Kind:    model.NetworkPolicy,
				Content: kubernetesResourcesYAML,
				Success: 1,
				Status:  "创建成功",
			})
		}
	}

	cmMap := make(map[string]corev1.ConfigMap)
	cmList, err := c.clientset.CoreV1().ConfigMaps(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("namespace:%v get ConfigMaps error:%v", namespace, err)
	}
	if len(others.ConfigMaps) != 0 && err == nil {
		for _, cm := range cmList.Items {
			cmMap[cm.Name] = cm
		}
		for _, configMapsName := range others.ConfigMaps {
			configMaps, _ := cmMap[configMapsName]
			configMaps.ManagedFields = []metav1.ManagedFieldsEntry{}
			configMaps.Kind = model.ConfigMap
			configMaps.APIVersion = "v1"
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", configMaps)
			if err != nil {
				logrus.Errorf("namespace:%v ConfigMaps:%v error: %v", namespace, configMaps.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:    configMaps.Name,
				Kind:    model.ConfigMap,
				Content: kubernetesResourcesYAML,
				Success: 1,
				Status:  "创建成功",
			})
		}
	}

	secretsMap := make(map[string]corev1.Secret)
	secretsList, err := c.clientset.CoreV1().Secrets(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("namespace:%v get Secrets error:%v", namespace, err)
	}
	if len(others.Secrets) != 0 && err == nil {
		for _, secrets := range secretsList.Items {
			secretsMap[secrets.Name] = secrets
		}
		for _, secretsName := range others.Secrets {
			secrets, _ := secretsMap[secretsName]
			secrets.ManagedFields = []metav1.ManagedFieldsEntry{}
			secrets.Kind = model.Secret
			secrets.APIVersion = "v1"
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", secrets)
			if err != nil {
				logrus.Errorf("namespace:%v Secrets:%v error: %v", namespace, secrets.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:    secrets.Name,
				Kind:    model.Secret,
				Content: kubernetesResourcesYAML,
				Success: 1,
				Status:  "创建成功",
			})
		}
	}

	serviceAccountsMap := make(map[string]corev1.ServiceAccount)
	serviceAccountsList, err := c.clientset.CoreV1().ServiceAccounts(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("namespace:%v get ServiceAccounts error:%v", namespace, err)
	}
	if len(others.ServiceAccounts) != 0 && err == nil {
		for _, serviceAccounts := range serviceAccountsList.Items {
			serviceAccountsMap[serviceAccounts.Name] = serviceAccounts
		}
		for _, serviceAccountsName := range others.ServiceAccounts {
			serviceAccounts, _ := serviceAccountsMap[serviceAccountsName]
			serviceAccounts.ManagedFields = []metav1.ManagedFieldsEntry{}
			serviceAccounts.Kind = model.ServiceAccount
			serviceAccounts.APIVersion = "v1"
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", serviceAccounts)
			if err != nil {
				logrus.Errorf("namespace:%v ServiceAccounts:%v error: %v", namespace, serviceAccounts.Name, err)
				continue
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:    serviceAccounts.Name,
				Kind:    model.ServiceAccount,
				Content: kubernetesResourcesYAML,
				Success: 1,
				Status:  "创建成功",
			})
		}
	}

	roleBindingsMap := make(map[string]rbacv1.RoleBinding)
	roleBindingsList, _ := c.clientset.RbacV1().RoleBindings(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("namespace:%v get RoleBindings error:%v", namespace, err)
	}
	if len(others.RoleBindings) != 0 && err == nil {
		for _, roleBindings := range roleBindingsList.Items {
			roleBindingsMap[roleBindings.Name] = roleBindings
		}
		for _, roleBindingsName := range others.RoleBindings {
			roleBindings, _ := roleBindingsMap[roleBindingsName]
			roleBindings.ManagedFields = []metav1.ManagedFieldsEntry{}
			roleBindings.Kind = model.RoleBinding
			roleBindings.APIVersion = "rbac.authorization.k8s.io/v1"
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", roleBindings)
			if err != nil {
				logrus.Errorf("namespace:%v RoleBindings:%v error: %v", namespace, roleBindings.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:    roleBindings.Name,
				Kind:    model.RoleBinding,
				Content: kubernetesResourcesYAML,
				Success: 1,
				Status:  "创建成功",
			})
		}
	}

	hpaMap := make(map[string]v1.HorizontalPodAutoscaler)
	hpaList, _ := c.clientset.AutoscalingV1().HorizontalPodAutoscalers(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("namespace:%v get HorizontalPodAutoscalers error:%v", namespace, err)
	}
	if len(others.HorizontalPodAutoscalers) != 0 && err == nil {
		for _, hpa := range hpaList.Items {
			hpaMap[hpa.Name] = hpa
		}
		for _, hpaName := range others.HorizontalPodAutoscalers {
			hpa, _ := hpaMap[hpaName]
			hpa.Status = v1.HorizontalPodAutoscalerStatus{}
			hpa.ManagedFields = []metav1.ManagedFieldsEntry{}
			hpa.Kind = model.HorizontalPodAutoscaler
			hpa.APIVersion = "autoscaling/v1"
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", hpa)
			if err != nil {
				logrus.Errorf("namespace:%v HorizontalPodAutoscalers:%v error: %v", namespace, hpa.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:    hpa.Name,
				Kind:    model.HorizontalPodAutoscaler,
				Content: kubernetesResourcesYAML,
				Success: 1,
				Status:  "创建成功",
			})
		}
	}

	rolesMap := make(map[string]rbacv1.Role)
	rolesList, err := c.clientset.RbacV1().Roles(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("namespace:%v get roles error:%v", namespace, err)
	}
	if len(others.Roles) != 0 && err == nil {
		for _, roles := range rolesList.Items {
			rolesMap[roles.Name] = roles
		}
		for _, rolesName := range others.Roles {
			roles, _ := rolesMap[rolesName]
			roles.Kind = model.Role
			roles.APIVersion = "rbac.authorization.k8s.io/v1"
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", roles)
			if err != nil {
				logrus.Errorf("namespace:%v roles:%v error: %v", namespace, roles.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:    roles.Name,
				Kind:    model.Role,
				Content: kubernetesResourcesYAML,
				Status:  "创建成功",
				Success: 1,
			})
		}
	}
	return k8sResources
}

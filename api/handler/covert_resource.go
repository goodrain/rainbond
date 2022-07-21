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
	dmCR := c.workloadDeployments(ctx, lr.Workloads.Deployments, namespace)
	sfsCR := c.workloadStateFulSets(ctx, lr.Workloads.StateFulSets, namespace)
	jCR := c.workloadJobs(ctx, lr.Workloads.Jobs, namespace)
	wCJ := c.workloadCronJobs(ctx, lr.Workloads.CronJobs, namespace)
	convertResource := append(dmCR, append(sfsCR, append(jCR, append(wCJ)...)...)...)

	k8sResources := c.getAppKubernetesResources(ctx, lr.Others, namespace)
	cr[app] = model.ApplicationResource{
		ConvertResource:     convertResource,
		KubernetesResources: k8sResources,
	}
}

func (c *clusterAction) workloadDeployments(ctx context.Context, dmNames []string, namespace string) []model.ConvertResource {
	var componentsCR []model.ConvertResource
	for _, dmName := range dmNames {
		resources, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, dmName, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("Failed to get Deployment %v:%v", dmName, err)
			return nil
		}

		//BasicManagement
		b := model.BasicManagement{
			ResourceType: model.Deployment,
			Replicas:     *resources.Spec.Replicas,
			Memory:       resources.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().Value() / 1024 / 1024,
			CPU:          resources.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().Value(),
			Image:        resources.Spec.Template.Spec.Containers[0].Image,
			Cmd:          strings.Join(append(resources.Spec.Template.Spec.Containers[0].Command, resources.Spec.Template.Spec.Containers[0].Args...), " "),
		}

		//Port
		var ps []model.PortManagement
		for _, port := range resources.Spec.Template.Spec.Containers[0].Ports {
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
					Protocol: "UDP",
					Inner:    false,
					Outer:    false,
				})
				continue
			}
			logrus.Warningf("Transport protocol type not recognized%v", port.Protocol)
		}

		//ENV
		var envs []model.ENVManagement
		for _, env := range resources.Spec.Template.Spec.Containers[0].Env {
			if cm := env.ValueFrom; cm == nil {
				envs = append(envs, model.ENVManagement{
					ENVKey:     env.Name,
					ENVValue:   env.Value,
					ENVExplain: "",
				})
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
		cmList, err := c.clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get ConfigMap%v", err)
		}
		for _, volume := range resources.Spec.Template.Spec.Volumes {
			for _, cm := range cmList.Items {
				cmMap[cm.Name] = cm
			}
			if volume.ConfigMap != nil && err == nil {
				cm, _ := cmMap[volume.ConfigMap.Name]
				cmData := cm.Data
				isLog := true
				for _, volumeMount := range resources.Spec.Template.Spec.Containers[0].VolumeMounts {
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
							configs = append(configs, model.ConfigManagement{
								ConfigName:  item.Key,
								ConfigPath:  p,
								ConfigValue: cmData[item.Key],
								Mode:        *item.Mode,
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
		HPAResource, err := c.clientset.AutoscalingV1().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get HorizontalPodAutoscalers list:%v", err)
			return nil
		}
		var t model.TelescopicManagement
		//这一块就是自动伸缩的对应解析，
		//需要注意的一点是hpa的cpu和memory的阈值设置是通过Annotations["autoscaling.alpha.kubernetes.io/metrics"]字段设置
		//而且它的返回值是个json字符串所以设置了一个结构体进行解析。
		for _, hpa := range HPAResource.Items {
			if hpa.Spec.ScaleTargetRef.Kind != model.Deployment || hpa.Spec.ScaleTargetRef.Name != dmName {
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
					return nil
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
		livenessProbe := resources.Spec.Template.Spec.Containers[0].LivenessProbe
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
			readinessProbe := resources.Spec.Template.Spec.Containers[0].ReadinessProbe
			if readinessProbe != nil {
				var httpHeaders []string
				for _, httpHeader := range readinessProbe.HTTPGet.HTTPHeaders {
					nv := httpHeader.Name + "=" + httpHeader.Value
					httpHeaders = append(httpHeaders, nv)
				}
				hcm.Status = 1
				hcm.DetectionMethod = strings.ToLower(string(readinessProbe.HTTPGet.Scheme))
				hcm.Mode = "readiness"
				hcm.Port = int(livenessProbe.HTTPGet.Port.IntVal)
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

		if resources.Spec.Template.Spec.Volumes != nil {
			volumesYaml, err := ObjectToJSONORYaml("yaml", resources.Spec.Template.Spec.Volumes)
			if err != nil {
				logrus.Errorf("deployment:%v volumes %v", dmName, err)
				return nil
			}
			volumesAttributes := &dbmodel.ComponentK8sAttributes{
				Name:           dbmodel.K8sAttributeNameVolumes,
				SaveType:       "yaml",
				AttributeValue: volumesYaml,
			}
			attributes = append(attributes, volumesAttributes)

		}
		if resources.Spec.Template.Spec.Containers[0].VolumeMounts != nil {
			volumeMountsYaml, err := ObjectToJSONORYaml("yaml", resources.Spec.Template.Spec.Containers[0].VolumeMounts)
			if err != nil {
				logrus.Errorf("deployment:%v volumeMounts %v", dmName, err)
				return nil
			}
			volumeMountsAttributes := &dbmodel.ComponentK8sAttributes{
				Name:           dbmodel.K8sAttributeNameVolumeMounts,
				SaveType:       "yaml",
				AttributeValue: volumeMountsYaml,
			}
			attributes = append(attributes, volumeMountsAttributes)
		}
		if resources.Spec.Template.Spec.ServiceAccountName != "" {
			serviceAccountAttributes := &dbmodel.ComponentK8sAttributes{
				Name:           dbmodel.K8sAttributeNameServiceAccountName,
				SaveType:       "string",
				AttributeValue: resources.Spec.Template.Spec.ServiceAccountName,
			}
			attributes = append(attributes, serviceAccountAttributes)
		}
		if resources.Labels != nil {
			labelsJSON, err := ObjectToJSONORYaml("json", resources.Labels)
			if err != nil {
				logrus.Errorf("deployment:%v labels %v", dmName, err)
				return nil
			}
			labelsAttributes := &dbmodel.ComponentK8sAttributes{
				Name:           dbmodel.K8sAttributeNameLabels,
				SaveType:       "json",
				AttributeValue: labelsJSON,
			}
			attributes = append(attributes, labelsAttributes)
		}
		if resources.Spec.Template.Spec.NodeSelector != nil {
			NodeSelectorJSON, err := ObjectToJSONORYaml("json", resources.Spec.Template.Spec.NodeSelector)
			if err != nil {
				logrus.Errorf("deployment:%v nodeSelector %v", dmName, err)
				return nil
			}
			nodeSelectorAttributes := &dbmodel.ComponentK8sAttributes{
				Name:           dbmodel.K8sAttributeNameNodeSelector,
				SaveType:       "json",
				AttributeValue: NodeSelectorJSON,
			}
			attributes = append(attributes, nodeSelectorAttributes)
		}
		if resources.Spec.Template.Spec.Tolerations != nil {
			tolerationsYaml, err := ObjectToJSONORYaml("yaml", resources.Spec.Template.Spec.Tolerations)
			if err != nil {
				logrus.Errorf("deployment:%v tolerations %v", dmName, err)
				return nil
			}
			tolerationsAttributes := &dbmodel.ComponentK8sAttributes{
				Name:           dbmodel.K8sAttributeNameTolerations,
				SaveType:       "yaml",
				AttributeValue: tolerationsYaml,
			}
			attributes = append(attributes, tolerationsAttributes)
		}
		if resources.Spec.Template.Spec.Affinity != nil {
			affinityYaml, err := ObjectToJSONORYaml("yaml", resources.Spec.Template.Spec.Affinity)
			if err != nil {
				logrus.Errorf("deployment:%v affinity %v", dmName, err)
				return nil
			}
			affinityAttributes := &dbmodel.ComponentK8sAttributes{
				Name:           dbmodel.K8sAttributeNameAffinity,
				SaveType:       "yaml",
				AttributeValue: affinityYaml,
			}
			attributes = append(attributes, affinityAttributes)
		}
		if securityContext := resources.Spec.Template.Spec.Containers[0].SecurityContext; securityContext != nil && securityContext.Privileged != nil {
			privilegedAttributes := &dbmodel.ComponentK8sAttributes{
				Name:           dbmodel.K8sAttributeNamePrivileged,
				SaveType:       "string",
				AttributeValue: strconv.FormatBool(*securityContext.Privileged),
			}
			attributes = append(attributes, privilegedAttributes)
		}

		componentsCR = append(componentsCR, model.ConvertResource{
			ComponentsName:                   dmName,
			BasicManagement:                  b,
			PortManagement:                   ps,
			ENVManagement:                    envs,
			ConfigManagement:                 configs,
			TelescopicManagement:             t,
			HealthyCheckManagement:           hcm,
			ComponentK8sAttributesManagement: attributes,
		})

	}
	return componentsCR
}

func (c *clusterAction) workloadStateFulSets(ctx context.Context, sfsNames []string, namespace string) []model.ConvertResource {
	return nil
}

func (c *clusterAction) workloadJobs(ctx context.Context, jNames []string, namespace string) []model.ConvertResource {
	return nil
}

func (c *clusterAction) workloadCronJobs(ctx context.Context, cjNames []string, namespace string) []model.ConvertResource {
	return nil
}

func (c *clusterAction) getAppKubernetesResources(ctx context.Context, others model.OtherResource, namespace string) []dbmodel.K8sResource {
	logrus.Infof("getAppKubernetesResources is begin")
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
			services.Status = corev1.ServiceStatus{}
			services.ManagedFields = []metav1.ManagedFieldsEntry{}
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", services)
			if err != nil {
				logrus.Errorf("namespace:%v service:%v error: %v", namespace, services.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:    services.Name,
				Kind:    model.Service,
				Content: kubernetesResourcesYAML,
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
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", pvc)
			if err != nil {
				logrus.Errorf("namespace:%v pvc:%v error: %v", namespace, pvc.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:    pvc.Name,
				Kind:    pvcList.Kind,
				Content: kubernetesResourcesYAML,
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
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", ingresses)
			if err != nil {
				logrus.Errorf("namespace:%v ingresses:%v error: %v", namespace, ingresses.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:    ingresses.Name,
				Kind:    ingressList.Kind,
				Content: kubernetesResourcesYAML,
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
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", networkPolicies)
			if err != nil {
				logrus.Errorf("namespace:%v NetworkPolicies:%v error: %v", namespace, networkPolicies.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:    networkPolicies.Name,
				Kind:    networkPoliciesList.Kind,
				Content: kubernetesResourcesYAML,
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
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", configMaps)
			if err != nil {
				logrus.Errorf("namespace:%v ConfigMaps:%v error: %v", namespace, configMaps.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:    configMaps.Name,
				Kind:    cmList.Kind,
				Content: kubernetesResourcesYAML,
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
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", secrets)
			if err != nil {
				logrus.Errorf("namespace:%v Secrets:%v error: %v", namespace, secrets.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:    secrets.Name,
				Kind:    secretsList.Kind,
				Content: kubernetesResourcesYAML,
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
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", serviceAccounts)
			if err != nil {
				logrus.Errorf("namespace:%v ServiceAccounts:%v error: %v", namespace, serviceAccounts.Name, err)
				continue
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:    serviceAccounts.Name,
				Kind:    serviceAccountsList.Kind,
				Content: kubernetesResourcesYAML,
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
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", roleBindings)
			if err != nil {
				logrus.Errorf("namespace:%v RoleBindings:%v error: %v", namespace, roleBindings.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:    roleBindings.Name,
				Kind:    roleBindingsList.Kind,
				Content: kubernetesResourcesYAML,
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
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", hpa)
			if err != nil {
				logrus.Errorf("namespace:%v HorizontalPodAutoscalers:%v error: %v", namespace, hpa.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:    hpa.Name,
				Kind:    hpaList.Kind,
				Content: kubernetesResourcesYAML,
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
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", roles)
			if err != nil {
				logrus.Errorf("namespace:%v roles:%v error: %v", namespace, roles.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:    roles.Name,
				Kind:    rolesList.Kind,
				Content: kubernetesResourcesYAML,
			})
		}
	}
	logrus.Infof("getAppKubernetesResources is end")
	return k8sResources
}

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/goodrain/rainbond/api/model"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path"
	"sigs.k8s.io/yaml"
	"strconv"
	"strings"
)

func (c *clusterAction) PodTemplateSpecResource(parameter model.YamlResourceParameter) {
	//Port
	var ps []model.PortManagement
	for _, port := range parameter.Template.Spec.Containers[0].Ports {
		if string(port.Protocol) == "UDP" {
			ps = append(ps, model.PortManagement{
				Port:     port.ContainerPort,
				Protocol: "UDP",
				Inner:    false,
				Outer:    false,
			})
		} else {
			ps = append(ps, model.PortManagement{
				Port:     port.ContainerPort,
				Protocol: "TCP",
				Inner:    false,
				Outer:    false,
			})
		}
		logrus.Warningf("Transport protocol type not recognized%v", port.Protocol)
	}

	//ENV
	var envs []model.ENVManagement
	for i := 0; i < len(parameter.Template.Spec.Containers[0].Env); i++ {
		if cm := parameter.Template.Spec.Containers[0].Env[i].ValueFrom; cm == nil {
			envs = append(envs, model.ENVManagement{
				ENVKey:     parameter.Template.Spec.Containers[0].Env[i].Name,
				ENVValue:   parameter.Template.Spec.Containers[0].Env[i].Value,
				ENVExplain: "",
			})
			parameter.Template.Spec.Containers[0].Env = append(parameter.Template.Spec.Containers[0].Env[:i], parameter.Template.Spec.Containers[0].Env[i+1:]...)
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
	cmList, err := c.clientset.CoreV1().ConfigMaps(parameter.Namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to get ConfigMap%v", err)
	}
	for _, cm := range cmList.Items {
		cmMap[cm.Name] = cm
	}
	cmList.Items = append(cmList.Items, parameter.CMs...)
	for _, volume := range parameter.Template.Spec.Volumes {
		if volume.ConfigMap != nil && err == nil {
			cm, _ := cmMap[volume.ConfigMap.Name]
			cmData := cm.Data
			isLog := true
			var index int
			for i, volumeMount := range parameter.Template.Spec.Containers[0].VolumeMounts {
				if volume.Name != volumeMount.Name {
					continue
				}
				isLog = false
				index = i
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
					mode := int32(777)
					if volume.ConfigMap.DefaultMode != nil {
						mode = *volume.ConfigMap.DefaultMode
					}
					if volumeMount.SubPath != "" {
						configs = append(configs, model.ConfigManagement{
							ConfigName:  volumeMount.SubPath,
							ConfigPath:  volumeMount.MountPath,
							ConfigValue: cmData[volumeMount.SubPath],
							Mode:        mode,
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
				continue
			}
			parameter.Template.Spec.Containers[0].VolumeMounts = append(parameter.Template.Spec.Containers[0].VolumeMounts[:index], parameter.Template.Spec.Containers[0].VolumeMounts[index+1:]...)
		}
	}

	//TelescopicManagement
	HPAList, err := c.clientset.AutoscalingV1().HorizontalPodAutoscalers(parameter.Namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to get HorizontalPodAutoscalers list:%v", err)
	}
	HPAList.Items = append(HPAList.Items, parameter.HPAs...)
	var t model.TelescopicManagement
	//这一块就是自动伸缩的对应解析，
	//需要注意的一点是hpa的cpu和memory的阈值设置是通过Annotations["autoscaling.alpha.kubernetes.io/metrics"]字段设置
	//而且它的返回值是个json字符串所以设置了一个结构体进行解析。
	for _, hpa := range HPAList.Items {
		if (hpa.Spec.ScaleTargetRef.Kind != model.Deployment && hpa.Spec.ScaleTargetRef.Kind != model.StateFulSet) || hpa.Spec.ScaleTargetRef.Name != parameter.Name {
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
	livenessProbe := parameter.Template.Spec.Containers[0].LivenessProbe
	if livenessProbe != nil {
		var httpHeaders []string
		if livenessProbe.HTTPGet != nil {
			for _, httpHeader := range livenessProbe.HTTPGet.HTTPHeaders {
				nv := httpHeader.Name + "=" + httpHeader.Value
				httpHeaders = append(httpHeaders, nv)
			}
			hcm.DetectionMethod = strings.ToLower(string(livenessProbe.HTTPGet.Scheme))
			hcm.Path = livenessProbe.HTTPGet.Path
			hcm.Port = int(livenessProbe.HTTPGet.Port.IntVal)
		}
		hcm.Status = 1
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
		readinessProbe := parameter.Template.Spec.Containers[0].ReadinessProbe
		if readinessProbe != nil {
			var httpHeaders []string
			if readinessProbe.HTTPGet != nil {
				for _, httpHeader := range readinessProbe.HTTPGet.HTTPHeaders {
					nv := httpHeader.Name + "=" + httpHeader.Value
					httpHeaders = append(httpHeaders, nv)
				}
				hcm.DetectionMethod = strings.ToLower(string(readinessProbe.HTTPGet.Scheme))
				hcm.Path = readinessProbe.HTTPGet.Path
				hcm.Port = int(readinessProbe.HTTPGet.Port.IntVal)
			}
			hcm.Status = 1
			hcm.Mode = "readiness"
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
	if parameter.Template.Spec.Containers[0].Env != nil && len(parameter.Template.Spec.Containers[0].Env) > 0 {
		envYaml, err := ObjectToJSONORYaml("yaml", parameter.Template.Spec.Containers[0].Env)
		if err != nil {
			logrus.Errorf("deployment:%v env %v", parameter.Name, err)
		}
		envAttributes := &dbmodel.ComponentK8sAttributes{
			Name:           dbmodel.K8sAttributeNameENV,
			SaveType:       "yaml",
			AttributeValue: envYaml,
		}
		attributes = append(attributes, envAttributes)
	}
	if parameter.Template.Spec.Volumes != nil {
		volumesYaml, err := ObjectToJSONORYaml("yaml", parameter.Template.Spec.Volumes)
		if err != nil {
			logrus.Errorf("deployment:%v volumes %v", parameter.Name, err)
		}
		volumesAttributes := &dbmodel.ComponentK8sAttributes{
			Name:           dbmodel.K8sAttributeNameVolumes,
			SaveType:       "yaml",
			AttributeValue: volumesYaml,
		}
		attributes = append(attributes, volumesAttributes)

	}
	if parameter.Template.Spec.Containers[0].VolumeMounts != nil {
		volumeMountsYaml, err := ObjectToJSONORYaml("yaml", parameter.Template.Spec.Containers[0].VolumeMounts)
		if err != nil {
			logrus.Errorf("deployment:%v volumeMounts %v", parameter.Name, err)
		}
		volumeMountsAttributes := &dbmodel.ComponentK8sAttributes{
			Name:           dbmodel.K8sAttributeNameVolumeMounts,
			SaveType:       "yaml",
			AttributeValue: volumeMountsYaml,
		}
		attributes = append(attributes, volumeMountsAttributes)
	}
	if parameter.Template.Spec.ServiceAccountName != "" {
		serviceAccountAttributes := &dbmodel.ComponentK8sAttributes{
			Name:           dbmodel.K8sAttributeNameServiceAccountName,
			SaveType:       "string",
			AttributeValue: parameter.Template.Spec.ServiceAccountName,
		}
		attributes = append(attributes, serviceAccountAttributes)
	}
	if parameter.RsLabel != nil {
		labelsJSON, err := ObjectToJSONORYaml("json", parameter.RsLabel)
		if err != nil {
			logrus.Errorf("deployment:%v labels %v", parameter.Name, err)
		}
		labelsAttributes := &dbmodel.ComponentK8sAttributes{
			Name:           dbmodel.K8sAttributeNameLabels,
			SaveType:       "json",
			AttributeValue: labelsJSON,
		}
		attributes = append(attributes, labelsAttributes)
	}

	if parameter.Template.Spec.NodeSelector != nil {
		NodeSelectorJSON, err := ObjectToJSONORYaml("json", parameter.Template.Spec.NodeSelector)
		if err != nil {
			logrus.Errorf("deployment:%v nodeSelector %v", parameter.Name, err)
		}
		nodeSelectorAttributes := &dbmodel.ComponentK8sAttributes{
			Name:           dbmodel.K8sAttributeNameNodeSelector,
			SaveType:       "json",
			AttributeValue: NodeSelectorJSON,
		}
		attributes = append(attributes, nodeSelectorAttributes)
	}
	if parameter.Template.Spec.Tolerations != nil {
		tolerationsYaml, err := ObjectToJSONORYaml("yaml", parameter.Template.Spec.Tolerations)
		if err != nil {
			logrus.Errorf("deployment:%v tolerations %v", parameter.Name, err)
		}
		tolerationsAttributes := &dbmodel.ComponentK8sAttributes{
			Name:           dbmodel.K8sAttributeNameTolerations,
			SaveType:       "yaml",
			AttributeValue: tolerationsYaml,
		}
		attributes = append(attributes, tolerationsAttributes)
	}
	if parameter.Template.Spec.Affinity != nil {
		affinityYaml, err := ObjectToJSONORYaml("yaml", parameter.Template.Spec.Affinity)
		if err != nil {
			logrus.Errorf("deployment:%v affinity %v", parameter.Name, err)
		}
		affinityAttributes := &dbmodel.ComponentK8sAttributes{
			Name:           dbmodel.K8sAttributeNameAffinity,
			SaveType:       "yaml",
			AttributeValue: affinityYaml,
		}
		attributes = append(attributes, affinityAttributes)
	}
	if securityContext := parameter.Template.Spec.Containers[0].SecurityContext; securityContext != nil && securityContext.Privileged != nil {
		privilegedAttributes := &dbmodel.ComponentK8sAttributes{
			Name:           dbmodel.K8sAttributeNamePrivileged,
			SaveType:       "string",
			AttributeValue: strconv.FormatBool(*securityContext.Privileged),
		}
		attributes = append(attributes, privilegedAttributes)
	}

	*parameter.ComponentsCR = append(*parameter.ComponentsCR, model.ConvertResource{
		ComponentsName:                   parameter.Name,
		BasicManagement:                  parameter.Basic,
		PortManagement:                   ps,
		ENVManagement:                    envs,
		ConfigManagement:                 configs,
		TelescopicManagement:             t,
		HealthyCheckManagement:           hcm,
		ComponentK8sAttributesManagement: attributes,
	})
}

//ObjectToJSONORYaml changeType true is json / yaml
func ObjectToJSONORYaml(changeType string, data interface{}) (string, error) {
	if data == nil {
		return "", nil
	}
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("json serialization failed err:%v", err)
	}
	if changeType == "json" {
		return string(dataJSON), nil
	}
	dataYaml, err := yaml.JSONToYAML(dataJSON)
	if err != nil {
		return "", fmt.Errorf("yaml serialization failed err:%v", err)
	}
	return string(dataYaml), nil
}

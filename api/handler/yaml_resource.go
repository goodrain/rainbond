package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	appv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	yamlt "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"path"
	"path/filepath"
	"strings"
)

// HandleK8SResourceObjectsName -
func HandleK8SResourceObjectsName(k8sResourceObjects []apimodel.K8sResourceObject) map[string]apimodel.LabelResource {
	fileResource := make(map[string]apimodel.LabelResource)
	var DeployNames, JobNames, CJNames, STSNames, RoleNames, HPANames, RBNames, SANames, SecretNames, ServiceNames, CMNames, NetworkPolicyNames, IngressNames, PVCNames []string
	defaultResource := make(map[string][]string)
	for _, k8sResourceObject := range k8sResourceObjects {
		if k8sResourceObject.Error != "" {
			fileResource[k8sResourceObject.FileName] = apimodel.LabelResource{
				Status: k8sResourceObject.Error,
			}
			continue
		}
		for _, buildResource := range k8sResourceObject.BuildResources {
			switch buildResource.Resource.GetKind() {
			case apimodel.Deployment:
				DeployNames = append(DeployNames, buildResource.Resource.GetName())
			case apimodel.Job:
				JobNames = append(JobNames, buildResource.Resource.GetName())
			case apimodel.CronJob:
				CJNames = append(CJNames, buildResource.Resource.GetName())
			case apimodel.StateFulSet:
				STSNames = append(STSNames, buildResource.Resource.GetName())
			case apimodel.Role:
				RoleNames = append(RoleNames, buildResource.Resource.GetName())
			case apimodel.HorizontalPodAutoscaler:
				HPANames = append(HPANames, buildResource.Resource.GetName())
			case apimodel.RoleBinding:
				RBNames = append(RBNames, buildResource.Resource.GetName())
			case apimodel.ServiceAccount:
				SANames = append(SANames, buildResource.Resource.GetName())
			case apimodel.Secret:
				SecretNames = append(SecretNames, buildResource.Resource.GetName())
			case apimodel.Service:
				ServiceNames = append(ServiceNames, buildResource.Resource.GetName())
			case apimodel.ConfigMap:
				CMNames = append(CMNames, buildResource.Resource.GetName())
			case apimodel.NetworkPolicy:
				NetworkPolicyNames = append(NetworkPolicyNames, buildResource.Resource.GetName())
			case apimodel.Ingress:
				IngressNames = append(IngressNames, buildResource.Resource.GetName())
			case apimodel.PVC:
				PVCNames = append(PVCNames, buildResource.Resource.GetName())
			default:
				defaultNames, ok := defaultResource[buildResource.Resource.GetKind()]
				if ok {
					defaultResource[buildResource.Resource.GetKind()] = append(defaultNames, buildResource.Resource.GetName())
				} else {
					defaultResource[buildResource.Resource.GetKind()] = []string{buildResource.Resource.GetName()}
				}
			}
		}
	}
	fileResource["app_resource"] = apimodel.LabelResource{
		UnSupport: defaultResource,
		Workloads: apimodel.WorkLoadsResource{
			Deployments:  DeployNames,
			Jobs:         JobNames,
			CronJobs:     CJNames,
			StateFulSets: STSNames,
		},
		Others: apimodel.OtherResource{
			Services:                 ServiceNames,
			PVC:                      PVCNames,
			Ingresses:                IngressNames,
			NetworkPolicies:          NetworkPolicyNames,
			ConfigMaps:               CMNames,
			Secrets:                  SecretNames,
			ServiceAccounts:          ServiceNames,
			RoleBindings:             RoleNames,
			HorizontalPodAutoscalers: HPANames,
			Roles:                    RoleNames,
		},
		Status: "",
	}
	return fileResource
}

// AppYamlResourceName -
func (c *clusterAction) AppYamlResourceName(yamlResource apimodel.YamlResource) (map[string]apimodel.LabelResource, *util.APIHandleError) {
	logrus.Infof("AppYamlResourceName begin")
	k8sResourceObjects := c.YamlToResource(yamlResource, apimodel.YamlSourceFile, "")
	fileResource := HandleK8SResourceObjectsName(k8sResourceObjects)
	logrus.Infof("AppYamlResourceName end")
	return fileResource, nil
}

// AppYamlResourceDetailed -
func (c *clusterAction) AppYamlResourceDetailed(yamlResource apimodel.YamlResource, yamlImport bool) (apimodel.ApplicationResource, *util.APIHandleError) {
	logrus.Infof("AppYamlResourceDetailed begin")
	source := apimodel.YamlSourceFile
	if yamlResource.Yaml != "" {
		source = apimodel.YamlSourceHelm
	}
	k8sResourceObjects := c.YamlToResource(yamlResource, source, yamlResource.Yaml)
	appResource := HandleDetailResource(yamlResource.Namespace, k8sResourceObjects, yamlImport, c.clientset, c.mapper)
	return appResource, nil
}

// AppYamlResourceImport -
func (c *clusterAction) AppYamlResourceImport(namespace, tenantID, appID string, components apimodel.ApplicationResource) (apimodel.AppComponent, *util.APIHandleError) {
	logrus.Infof("AppYamlResourceImport begin")
	app, err := db.GetManager().ApplicationDao().GetAppByID(appID)
	if err != nil {
		return apimodel.AppComponent{}, &util.APIHandleError{Code: 400, Err: fmt.Errorf("GetAppByID error %v", err)}
	}
	var ar apimodel.AppComponent
	k8sResource, err := c.CreateK8sResource(components.KubernetesResources, app.AppID)
	if err != nil {
		logrus.Errorf("create K8sResources err:%v", err)
		return ar, &util.APIHandleError{Code: 400, Err: fmt.Errorf("create K8sResources err:%v", err)}
	}
	var componentAttributes []apimodel.ComponentAttributes
	existComponents, err := db.GetManager().TenantServiceDao().ListByAppID(app.AppID)
	if err != nil {
		return apimodel.AppComponent{}, &util.APIHandleError{Code: 400, Err: fmt.Errorf("get app service by appID failure: %v", err)}
	}
	for _, componentResource := range components.ConvertResource {
		component, err := c.CreateComponent(app, tenantID, componentResource, namespace, true, existComponents)
		if err != nil {
			logrus.Errorf("%v", err)
			return ar, &util.APIHandleError{Code: 400, Err: fmt.Errorf("create app error:%v", err)}
		}
		c.createENV(componentResource.ENVManagement, component)
		c.createConfig(componentResource.ConfigManagement, component)
		c.createPort(componentResource.PortManagement, component)
		componentResource.TelescopicManagement.RuleID = c.createTelescopic(componentResource.TelescopicManagement, component)
		componentResource.HealthyCheckManagement.ProbeID = c.createHealthyCheck(componentResource.HealthyCheckManagement, component)
		c.createK8sAttributes(componentResource.ComponentK8sAttributesManagement, tenantID, component)
		componentAttributes = append(componentAttributes, apimodel.ComponentAttributes{
			TS:                     component,
			Image:                  componentResource.BasicManagement.Image,
			Cmd:                    componentResource.BasicManagement.Cmd,
			ENV:                    componentResource.ENVManagement,
			Config:                 componentResource.ConfigManagement,
			Port:                   componentResource.PortManagement,
			Telescopic:             componentResource.TelescopicManagement,
			HealthyCheck:           componentResource.HealthyCheckManagement,
			ComponentK8sAttributes: componentResource.ComponentK8sAttributesManagement,
		})
	}
	ar = apimodel.AppComponent{
		App:          app,
		K8sResources: k8sResource,
		Component:    componentAttributes,
	}

	if err != nil {
		return apimodel.AppComponent{}, &util.APIHandleError{Code: 400, Err: fmt.Errorf("app yaml resource import error:%v", err)}
	}
	logrus.Infof("AppYamlResourceImport end")
	return ar, nil
}

// YamlToResource -
func (c *clusterAction) YamlToResource(yamlResource apimodel.YamlResource, yamlSource, yamlContent string) []apimodel.K8sResourceObject {
	yamlDirectoryPath := path.Join("/grdata/package_build/temp/events", yamlResource.EventID, "*")
	yamlFilesPath := []string{apimodel.YamlSourceHelm}
	if yamlSource == apimodel.YamlSourceFile {
		yamlFilesPath, _ = filepath.Glob(yamlDirectoryPath)
	}
	var fileBuildResourceList []apimodel.K8sResourceObject
	for _, yamlFilePath := range yamlFilesPath {
		var fileName string
		yamlFileBytes := []byte(strings.TrimPrefix(yamlContent, "\n"))
		if yamlSource == apimodel.YamlSourceFile {
			fileName = path.Base(yamlFilePath)
			var err error
			yamlFileBytes, err = ioutil.ReadFile(yamlFilePath)
			yamlFileBytes = []byte(strings.TrimPrefix(string(yamlFileBytes), "\n"))
			if err != nil {
				logrus.Errorf("yaml to resource first step failure: %v", err)
				fileBuildResourceList = append(fileBuildResourceList, apimodel.K8sResourceObject{
					FileName:       fileName,
					BuildResources: nil,
					Error:          err.Error(),
				})
				continue
			}
		}
		fileBuildResourceList = handleFileORYamlToObject(fileName, yamlFileBytes, c.config)
	}
	return fileBuildResourceList
}

// ResourceCreate -
func ResourceCreate(buildResource apimodel.BuildResource, namespace string, mapper meta.RESTMapper, clientset *kubernetes.Clientset) (*unstructured.Unstructured, error) {
	logrus.Infof("begin ResourceCreate function")
	mapping, err := mapper.RESTMapping(buildResource.GVK.GroupKind(), buildResource.GVK.Version)
	if err != nil {
		if !meta.IsNoMatchError(err) {
			return nil, err
		}
		mapper, err = RefreshMapper(clientset)
		if err != nil {
			return nil, err
		}
		mapping, err = mapper.RESTMapping(buildResource.GVK.GroupKind(), buildResource.GVK.Version)
		if err != nil {
			return nil, err
		}
	}
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		buildResource.Resource.SetNamespace(namespace)
		buildResource.Dri = buildResource.DC.Resource(mapping.Resource).Namespace(buildResource.Resource.GetNamespace())
	} else {
		buildResource.Dri = buildResource.DC.Resource(mapping.Resource)
	}
	obj, err := buildResource.Dri.Create(context.Background(), buildResource.Resource, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func handleFileORYamlToObject(fileName string, yamlFileBytes []byte, config *rest.Config) []apimodel.K8sResourceObject {
	var fileBuildResourceList []apimodel.K8sResourceObject
	dc, err := dynamic.NewForConfig(config)
	if err != nil {
		logrus.Errorf("yaml to resource second step failure: %v", err)
		fileBuildResourceList = append(fileBuildResourceList, apimodel.K8sResourceObject{
			FileName:       fileName,
			BuildResources: nil,
			Error:          err.Error(),
		})
		return fileBuildResourceList
	}
	decoder := yamlt.NewYAMLOrJSONDecoder(bytes.NewReader(yamlFileBytes), 1000)
	var buildResourceList []apimodel.BuildResource
	for {
		var rawObj runtime.RawExtension
		if err = decoder.Decode(&rawObj); err != nil {
			if err.Error() == "EOF" {
				break
			}
			logrus.Errorf("yaml to resource third step failure: %v", err)
			fileBuildResourceList = append(fileBuildResourceList, apimodel.K8sResourceObject{
				FileName:       fileName,
				BuildResources: nil,
				Error:          err.Error(),
			})
			continue
		}
		obj, gvk, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		if err != nil {
			logrus.Errorf("yaml to resource fourth step failure: %v", err)
			fileBuildResourceList = append(fileBuildResourceList, apimodel.K8sResourceObject{
				FileName:       fileName,
				BuildResources: nil,
				Error:          err.Error(),
			})
			continue
		}
		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			logrus.Errorf("yaml to resource fifth step failure: %v", err)
			fileBuildResourceList = append(fileBuildResourceList, apimodel.K8sResourceObject{
				FileName:       fileName,
				BuildResources: nil,
				Error:          err.Error(),
			})
			continue
		}
		unstructuredObj := &unstructured.Unstructured{Object: unstructuredMap}
		buildResourceList = append(buildResourceList, apimodel.BuildResource{
			Resource:      unstructuredObj,
			State:         apimodel.CreateError,
			ErrorOverview: "",
			Dri:           nil,
			DC:            dc,
			GVK:           gvk,
		})
	}
	fileBuildResourceList = append(fileBuildResourceList, apimodel.K8sResourceObject{
		FileName:       fileName,
		BuildResources: buildResourceList,
		Error:          "",
	})
	return fileBuildResourceList
}

// HandleDetailResource -
func HandleDetailResource(namespace string, k8sResourceObjects []apimodel.K8sResourceObject, yamlImport bool, clientset *kubernetes.Clientset, mapper meta.RESTMapper) apimodel.ApplicationResource {
	var K8SResource []dbmodel.K8sResource
	var ConvertResource []apimodel.ConvertResource
	for _, k8sResourceObject := range k8sResourceObjects {
		if k8sResourceObject.Error != "" {
			continue
		}
		var cms []corev1.ConfigMap
		var hpas []autoscalingv1.HorizontalPodAutoscaler
		for _, buildResource := range k8sResourceObject.BuildResources {
			if buildResource.Resource.GetKind() == apimodel.ConfigMap {
				var cm corev1.ConfigMap
				cmJSON, _ := json.Marshal(buildResource.Resource)
				json.Unmarshal(cmJSON, &cm)
				cms = append(cms, cm)
				continue
			}
			if buildResource.Resource.GetKind() == apimodel.HorizontalPodAutoscaler {
				var hpa autoscalingv1.HorizontalPodAutoscaler
				cmJSON, _ := json.Marshal(buildResource.Resource)
				json.Unmarshal(cmJSON, &hpa)
				hpas = append(hpas, hpa)
			}
		}

		for _, buildResource := range k8sResourceObject.BuildResources {
			errorOverview := "创建成功"
			state := apimodel.CreateSuccess
			switch buildResource.Resource.GetKind() {
			case apimodel.Deployment:
				deployJSON, _ := json.Marshal(buildResource.Resource)
				var deployObject appv1.Deployment
				json.Unmarshal(deployJSON, &deployObject)
				memory, cpu := deployObject.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().Value(), deployObject.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().MilliValue()
				if memory == 0 {
					memory = deployObject.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().Value()
				}
				if cpu == 0 {
					cpu = deployObject.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().MilliValue()
				}
				basic := apimodel.BasicManagement{
					ResourceType: apimodel.Deployment,
					Replicas:     deployObject.Spec.Replicas,
					Memory:       memory / 1024 / 1024,
					CPU:          cpu,
					Image:        deployObject.Spec.Template.Spec.Containers[0].Image,
					Cmd:          strings.Join(append(deployObject.Spec.Template.Spec.Containers[0].Command, deployObject.Spec.Template.Spec.Containers[0].Args...), " "),
				}
				parameter := apimodel.YamlResourceParameter{
					ComponentsCR: &ConvertResource,
					Basic:        basic,
					Template:     deployObject.Spec.Template,
					Namespace:    namespace,
					Name:         buildResource.Resource.GetName(),
					RsLabel:      deployObject.Labels,
					HPAs:         hpas,
					CMs:          cms,
				}
				PodTemplateSpecResource(parameter, nil, clientset)
			case apimodel.Job:
				jobJSON, _ := json.Marshal(buildResource.Resource)
				var jobObject batchv1.Job
				json.Unmarshal(jobJSON, &jobObject)
				memory, cpu := jobObject.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().Value(), jobObject.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().MilliValue()
				if memory == 0 {
					memory = jobObject.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().Value()
				}
				if cpu == 0 {
					cpu = jobObject.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().MilliValue()
				}
				basic := apimodel.BasicManagement{
					ResourceType: apimodel.Job,
					Replicas:     jobObject.Spec.Completions,
					Memory:       memory / 1024 / 1024,
					CPU:          cpu,
					Image:        jobObject.Spec.Template.Spec.Containers[0].Image,
					Cmd:          strings.Join(append(jobObject.Spec.Template.Spec.Containers[0].Command, jobObject.Spec.Template.Spec.Containers[0].Args...), " "),
				}
				parameter := apimodel.YamlResourceParameter{
					ComponentsCR: &ConvertResource,
					Basic:        basic,
					Template:     jobObject.Spec.Template,
					Namespace:    namespace,
					Name:         buildResource.Resource.GetName(),
					RsLabel:      jobObject.Labels,
					HPAs:         hpas,
					CMs:          cms,
				}
				PodTemplateSpecResource(parameter, nil, clientset)
			case apimodel.CronJob:
				cjJSON, _ := json.Marshal(buildResource.Resource)
				var cjObject batchv1.CronJob
				json.Unmarshal(cjJSON, &cjObject)
				memory, cpu := cjObject.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().Value(), cjObject.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().MilliValue()
				if memory == 0 {
					memory = cjObject.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().Value()
				}
				if cpu == 0 {
					cpu = cjObject.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().MilliValue()
				}
				basic := apimodel.BasicManagement{
					ResourceType: apimodel.CronJob,
					Replicas:     cjObject.Spec.JobTemplate.Spec.Completions,
					Memory:       memory / 1024 / 1024,
					CPU:          cpu,
					Image:        cjObject.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image,
					Cmd:          strings.Join(append(cjObject.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Command, cjObject.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Args...), " "),
				}
				parameter := apimodel.YamlResourceParameter{
					ComponentsCR: &ConvertResource,
					Basic:        basic,
					Template:     cjObject.Spec.JobTemplate.Spec.Template,
					Namespace:    namespace,
					Name:         buildResource.Resource.GetName(),
					RsLabel:      cjObject.Labels,
					HPAs:         hpas,
					CMs:          cms,
				}
				PodTemplateSpecResource(parameter, nil, clientset)
			case apimodel.StateFulSet:
				stsJSON, _ := json.Marshal(buildResource.Resource)
				var stsObject appv1.StatefulSet
				json.Unmarshal(stsJSON, &stsObject)
				memory, cpu := stsObject.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().Value(), stsObject.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().MilliValue()
				if memory == 0 {
					memory = stsObject.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().Value()
				}
				if cpu == 0 {
					cpu = stsObject.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().MilliValue()
				}
				basic := apimodel.BasicManagement{
					ResourceType: apimodel.StateFulSet,
					Replicas:     stsObject.Spec.Replicas,
					Memory:       memory / 1024 / 1024,
					CPU:          cpu,
					Image:        stsObject.Spec.Template.Spec.Containers[0].Image,
					Cmd:          strings.Join(append(stsObject.Spec.Template.Spec.Containers[0].Command, stsObject.Spec.Template.Spec.Containers[0].Args...), " "),
				}
				parameter := apimodel.YamlResourceParameter{
					ComponentsCR: &ConvertResource,
					Basic:        basic,
					Template:     stsObject.Spec.Template,
					Namespace:    namespace,
					Name:         buildResource.Resource.GetName(),
					RsLabel:      stsObject.Labels,
					HPAs:         hpas,
					CMs:          cms,
				}
				PodTemplateSpecResource(parameter, stsObject.Spec.VolumeClaimTemplates, clientset)
			default:
				if yamlImport {
					resource, err := ResourceCreate(buildResource, namespace, mapper, clientset)
					if err != nil {
						errorOverview = err.Error()
						state = apimodel.CreateError
					} else {
						buildResource.Resource = resource
					}
				}
				kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", buildResource.Resource)
				if err != nil {
					logrus.Errorf("namespace:%v %v:%v error: %v", namespace, buildResource.Resource.GetKind(), buildResource.Resource.GetName(), err)
					continue
				}
				K8SResource = append(K8SResource, dbmodel.K8sResource{
					Name:          buildResource.Resource.GetName(),
					Kind:          buildResource.Resource.GetKind(),
					Content:       kubernetesResourcesYAML,
					State:         state,
					ErrorOverview: errorOverview,
				})
			}
		}

	}
	logrus.Infof("AppYamlResourceDetailed end")
	return apimodel.ApplicationResource{
		KubernetesResources: K8SResource,
		ConvertResource:     ConvertResource,
	}
}

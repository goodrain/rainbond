package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	api_model "github.com/goodrain/rainbond/api/model"
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
func HandleK8SResourceObjectsName(k8sResourceObjects []api_model.K8sResourceObject) map[string]api_model.LabelResource {
	fileResource := make(map[string]api_model.LabelResource)
	var DeployNames, JobNames, CJNames, STSNames, RoleNames, HPANames, RBNames, SANames, SecretNames, ServiceNames, CMNames, NetworkPolicyNames, IngressNames, PVCNames []string
	defaultResource := make(map[string][]string)
	for _, k8sResourceObject := range k8sResourceObjects {
		if k8sResourceObject.Error != "" {
			fileResource[k8sResourceObject.FileName] = api_model.LabelResource{
				Status: k8sResourceObject.Error,
			}
			continue
		}
		for _, buildResource := range k8sResourceObject.BuildResources {
			switch buildResource.Resource.GetKind() {
			case api_model.Deployment:
				DeployNames = append(DeployNames, buildResource.Resource.GetName())
			case api_model.Job:
				JobNames = append(JobNames, buildResource.Resource.GetName())
			case api_model.CronJob:
				CJNames = append(CJNames, buildResource.Resource.GetName())
			case api_model.StateFulSet:
				STSNames = append(STSNames, buildResource.Resource.GetName())
			case api_model.Role:
				RoleNames = append(RoleNames, buildResource.Resource.GetName())
			case api_model.HorizontalPodAutoscaler:
				HPANames = append(HPANames, buildResource.Resource.GetName())
			case api_model.RoleBinding:
				RBNames = append(RBNames, buildResource.Resource.GetName())
			case api_model.ServiceAccount:
				SANames = append(SANames, buildResource.Resource.GetName())
			case api_model.Secret:
				SecretNames = append(SecretNames, buildResource.Resource.GetName())
			case api_model.Service:
				ServiceNames = append(ServiceNames, buildResource.Resource.GetName())
			case api_model.ConfigMap:
				CMNames = append(CMNames, buildResource.Resource.GetName())
			case api_model.NetworkPolicy:
				NetworkPolicyNames = append(NetworkPolicyNames, buildResource.Resource.GetName())
			case api_model.Ingress:
				IngressNames = append(IngressNames, buildResource.Resource.GetName())
			case api_model.PVC:
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
	fileResource["app_resource"] = api_model.LabelResource{
		UnSupport: defaultResource,
		Workloads: api_model.WorkLoadsResource{
			Deployments:  DeployNames,
			Jobs:         JobNames,
			CronJobs:     CJNames,
			StateFulSets: STSNames,
		},
		Others: api_model.OtherResource{
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
func (c *clusterAction) AppYamlResourceName(yamlResource api_model.YamlResource) (map[string]api_model.LabelResource, *util.APIHandleError) {
	logrus.Infof("AppYamlResourceName begin")
	k8sResourceObjects := c.YamlToResource(yamlResource, api_model.YamlSourceFile, "")
	fileResource := HandleK8SResourceObjectsName(k8sResourceObjects)
	logrus.Infof("AppYamlResourceName end")
	return fileResource, nil
}

// AppYamlResourceDetailed -
func (c *clusterAction) AppYamlResourceDetailed(yamlResource api_model.YamlResource) (api_model.ApplicationResource, *util.APIHandleError) {
	logrus.Infof("AppYamlResourceDetailed begin")
	source := api_model.YamlSourceFile
	if yamlResource.Yaml != "" {
		source = api_model.YamlSourceHelm
	}
	k8sResourceObjects := c.YamlToResource(yamlResource, source, yamlResource.Yaml)
	appResource := HandleDetailResource(yamlResource.Namespace, k8sResourceObjects, c.clientset, c.mapper)
	return appResource, nil
}

// AppYamlResourceImport -
func (c *clusterAction) AppYamlResourceImport(namespace, tenantID, appID string, components api_model.ApplicationResource) (api_model.AppComponent, *util.APIHandleError) {
	logrus.Infof("AppYamlResourceImport begin")
	var k8sResource []dbmodel.K8sResource
	for _, ks := range components.KubernetesResources {
		rri, err := c.AddAppK8SResource(context.Background(), namespace, appID, ks.Content)
		if err != nil || rri == nil || len(rri) == 0 {
			continue
		}
		k8sResource = append(k8sResource, *rri[0])
	}
	app, err := db.GetManager().ApplicationDao().GetAppByID(appID)
	if err != nil {
		return api_model.AppComponent{}, &util.APIHandleError{Code: 400, Err: fmt.Errorf("GetAppByID error %v", err)}
	}
	var ar api_model.AppComponent
	var componentAttributes []api_model.ComponentAttributes
	existComponents, err := db.GetManager().TenantServiceDao().ListByAppID(app.AppID)
	if err != nil {
		return api_model.AppComponent{}, &util.APIHandleError{Code: 400, Err: fmt.Errorf("get app service by appID failure: %v", err)}
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
		componentAttributes = append(componentAttributes, api_model.ComponentAttributes{
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
	ar = api_model.AppComponent{
		App:          app,
		K8sResources: k8sResource,
		Component:    componentAttributes,
	}

	if err != nil {
		return api_model.AppComponent{}, &util.APIHandleError{Code: 400, Err: fmt.Errorf("app yaml resource import error:%v", err)}
	}
	logrus.Infof("AppYamlResourceImport end")
	return ar, nil
}

// YamlToResource -
func (c *clusterAction) YamlToResource(yamlResource api_model.YamlResource, yamlSource, yamlContent string) []api_model.K8sResourceObject {
	yamlDirectoryPath := path.Join("/grdata/package_build/temp/events", yamlResource.EventID, "*")
	yamlFilesPath := []string{api_model.YamlSourceHelm}
	if yamlSource == api_model.YamlSourceFile {
		yamlFilesPath, _ = filepath.Glob(yamlDirectoryPath)
	}
	var fileBuildResourceList []api_model.K8sResourceObject
	for _, yamlFilePath := range yamlFilesPath {
		var fileName string
		yamlFileBytes := []byte(strings.TrimPrefix(yamlContent, "\n"))
		if yamlSource == api_model.YamlSourceFile {
			fileName = path.Base(yamlFilePath)
			var err error
			yamlFileBytes, err = ioutil.ReadFile(yamlFilePath)
			yamlFileBytes = []byte(strings.TrimPrefix(string(yamlFileBytes), "\n"))
			if err != nil {
				logrus.Errorf("yaml to resource first step failure: %v", err)
				fileBuildResourceList = append(fileBuildResourceList, api_model.K8sResourceObject{
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
func ResourceCreate(buildResource api_model.BuildResource, namespace string, mapper meta.RESTMapper, clientset *kubernetes.Clientset) (*unstructured.Unstructured, error) {
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

func handleFileORYamlToObject(fileName string, yamlFileBytes []byte, config *rest.Config) []api_model.K8sResourceObject {
	var fileBuildResourceList []api_model.K8sResourceObject
	dc, err := dynamic.NewForConfig(config)
	if err != nil {
		logrus.Errorf("yaml to resource second step failure: %v", err)
		fileBuildResourceList = append(fileBuildResourceList, api_model.K8sResourceObject{
			FileName:       fileName,
			BuildResources: nil,
			Error:          err.Error(),
		})
		return fileBuildResourceList
	}
	decoder := yamlt.NewYAMLOrJSONDecoder(bytes.NewReader(yamlFileBytes), 1000)
	var buildResourceList []api_model.BuildResource
	for {
		var rawObj runtime.RawExtension
		if err = decoder.Decode(&rawObj); err != nil {
			if err.Error() == "EOF" {
				break
			}
			logrus.Errorf("yaml to resource third step failure: %v", err)
			fileBuildResourceList = append(fileBuildResourceList, api_model.K8sResourceObject{
				FileName:       fileName,
				BuildResources: nil,
				Error:          err.Error(),
			})
			continue
		}
		obj, gvk, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		if err != nil {
			logrus.Errorf("yaml to resource fourth step failure: %v", err)
			fileBuildResourceList = append(fileBuildResourceList, api_model.K8sResourceObject{
				FileName:       fileName,
				BuildResources: nil,
				Error:          err.Error(),
			})
			continue
		}
		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			logrus.Errorf("yaml to resource fifth step failure: %v", err)
			fileBuildResourceList = append(fileBuildResourceList, api_model.K8sResourceObject{
				FileName:       fileName,
				BuildResources: nil,
				Error:          err.Error(),
			})
			continue
		}
		unstructuredObj := &unstructured.Unstructured{Object: unstructuredMap}
		buildResourceList = append(buildResourceList, api_model.BuildResource{
			Resource:      unstructuredObj,
			State:         api_model.CreateError,
			ErrorOverview: "",
			Dri:           nil,
			DC:            dc,
			GVK:           gvk,
		})
	}
	fileBuildResourceList = append(fileBuildResourceList, api_model.K8sResourceObject{
		FileName:       fileName,
		BuildResources: buildResourceList,
		Error:          "",
	})
	return fileBuildResourceList
}

// HandleDetailResource -
func HandleDetailResource(namespace string, k8sResourceObjects []api_model.K8sResourceObject, clientset *kubernetes.Clientset, mapper meta.RESTMapper) api_model.ApplicationResource {
	var K8SResource []dbmodel.K8sResource
	var ConvertResource []api_model.ConvertResource
	for _, k8sResourceObject := range k8sResourceObjects {
		if k8sResourceObject.Error != "" {
			continue
		}
		var cms []corev1.ConfigMap
		var hpas []autoscalingv1.HorizontalPodAutoscaler
		for _, buildResource := range k8sResourceObject.BuildResources {
			if buildResource.Resource.GetKind() == api_model.ConfigMap {
				var cm corev1.ConfigMap
				cmJSON, _ := json.Marshal(buildResource.Resource)
				json.Unmarshal(cmJSON, &cm)
				cms = append(cms, cm)
				continue
			}
			if buildResource.Resource.GetKind() == api_model.HorizontalPodAutoscaler {
				var hpa autoscalingv1.HorizontalPodAutoscaler
				cmJSON, _ := json.Marshal(buildResource.Resource)
				json.Unmarshal(cmJSON, &hpa)
				hpas = append(hpas, hpa)
			}
		}

		for _, buildResource := range k8sResourceObject.BuildResources {
			errorOverview := "创建成功"
			state := api_model.CreateSuccess
			switch buildResource.Resource.GetKind() {
			case api_model.Deployment:
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
				basic := api_model.BasicManagement{
					ResourceType: api_model.Deployment,
					Replicas:     deployObject.Spec.Replicas,
					Memory:       memory / 1024 / 1024,
					CPU:          cpu,
					Image:        deployObject.Spec.Template.Spec.Containers[0].Image,
					Cmd:          strings.Join(append(deployObject.Spec.Template.Spec.Containers[0].Command, deployObject.Spec.Template.Spec.Containers[0].Args...), " "),
				}
				parameter := api_model.YamlResourceParameter{
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
			case api_model.Job:
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
				basic := api_model.BasicManagement{
					ResourceType: api_model.Job,
					Replicas:     jobObject.Spec.Completions,
					Memory:       memory / 1024 / 1024,
					CPU:          cpu,
					Image:        jobObject.Spec.Template.Spec.Containers[0].Image,
					Cmd:          strings.Join(append(jobObject.Spec.Template.Spec.Containers[0].Command, jobObject.Spec.Template.Spec.Containers[0].Args...), " "),
				}
				parameter := api_model.YamlResourceParameter{
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
			case api_model.CronJob:
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
				basic := api_model.BasicManagement{
					ResourceType: api_model.CronJob,
					Replicas:     cjObject.Spec.JobTemplate.Spec.Completions,
					Memory:       memory / 1024 / 1024,
					CPU:          cpu,
					Image:        cjObject.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image,
					Cmd:          strings.Join(append(cjObject.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Command, cjObject.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Args...), " "),
				}
				parameter := api_model.YamlResourceParameter{
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
			case api_model.StateFulSet:
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
				basic := api_model.BasicManagement{
					ResourceType: api_model.StateFulSet,
					Replicas:     stsObject.Spec.Replicas,
					Memory:       memory / 1024 / 1024,
					CPU:          cpu,
					Image:        stsObject.Spec.Template.Spec.Containers[0].Image,
					Cmd:          strings.Join(append(stsObject.Spec.Template.Spec.Containers[0].Command, stsObject.Spec.Template.Spec.Containers[0].Args...), " "),
				}
				parameter := api_model.YamlResourceParameter{
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
	return api_model.ApplicationResource{
		KubernetesResources: K8SResource,
		ConvertResource:     ConvertResource,
	}
}

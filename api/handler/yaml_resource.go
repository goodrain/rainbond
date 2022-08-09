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
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	appv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	yamlt "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"path"
	"path/filepath"
	"strings"
)

//AppYamlResourceName -
func (c *clusterAction) AppYamlResourceName(yamlResource api_model.YamlResource) (map[string]api_model.LabelResource, *util.APIHandleError) {
	logrus.Infof("AppYamlResourceName begin")
	fileResource := make(map[string]api_model.LabelResource)
	k8sResourceObjects := c.YamlToResource(yamlResource)
	var DeployNames, JobNames, CJNames, SFSNames, RoleNames, HPANames, RBNames, SANames, SecretNames, ServiceNames, CMNames, NetworkPolicyNames, IngressNames, PVCNames []string
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
				SFSNames = append(SFSNames, buildResource.Resource.GetName())
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
			StateFulSets: SFSNames,
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
	logrus.Infof("AppYamlResourceName end")
	return fileResource, nil
}

//AppYamlResourceDetailed -
func (c *clusterAction) AppYamlResourceDetailed(yamlResource api_model.YamlResource, yamlImport bool) (api_model.ApplicationResource, *util.APIHandleError) {
	logrus.Infof("AppYamlResourceDetailed begin")
	k8sResourceObjects := c.YamlToResource(yamlResource)
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
			state := 1
			switch buildResource.Resource.GetKind() {
			case api_model.Deployment:
				deployJSON, _ := json.Marshal(buildResource.Resource)
				var deployObject appv1.Deployment
				json.Unmarshal(deployJSON, &deployObject)
				basic := api_model.BasicManagement{
					ResourceType: api_model.Deployment,
					Replicas:     deployObject.Spec.Replicas,
					Memory:       deployObject.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().Value() / 1024 / 1024,
					CPU:          deployObject.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().Value(),
					Image:        deployObject.Spec.Template.Spec.Containers[0].Image,
					Cmd:          strings.Join(append(deployObject.Spec.Template.Spec.Containers[0].Command, deployObject.Spec.Template.Spec.Containers[0].Args...), " "),
				}
				parameter := api_model.YamlResourceParameter{
					ComponentsCR: &ConvertResource,
					Basic:        basic,
					Template:     deployObject.Spec.Template,
					Namespace:    yamlResource.Namespace,
					Name:         buildResource.Resource.GetName(),
					RsLabel:      deployObject.Labels,
					HPAs:         hpas,
					CMs:          cms,
				}
				c.PodTemplateSpecResource(parameter)
			case api_model.Job:
				jobJSON, _ := json.Marshal(buildResource.Resource)
				var jobObject batchv1.Job
				json.Unmarshal(jobJSON, &jobObject)
				basic := api_model.BasicManagement{
					ResourceType: api_model.Job,
					Replicas:     jobObject.Spec.Completions,
					Memory:       jobObject.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().Value() / 1024 / 1024,
					CPU:          jobObject.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().Value(),
					Image:        jobObject.Spec.Template.Spec.Containers[0].Image,
					Cmd:          strings.Join(append(jobObject.Spec.Template.Spec.Containers[0].Command, jobObject.Spec.Template.Spec.Containers[0].Args...), " "),
				}
				parameter := api_model.YamlResourceParameter{
					ComponentsCR: &ConvertResource,
					Basic:        basic,
					Template:     jobObject.Spec.Template,
					Namespace:    yamlResource.Namespace,
					Name:         buildResource.Resource.GetName(),
					RsLabel:      jobObject.Labels,
					HPAs:         hpas,
					CMs:          cms,
				}
				c.PodTemplateSpecResource(parameter)
			case api_model.CronJob:
				cjJSON, _ := json.Marshal(buildResource.Resource)
				var cjObject v1beta1.CronJob
				json.Unmarshal(cjJSON, &cjObject)
				basic := api_model.BasicManagement{
					ResourceType: api_model.CronJob,
					Replicas:     cjObject.Spec.JobTemplate.Spec.Completions,
					Memory:       cjObject.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().Value() / 1024 / 1024,
					CPU:          cjObject.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().Value(),
					Image:        cjObject.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image,
					Cmd:          strings.Join(append(cjObject.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Command, cjObject.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Args...), " "),
				}
				parameter := api_model.YamlResourceParameter{
					ComponentsCR: &ConvertResource,
					Basic:        basic,
					Template:     cjObject.Spec.JobTemplate.Spec.Template,
					Namespace:    yamlResource.Namespace,
					Name:         buildResource.Resource.GetName(),
					RsLabel:      cjObject.Labels,
					HPAs:         hpas,
					CMs:          cms,
				}
				c.PodTemplateSpecResource(parameter)
			case api_model.StateFulSet:
				sfsJSON, _ := json.Marshal(buildResource.Resource)
				var sfsObject appv1.StatefulSet
				json.Unmarshal(sfsJSON, &sfsObject)
				basic := api_model.BasicManagement{
					ResourceType: api_model.StateFulSet,
					Replicas:     sfsObject.Spec.Replicas,
					Memory:       sfsObject.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().Value() / 1024 / 1024,
					CPU:          sfsObject.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().Value(),
					Image:        sfsObject.Spec.Template.Spec.Containers[0].Image,
					Cmd:          strings.Join(append(sfsObject.Spec.Template.Spec.Containers[0].Command, sfsObject.Spec.Template.Spec.Containers[0].Args...), " "),
				}
				parameter := api_model.YamlResourceParameter{
					ComponentsCR: &ConvertResource,
					Basic:        basic,
					Template:     sfsObject.Spec.Template,
					Namespace:    yamlResource.Namespace,
					Name:         buildResource.Resource.GetName(),
					RsLabel:      sfsObject.Labels,
					HPAs:         hpas,
					CMs:          cms,
				}
				c.PodTemplateSpecResource(parameter)
			default:
				if yamlImport {
					resource, err := c.ResourceCreate(buildResource, yamlResource.Namespace)
					if err != nil {
						errorOverview = err.Error()
						state = api_model.CreateError
					} else {
						buildResource.Resource = resource
					}
				}
				kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", buildResource.Resource)
				if err != nil {
					logrus.Errorf("namespace:%v %v:%v error: %v", yamlResource.Namespace, buildResource.Resource.GetKind(), buildResource.Resource.GetName(), err)
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
		K8SResource,
		ConvertResource,
	}, nil
}

//AppYamlResourceImport -
func (c *clusterAction) AppYamlResourceImport(yamlResource api_model.YamlResource, components api_model.ApplicationResource) (api_model.AppComponent, *util.APIHandleError) {
	logrus.Infof("AppYamlResourceImport begin")
	app, err := db.GetManager().ApplicationDao().GetAppByID(yamlResource.AppID)
	if err != nil {
		return api_model.AppComponent{}, &util.APIHandleError{Code: 400, Err: fmt.Errorf("GetAppByID error %v", err)}
	}
	var ar api_model.AppComponent
	err = db.GetManager().DB().Transaction(func(tx *gorm.DB) error {
		k8sResource, err := c.CreateK8sResource(tx, components.KubernetesResources, app.AppID)
		if err != nil {
			logrus.Errorf("create K8sResources err:%v", err)
			return &util.APIHandleError{Code: 400, Err: fmt.Errorf("create K8sResources err:%v", err)}
		}
		var componentAttributes []api_model.ComponentAttributes
		for _, componentResource := range components.ConvertResource {
			component, err := c.CreateComponent(app, yamlResource.TenantID, componentResource, yamlResource.Namespace, true)
			if err != nil {
				logrus.Errorf("%v", err)
				return &util.APIHandleError{Code: 400, Err: fmt.Errorf("create app error:%v", err)}
			}
			c.createENV(componentResource.ENVManagement, component)
			c.createConfig(componentResource.ConfigManagement, component)
			c.createPort(componentResource.PortManagement, component)
			componentResource.TelescopicManagement.RuleID = c.createTelescopic(componentResource.TelescopicManagement, component)
			componentResource.HealthyCheckManagement.ProbeID = c.createHealthyCheck(componentResource.HealthyCheckManagement, component)
			c.createK8sAttributes(componentResource.ComponentK8sAttributesManagement, yamlResource.TenantID, component)
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
		return nil
	})
	if err != nil {
		return api_model.AppComponent{}, &util.APIHandleError{Code: 400, Err: fmt.Errorf("app yaml resource import error:%v", err)}
	}
	logrus.Infof("AppYamlResourceImport end")
	return ar, nil
}

//YamlToResource -
func (c *clusterAction) YamlToResource(yamlResource api_model.YamlResource) []api_model.K8sResourceObject {
	yamlDirectoryPath := path.Join("/grdata/package_build/temp/events", yamlResource.EventID, "*")
	yamlFilesPath, _ := filepath.Glob(yamlDirectoryPath)
	var fileBuildResourceList []api_model.K8sResourceObject
	for _, yamlFilePath := range yamlFilesPath {
		pathSplitList := strings.Split(yamlFilePath, "/")
		fileName := pathSplitList[len(pathSplitList)-1]
		yamlFileBytes, err := ioutil.ReadFile(yamlFilePath)
		if err != nil {
			logrus.Errorf("%v", err)
			fileBuildResourceList = append(fileBuildResourceList, api_model.K8sResourceObject{
				FileName:       fileName,
				BuildResources: nil,
				Error:          err.Error(),
			})
			continue
		}
		dc, err := dynamic.NewForConfig(c.config)
		if err != nil {
			logrus.Errorf("%v", err)
			fileBuildResourceList = append(fileBuildResourceList, api_model.K8sResourceObject{
				FileName:       fileName,
				BuildResources: nil,
				Error:          err.Error(),
			})
			continue
		}
		decoder := yamlt.NewYAMLOrJSONDecoder(bytes.NewReader(yamlFileBytes), 1000)
		var buildResourceList []api_model.BuildResource
		for {
			var rawObj runtime.RawExtension
			if err = decoder.Decode(&rawObj); err != nil {
				if err.Error() == "EOF" {
					break
				}
				logrus.Errorf("%v", err)
				fileBuildResourceList = append(fileBuildResourceList, api_model.K8sResourceObject{
					FileName:       fileName,
					BuildResources: nil,
					Error:          err.Error(),
				})
				break
			}
			obj, gvk, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
			if err != nil {
				logrus.Errorf("%v", err)
				fileBuildResourceList = append(fileBuildResourceList, api_model.K8sResourceObject{
					FileName:       fileName,
					BuildResources: nil,
					Error:          err.Error(),
				})
				break
			}
			unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
			if err != nil {
				logrus.Errorf("%v", err)
				fileBuildResourceList = append(fileBuildResourceList, api_model.K8sResourceObject{
					FileName:       fileName,
					BuildResources: nil,
					Error:          err.Error(),
				})
				break
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
	}
	return fileBuildResourceList
}

//ResourceCreate -
func (c *clusterAction) ResourceCreate(buildResource api_model.BuildResource, namespace string) (*unstructured.Unstructured, error) {
	mapping, err := c.mapper.RESTMapping(buildResource.GVK.GroupKind(), buildResource.GVK.Version)
	if err != nil {
		logrus.Errorf("%v", err)
		return nil, err
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

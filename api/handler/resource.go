package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	yamlt "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	"strings"
)

// AddAppK8SResource -
func (c *clusterAction) AddAppK8SResource(ctx context.Context, namespace string, appID string, resourceYaml string) ([]*dbmodel.K8sResource, *util.APIHandleError) {
	resourceObjects := c.HandleResourceYaml([]byte(strings.TrimPrefix(resourceYaml, "\n")), namespace, "create", "", map[string]string{"app_id": appID})
	var resourceList []*dbmodel.K8sResource
	for _, resourceObject := range resourceObjects {
		resource := resourceObject
		if resourceObject.State == model.CreateError {
			rsYaml := resourceYaml
			if resourceObject.Resource != nil {
				rsYaml, _ = ObjectToJSONORYaml("yaml", resourceObject.Resource)
			}
			resourceList = append(resourceList, &dbmodel.K8sResource{
				AppID:         appID,
				Name:          "未识别",
				Kind:          "未识别",
				Content:       rsYaml,
				ErrorOverview: resource.ErrorOverview,
				State:         resource.State,
			})
		} else {
			resourceObject.Resource = c.ResourceProcessing(resourceObject.Resource, namespace)
			rsYaml, _ := ObjectToJSONORYaml("yaml", resourceObject.Resource)
			resourceList = append(resourceList, &dbmodel.K8sResource{
				AppID:         appID,
				Name:          resource.Resource.GetName(),
				Kind:          resource.Resource.GetKind(),
				Content:       rsYaml,
				ErrorOverview: resource.ErrorOverview,
				State:         resource.State,
			})
			err := db.GetManager().K8sResourceDao().CreateK8sResource(resourceList)
			if err != nil {
				return nil, &util.APIHandleError{Code: 400, Err: fmt.Errorf("CreateK8sResource %v", err)}
			}
		}
	}
	return resourceList, nil
}

func (c *clusterAction) GetAppK8SResource(ctx context.Context, namespace, appID, name, resourceYaml, kind string) (dbmodel.K8sResource, *util.APIHandleError) {
	rs, err := db.GetManager().K8sResourceDao().GetK8sResourceByName(appID, name, kind)
	if err != nil {
		return dbmodel.K8sResource{}, &util.APIHandleError{Code: 400, Err: fmt.Errorf("get k8s resource %v", err)}
	}
	resourceObjects := c.HandleResourceYaml([]byte(rs.Content), namespace, "get", name, nil)

	if resourceObjects[0].State != model.GetError {
		rs.Content, _ = ObjectToJSONORYaml("yaml", resourceObjects[0].Resource)
	}

	db.GetManager().K8sResourceDao().UpdateModel(&rs)
	return rs, nil
}

// UpdateAppK8SResource -
func (c *clusterAction) UpdateAppK8SResource(ctx context.Context, namespace, appID, name, resourceYaml, kind string) (dbmodel.K8sResource, *util.APIHandleError) {
	rs, err := db.GetManager().K8sResourceDao().GetK8sResourceByName(appID, name, kind)
	if err != nil {
		return dbmodel.K8sResource{}, &util.APIHandleError{Code: 400, Err: fmt.Errorf("get k8s resource %v", err)}
	}
	resourceObjects := c.HandleResourceYaml([]byte(resourceYaml), namespace, "update", name, map[string]string{"app_id": appID})
	var rsYaml string
	if resourceObjects[0].State == model.UpdateError {
		rsYaml = resourceYaml
		rs.State = resourceObjects[0].State
		rs.ErrorOverview = resourceObjects[0].ErrorOverview
		rs.Content = rsYaml
	} else {
		rsYaml, _ = ObjectToJSONORYaml("yaml", resourceObjects[0].Resource)
		rs.State = resourceObjects[0].State
		rs.ErrorOverview = resourceObjects[0].ErrorOverview
		rs.Content = rsYaml
		db.GetManager().K8sResourceDao().UpdateModel(&rs)
	}
	return rs, nil
}

// DeleteAppK8SResource -
func (c *clusterAction) DeleteAppK8SResource(ctx context.Context, namespace, appID, name, resourceYaml, kind string) {
	c.HandleResourceYaml([]byte(resourceYaml), namespace, "delete", name, nil)
}

// SyncAppK8SResources -
func (c *clusterAction) SyncAppK8SResources(ctx context.Context, req *model.SyncResources) ([]*dbmodel.K8sResource, *util.APIHandleError) {
	// Only Add
	var resourceList []*dbmodel.K8sResource
	for _, k8sResource := range req.K8sResources {
		resourceObjects := c.HandleResourceYaml([]byte(k8sResource.ResourceYaml), k8sResource.Namespace, "re-create", k8sResource.Name, map[string]string{"app_id": k8sResource.AppID})
		if len(resourceObjects) > 1 {
			logrus.Warningf("SyncAppK8SResources resourceObjects [%s] too much, ignore it", k8sResource.Name)
			continue
		}
		if len(resourceObjects) == 1 {
			rsYaml := k8sResource.ResourceYaml
			if resourceObjects[0].Resource != nil {
				rsYaml, _ = ObjectToJSONORYaml("yaml", resourceObjects[0].Resource)
			}
			resourceList = append(resourceList, &dbmodel.K8sResource{
				AppID:         k8sResource.AppID,
				Name:          k8sResource.Name,
				Kind:          k8sResource.Kind,
				Content:       rsYaml,
				ErrorOverview: resourceObjects[0].ErrorOverview,
				State:         resourceObjects[0].State,
			})
		}
	}
	err := db.GetManager().K8sResourceDao().CreateK8sResource(resourceList)
	if err != nil {
		return nil, &util.APIHandleError{Code: 400, Err: fmt.Errorf("SyncK8sResource %v", err)}
	}
	return resourceList, nil
}

// RefreshMapper -
func (c *clusterAction) RefreshMapper() error {
	gr, err := restmapper.GetAPIGroupResources(c.clientset)
	if err != nil {
		return err
	}
	c.mapper = restmapper.NewDiscoveryRESTMapper(gr)
	return nil
}

// HandleResourceYaml -
func (c *clusterAction) HandleResourceYaml(resourceYaml []byte, namespace string, change string, name string, commonLabels map[string]string) []*model.BuildResource {
	var buildResourceList []*model.BuildResource
	var state int
	if change == "create" || change == "re-create" {
		state = model.CreateError
	} else if change == "update" {
		state = model.UpdateError
	}

	addLabelsFunc := func(unstructuredObj *unstructured.Unstructured) {
		if commonLabels != nil && unstructuredObj != nil {
			labels := unstructuredObj.GetLabels()
			if labels == nil {
				labels = make(map[string]string)
			}
			for k, v := range commonLabels {
				labels[k] = v
			}
			unstructuredObj.SetLabels(labels)
		}
	}

	dc, err := dynamic.NewForConfig(c.config)
	if err != nil {
		logrus.Errorf("HandleResourceYaml dynamic.NewForConfig error %v", err)
		buildResourceList = []*model.BuildResource{{
			State:         state,
			ErrorOverview: err.Error(),
		}}
		return buildResourceList
	}
	resourceYamlByte := resourceYaml

	decoder := yamlt.NewYAMLOrJSONDecoder(bytes.NewReader(resourceYamlByte), 1000)
	for {
		var rawObj runtime.RawExtension
		if err = decoder.Decode(&rawObj); err != nil {
			if err.Error() == "EOF" {
				break
			}
			logrus.Errorf("HandleResourceYaml decoder.Decode error %v", err)
			buildResourceList = []*model.BuildResource{{
				State:         state,
				ErrorOverview: err.Error(),
			}}
			return buildResourceList
		}
		obj, gvk, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		if err != nil {
			logrus.Errorf("HandleResourceYaml yaml.NewDecodingSerializer error %v", err)
			buildResourceList = []*model.BuildResource{{
				State:         state,
				ErrorOverview: err.Error(),
			}}
			return buildResourceList
		}
		//转化成map
		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			logrus.Errorf("HandleResourceYaml runtime.DefaultUnstructuredConverter.ToUnstructured error %v", err)
			buildResourceList = []*model.BuildResource{{
				State:         state,
				ErrorOverview: err.Error(),
			}}
			return buildResourceList
		}
		//转化成对象
		unstructuredObj := unstructured.Unstructured{Object: unstructuredMap}
		mapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			buildResourceList = []*model.BuildResource{{
				State:         state,
				ErrorOverview: err.Error(),
			}}
			if !meta.IsNoMatchError(err) {
				return buildResourceList
			}
			err = c.RefreshMapper()
			if err != nil {
				return buildResourceList
			}
			mapping, err = c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if err != nil {
				return buildResourceList
			}
			buildResourceList = []*model.BuildResource{}
		}
		var dri dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			unstructuredObj.SetNamespace(namespace)
			dri = dc.Resource(mapping.Resource).Namespace(unstructuredObj.GetNamespace())
		} else {
			dri = dc.Resource(mapping.Resource)
		}
		br := &model.BuildResource{
			Resource: &unstructuredObj,
			Dri:      dri,
		}
		buildResourceList = append(buildResourceList, br)
	}
	for _, buildResource := range buildResourceList {
		unstructuredObj := buildResource.Resource
		switch change {
		case "get":
			obj, err := buildResource.Dri.Get(context.TODO(), unstructuredObj.GetName(), metav1.GetOptions{})
			if err != nil {
				logrus.Errorf("get k8s resource error %v", err)
				buildResource.State = model.GetError
			}
			obj.SetManagedFields(nil)
			buildResource.Resource = obj
		case "re-create":
			unstructuredObj.SetResourceVersion("")
			unstructuredObj.SetCreationTimestamp(metav1.Time{})
			unstructuredObj.SetUID("")
			fallthrough
		case "create":
			unstructuredObj = c.ResourceProcessing(unstructuredObj, namespace)
			addLabelsFunc(unstructuredObj)
			obj, err := buildResource.Dri.Create(context.TODO(), unstructuredObj, metav1.CreateOptions{})
			if err != nil {
				logrus.Errorf("k8s resource create error %v", err)
				buildResource.Resource = unstructuredObj
				buildResource.State = state
				buildResource.ErrorOverview = err.Error()
			} else {
				obj.SetManagedFields(nil)
				buildResource.Resource = obj
				buildResource.State = model.CreateSuccess
				buildResource.ErrorOverview = fmt.Sprintf("创建成功")
			}
		case "delete":
			err := buildResource.Dri.Delete(context.TODO(), name, metav1.DeleteOptions{})
			if err != nil {
				logrus.Errorf("delete k8s resource error %v", err)
			}
		case "update":
			addLabelsFunc(unstructuredObj)
			obj, err := buildResource.Dri.Update(context.TODO(), unstructuredObj, metav1.UpdateOptions{})
			if err != nil {
				logrus.Errorf("update k8s resource error %v", err)
				buildResource.Resource = unstructuredObj
				buildResource.State = state
				buildResource.ErrorOverview = err.Error()
			} else {
				obj.SetManagedFields(nil)
				buildResource.Resource = obj
				buildResource.State = model.UpdateSuccess
				buildResource.ErrorOverview = fmt.Sprintf("更新成功")
			}
		}
	}
	return buildResourceList
}

// ResourceProcessing -
func (c *clusterAction) ResourceProcessing(unstructuredObj *unstructured.Unstructured, namespace string) *unstructured.Unstructured {
	if unstructuredObj.GetKind() == model.RoleBinding {
		var rb v1.RoleBinding
		var subjects []v1.Subject
		rbJSON, _ := json.Marshal(unstructuredObj)
		_ = json.Unmarshal(rbJSON, &rb)
		for _, subject := range rb.Subjects {
			if subject.Namespace != "" {
				subject.Namespace = namespace
				subjects = append(subjects, subject)
			}
		}
		rb.Subjects = subjects
		unstructuredMap, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(&rb)
		unstructuredObj.Object = unstructuredMap
	}
	if unstructuredObj.GetKind() == model.ClusterRoleBinding {
		var crb v1.ClusterRoleBinding
		var subjects []v1.Subject
		crbJSON, _ := json.Marshal(unstructuredObj)
		_ = json.Unmarshal(crbJSON, &crb)
		for _, subject := range crb.Subjects {
			if subject.Namespace != "" {
				subject.Namespace = namespace
				subjects = append(subjects, subject)
			}
		}
		crb.Subjects = subjects
		unstructuredMap, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(&crb)
		unstructuredObj.Object = unstructuredMap
	}
	if unstructuredObj.GetKind() == model.Service {
		var service corev1.Service
		serviceJSON, _ := json.Marshal(unstructuredObj)
		_ = json.Unmarshal(serviceJSON, &service)
		service.Spec.ClusterIP = ""
		service.Spec.ClusterIPs = nil
		unstructuredMap, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(&service)
		unstructuredObj.Object = unstructuredMap
		return unstructuredObj
	}
	return unstructuredObj
}

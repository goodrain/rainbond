package handler

import (
	"bytes"
	"context"
	"fmt"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	yamlt "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
)

//AddAppK8SResource -
func (c *clusterAction) AddAppK8SResource(ctx context.Context, namespace string, appID string, resourceYaml string) ([]*dbmodel.K8sResource, *util.APIHandleError) {
	logrus.Info("begin AddAppK8SResource")
	resourceObjects := c.HandleResourceYaml([]byte(resourceYaml), namespace, "create", "")
	var resourceList []*dbmodel.K8sResource
	for _, resourceObject := range resourceObjects {
		resource := resourceObject
		if resourceObject.Success == 3 {
			rsYaml := resourceYaml
			if resourceObject.Resource != nil {
				rsYaml, _ = ObjectToJSONORYaml("yaml", resourceObject.Resource)
			}
			resourceList = append(resourceList, &dbmodel.K8sResource{
				AppID:   appID,
				Name:    "未识别",
				Kind:    "未识别",
				Content: rsYaml,
				Status:  resource.Status,
				Success: resource.Success,
			})
		} else {
			rsYaml, _ := ObjectToJSONORYaml("yaml", resourceObject.Resource)
			resourceList = append(resourceList, &dbmodel.K8sResource{
				AppID:   appID,
				Name:    resource.Resource.GetName(),
				Kind:    resource.Resource.GetKind(),
				Content: rsYaml,
				Status:  resource.Status,
				Success: resource.Success,
			})
			err := db.GetManager().K8sResourceDao().CreateK8sResourceInBatch(resourceList)
			if err != nil {
				return nil, &util.APIHandleError{Code: 400, Err: fmt.Errorf("CreateK8sResource %v", err)}
			}
		}
	}
	return resourceList, nil
}

//UpdateAppK8SResource -
func (c *clusterAction) UpdateAppK8SResource(ctx context.Context, namespace, appID, name, resourceYaml, kind string) (dbmodel.K8sResource, *util.APIHandleError) {
	logrus.Info("begin UpdateAppK8SResource")
	rs, err := db.GetManager().K8sResourceDao().GetK8sResourceByNameInBatch(appID, name, kind)
	if err != nil {
		return dbmodel.K8sResource{}, &util.APIHandleError{Code: 400, Err: fmt.Errorf("get k8s resource %v", err)}
	}
	resourceObjects := c.HandleResourceYaml([]byte(resourceYaml), namespace, "update", name)
	var rsYaml string
	if resourceObjects[0].Success == 4 {
		rsYaml = resourceYaml
		rs[0].Success = resourceObjects[0].Success
		rs[0].Status = resourceObjects[0].Status
		rs[0].Content = rsYaml
		db.GetManager().K8sResourceDao().UpdateModel(&rs[0])
	} else {
		rsYaml, _ = ObjectToJSONORYaml("yaml", resourceObjects[0].Resource)
		rs[0].Success = resourceObjects[0].Success
		rs[0].Status = resourceObjects[0].Status
		rs[0].Content = rsYaml
		db.GetManager().K8sResourceDao().UpdateModel(&rs[0])
	}
	return rs[0], nil
}

//DeleteAppK8SResource -
func (c *clusterAction) DeleteAppK8SResource(ctx context.Context, namespace, appID, name, resourceYaml, kind string) *util.APIHandleError {
	logrus.Info("begin DeleteAppK8SResource")
	c.HandleResourceYaml([]byte(resourceYaml), namespace, "delete", name)
	err := db.GetManager().K8sResourceDao().DeleteK8sResourceInBatch(appID, name, kind)
	if err != nil {
		return &util.APIHandleError{Code: 400, Err: fmt.Errorf("DeleteAppK8SResource %v", err)}
	}
	return nil
}

//BuildResource -
type BuildResource struct {
	Resource *unstructured.Unstructured
	Success  int
	Status   string
	Dri      dynamic.ResourceInterface
	DC       dynamic.Interface
	GVK      *schema.GroupVersionKind
}

//HandleResourceYaml -
func (c *clusterAction) HandleResourceYaml(resourceYaml []byte, namespace string, change string, name string) []*BuildResource {
	var buildResourceList []*BuildResource
	var status string
	var success int
	if change == "create" {
		status = "创建失败"
		success = model.CreateError
	} else if change == "update" {
		status = "更新失败"
		success = model.UpdateError
	}
	dc, err := dynamic.NewForConfig(c.config)
	if err != nil {
		logrus.Errorf("%v", err)
		buildResourceList = []*BuildResource{{
			Success: success,
			Status:  fmt.Sprintf("%v%v", status, err),
		}}
		return buildResourceList
	}
	resourceYamlByte := resourceYaml
	if err != nil {
		logrus.Errorf("%v", err)
		buildResourceList = []*BuildResource{{
			Success: success,
			Status:  fmt.Sprintf("%v%v", status, err),
		}}
		return buildResourceList
	}
	decoder := yamlt.NewYAMLOrJSONDecoder(bytes.NewReader(resourceYamlByte), 1000)
	for {
		var rawObj runtime.RawExtension
		if err = decoder.Decode(&rawObj); err != nil {
			if err.Error() == "EOF" {
				break
			}
			logrus.Errorf("%v", err)
			buildResourceList = []*BuildResource{{
				Success: success,
				Status:  fmt.Sprintf("%v%v", status, err),
			}}
			return buildResourceList
		}
		obj, gvk, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		if err != nil {
			logrus.Errorf("%v", err)
			buildResourceList = []*BuildResource{{
				Success: success,
				Status:  fmt.Sprintf("%v%v", status, err),
			}}
			return buildResourceList
		}
		//转化成map
		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			logrus.Errorf("%v", err)
			buildResourceList = []*BuildResource{{
				Success: success,
				Status:  fmt.Sprintf("%v%v", status, err),
			}}
			return buildResourceList
		}
		//转化成对象
		unstructuredObj := &unstructured.Unstructured{Object: unstructuredMap}
		gr, err := restmapper.GetAPIGroupResources(c.clientset.Discovery())
		if err != nil {
			logrus.Errorf("%v", err)
			buildResourceList = append(buildResourceList, &BuildResource{
				Success: success,
				Status:  fmt.Sprintf("%v%v", status, err),
			})
			continue
		}
		mapper := restmapper.NewDiscoveryRESTMapper(gr)
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			logrus.Errorf("%v", err)
			buildResourceList = []*BuildResource{{
				Success: success,
				Status:  fmt.Sprintf("%v%v", status, err),
			}}
			return buildResourceList
		}
		var dri dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			unstructuredObj.SetNamespace(namespace)
			dri = dc.Resource(mapping.Resource).Namespace(unstructuredObj.GetNamespace())
		} else {
			dri = dc.Resource(mapping.Resource)
		}
		br := &BuildResource{
			Resource: unstructuredObj,
			Dri:      dri,
		}
		buildResourceList = append(buildResourceList, br)
	}
	for _, buildResource := range buildResourceList {
		unstructuredObj := buildResource.Resource
		switch change {
		case "create":
			obj, err := buildResource.Dri.Create(context.TODO(), unstructuredObj, metav1.CreateOptions{})
			if err != nil {
				logrus.Errorf("k8s resource create error%v", err)
				buildResource.Resource = unstructuredObj
				buildResource.Success = success
				buildResource.Status = fmt.Sprintf("%v%v", status, err)
			} else {
				buildResource.Resource = obj
				buildResource.Success = model.CreateSuccess
				buildResource.Status = fmt.Sprintf("创建成功")
			}
		case "delete":
			err := buildResource.Dri.Delete(context.TODO(), name, metav1.DeleteOptions{})
			if err != nil {
				logrus.Errorf("delete k8s resource error%v", err)
			}
		case "update":
			obj, err := buildResource.Dri.Update(context.TODO(), unstructuredObj, metav1.UpdateOptions{})
			if err != nil {
				logrus.Errorf("update k8s resource error%v", err)
				buildResource.Resource = unstructuredObj
				buildResource.Success = success
				buildResource.Status = fmt.Sprintf("%v%v", status, err)
			} else {
				buildResource.Resource = obj
				buildResource.Success = model.UpdateSuccess
				buildResource.Status = fmt.Sprintf("更新成功")
			}
		}
	}
	return buildResourceList
}

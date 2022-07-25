package handler

import (
	"bytes"
	"context"
	"fmt"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	yamlt "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
)

func (c *clusterAction) AddAppK8SResource(ctx context.Context, namespace string, appID string, resourceYaml string) ([]*dbmodel.K8sResource, *util.APIHandleError) {
	logrus.Info("begin AddAppK8SResource")
	resourceObjects, err := c.HandleResourceYaml(resourceYaml, namespace, "create", "")
	if err != nil {
		return nil, &util.APIHandleError{Code: 400, Err: fmt.Errorf("failed to parse yaml into k8s resource:%v", err)}
	}
	var resourceList []*dbmodel.K8sResource
	for _, resourceObject := range resourceObjects {
		resource := resourceObject
		resourceYaml, err := ObjectToJSONORYaml("yaml", resource.Resource)
		if err != nil {
			resourceList = append(resourceList, &dbmodel.K8sResource{
				AppID:   appID,
				Name:    resource.Resource.GetName(),
				Kind:    resource.Resource.GetKind(),
				Content: resourceYaml,
				Status:  resource.Status,
				Success: resource.Success,
			})
		}
		resourceList = append(resourceList, &dbmodel.K8sResource{
			AppID:   appID,
			Name:    resource.Resource.GetName(),
			Kind:    resource.Resource.GetKind(),
			Content: resourceYaml,
			Status:  resource.Status,
			Success: resource.Success,
		})
	}
	err = db.GetManager().K8sResourceDao().CreateK8sResourceInBatch(resourceList)
	if err != nil {
		return nil, &util.APIHandleError{Code: 400, Err: fmt.Errorf("CreateK8sResource %v", err)}
	}
	return resourceList, nil
}

func (c *clusterAction) UpdateAppK8SResource(ctx context.Context, namespace string, appID string, name string, resourceYaml string) (dbmodel.K8sResource, *util.APIHandleError) {
	//resourceObjects, err := c.HandleResourceYaml(resourceYaml, namespace, "update", name)
	//if err != nil {
	//	return nil, &util.APIHandleError{Code: 400, Err: fmt.Errorf("failed to parse yaml into k8s resource:%v", err)}
	//}
	//var resourceList []*dbmodel.K8sResource
	//
	//err = db.GetManager().K8sResourceDao().CreateK8sResourceInBatch(resourceList)
	//if err != nil {
	//	return nil, &util.APIHandleError{Code: 400, Err: fmt.Errorf("CreateK8sResource %v", err)}
	//}
	//return resourceList, nil
	return dbmodel.K8sResource{}, nil
}

func (c *clusterAction) DeleteAppK8SResource(ctx context.Context, namespace string, appID string, name string, resourceYaml string) *util.APIHandleError {
	logrus.Info("begin DeleteAppK8SResource")
	_, err := c.HandleResourceYaml(resourceYaml, namespace, "delete", name)
	if err != nil {
		return &util.APIHandleError{Code: 400, Err: fmt.Errorf("DeleteAppK8SResource %v", err)}
	}
	err = db.GetManager().K8sResourceDao().DeleteK8sResourceInBatch(appID, name)
	if err != nil {
		return &util.APIHandleError{Code: 400, Err: fmt.Errorf("DeleteAppK8SResource %v", err)}
	}
	return nil
}

type BuildResource struct {
	Resource *unstructured.Unstructured
	Success  int
	Status   string
}

func (c *clusterAction) HandleResourceYaml(resourceYaml string, namespace string, change string, name string) ([]BuildResource, error) {
	var buildResourceList []BuildResource
	dc, err := dynamic.NewForConfig(c.config)
	if err != nil {
		logrus.Errorf("%v", err)
		return nil, err
	}
	resourceYamlByte := []byte(resourceYaml)
	if err != nil {
		logrus.Errorf("%v", err)
		return nil, err
	}
	decoder := yamlt.NewYAMLOrJSONDecoder(bytes.NewReader(resourceYamlByte), 100)
	for {
		var rawObj runtime.RawExtension
		if err = decoder.Decode(&rawObj); err != nil {
			break
		}
		var status string
		var success int
		if change == "create" {
			status = "创建失败"
			success = 3
		} else {
			status = "更新失败"
			success = 4
		}
		obj, gvk, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			logrus.Errorf("%v", err)
			buildResourceList = append(buildResourceList, BuildResource{
				Resource: nil,
				Success:  success,
				Status:   fmt.Sprintf("%v%v", status, err),
			})
			continue
		}
		unstructuredObj := &unstructured.Unstructured{Object: unstructuredMap}
		gr, err := restmapper.GetAPIGroupResources(c.clientset.Discovery())
		if err != nil {
			logrus.Errorf("%v", err)
			buildResourceList = append(buildResourceList, BuildResource{
				Resource: nil,
				Success:  success,
				Status:   fmt.Sprintf("%v%v", status, err),
			})
			continue
		}
		mapper := restmapper.NewDiscoveryRESTMapper(gr)
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			buildResourceList = append(buildResourceList, BuildResource{
				Resource: nil,
				Success:  success,
				Status:   fmt.Sprintf("%v%v", status, err),
			})
			continue
		}
		var dri dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			unstructuredObj.SetNamespace(namespace)
			dri = dc.Resource(mapping.Resource).Namespace(unstructuredObj.GetNamespace())
		} else {
			dri = dc.Resource(mapping.Resource)
		}
		switch change {
		case "create":
			obj, err := dri.Create(context.TODO(), unstructuredObj, metav1.CreateOptions{})
			var br BuildResource
			if err != nil {
				br = BuildResource{
					Resource: obj,
					Success:  success,
					Status:   fmt.Sprintf("%v%v", status, err),
				}
			} else {
				br = BuildResource{
					Resource: obj,
					Success:  1,
					Status:   fmt.Sprintf("创建成功"),
				}
			}
			buildResourceList = append(buildResourceList, br)
		case "delete":
			err := dri.Delete(context.TODO(), name, metav1.DeleteOptions{})
			if err != nil {
				logrus.Errorf("delete k8s resource error%v", err)
			}
		case "update":
			//obj, err := dri.Update(context.TODO(), unstructuredObj, metav1.UpdateOptions{})
			//if err != nil {
			//	r := []BuildResource{{
			//		obj,
			//		fmt.Sprintf("创建失败%v", err),
			//	}}
			//} else {
			//
			//}
			//
			//return r, nil
		}

	}
	return buildResourceList, nil
}

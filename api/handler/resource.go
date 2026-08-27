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
	"github.com/goodrain/rainbond/mq/client"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	yamlt "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"os"
	"strings"
)

// AddAppK8SResource -
func (c *clusterAction) AddAppK8SResource(ctx context.Context, namespace string, appID string, resourceYaml string) ([]*dbmodel.K8sResource, *util.APIHandleError) {
	if err := c.ensureK8sResourceCreateAllowed(appID, namespace, resourceYaml); err != nil {
		return nil, err
	}
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
				AppID:                 appID,
				Name:                  "未识别",
				Kind:                  "未识别",
				Content:               rsYaml,
				ResourceOrigin:        dbmodel.K8sResourceOriginUser,
				ForceFinalizerAllowed: true,
				ErrorOverview:         resource.ErrorOverview,
				State:                 resource.State,
			})
		} else {
			resourceObject.Resource = c.ResourceProcessing(resourceObject.Resource, namespace)
			rsYaml, _ := ObjectToJSONORYaml("yaml", resourceObject.Resource)
			resourceList = append(resourceList, &dbmodel.K8sResource{
				AppID:                 appID,
				Name:                  resource.Resource.GetName(),
				Kind:                  resource.Resource.GetKind(),
				Content:               rsYaml,
				ResourceUID:           string(resource.Resource.GetUID()),
				ResourceIdentity:      resourceIdentityFromBuildResource(resource),
				ResourceOrigin:        dbmodel.K8sResourceOriginUser,
				ForceFinalizerAllowed: true,
				ErrorOverview:         resource.ErrorOverview,
				State:                 resource.State,
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

	if len(resourceObjects) == 0 {
		return dbmodel.K8sResource{}, &util.APIHandleError{Code: 400, Err: fmt.Errorf("no resource objects returned for %v", name)}
	}

	if resourceObjects[0].State != model.GetError {
		rs.Content, _ = ObjectToJSONORYaml("yaml", resourceObjects[0].Resource)
		if resourceObjects[0].Resource != nil {
			rs.ResourceUID = string(resourceObjects[0].Resource.GetUID())
			rs.ResourceIdentity = resourceIdentityFromBuildResource(resourceObjects[0])
		}
	}

	if err := db.GetManager().K8sResourceDao().UpdateModel(&rs); err != nil {
		return dbmodel.K8sResource{}, &util.APIHandleError{Code: 409, Err: fmt.Errorf("k8s resource cannot be refreshed while deletion is in progress: %v", err)}
	}
	return rs, nil
}

// UpdateAppK8SResource -
func (c *clusterAction) UpdateAppK8SResource(ctx context.Context, namespace, appID, name, resourceYaml, kind string) (dbmodel.K8sResource, *util.APIHandleError) {
	rs, err := db.GetManager().K8sResourceDao().GetK8sResourceByName(appID, name, kind)
	if err != nil {
		return dbmodel.K8sResource{}, &util.APIHandleError{Code: 400, Err: fmt.Errorf("get k8s resource %v", err)}
	}
	if rs.DeleteStatus != dbmodel.K8sResourceDeleteStatusActive {
		return dbmodel.K8sResource{}, &util.APIHandleError{Code: 409, Err: fmt.Errorf("k8s resource %d cannot be updated while deletion is in progress", rs.ID)}
	}
	resourceObjects := c.HandleResourceYaml([]byte(resourceYaml), namespace, "update", name, map[string]string{"app_id": appID})

	if len(resourceObjects) == 0 {
		return dbmodel.K8sResource{}, &util.APIHandleError{Code: 400, Err: fmt.Errorf("no resource objects returned for %v", name)}
	}

	var rsYaml string
	if resourceObjects[0].State == model.UpdateError {
		rsYaml = resourceYaml
		rs.State = resourceObjects[0].State
		rs.ErrorOverview = resourceObjects[0].ErrorOverview
		rs.Content = rsYaml
		if resourceObjects[0].Resource != nil {
			rs.ResourceUID = string(resourceObjects[0].Resource.GetUID())
			rs.ResourceIdentity = resourceIdentityFromBuildResource(resourceObjects[0])
		}
	} else {
		rsYaml, _ = ObjectToJSONORYaml("yaml", resourceObjects[0].Resource)
		rs.State = resourceObjects[0].State
		rs.ErrorOverview = resourceObjects[0].ErrorOverview
		rs.Content = rsYaml
		if resourceObjects[0].Resource != nil {
			rs.ResourceUID = string(resourceObjects[0].Resource.GetUID())
			rs.ResourceIdentity = resourceIdentityFromBuildResource(resourceObjects[0])
		}
		if err := db.GetManager().K8sResourceDao().UpdateModel(&rs); err != nil {
			return dbmodel.K8sResource{}, &util.APIHandleError{Code: 409, Err: fmt.Errorf("k8s resource cannot be updated while deletion is in progress: %v", err)}
		}
	}
	return rs, nil
}

// DeleteAppK8SResource -
func (c *clusterAction) DeleteAppK8SResource(ctx context.Context, namespace, appID, name, resourceYaml, kind string) {
	_, err := c.AcceptAppK8SResourceDeletions(ctx, appID, []model.HandleResource{{Name: name, Kind: kind}})
	if err != nil {
		logrus.Errorf("accept legacy k8s resource deletion %s/%s: %v", name, kind, err)
	}
}

// AcceptAppK8SResourceDeletions durably accepts a whole deletion batch before
// trying to publish Worker work. MQ failure is logged only: the Worker recovery
// scanner finds the persisted DELETING rows and requeues them later.
func (c *clusterAction) AcceptAppK8SResourceDeletions(ctx context.Context, appID string, resources []model.HandleResource) ([]model.K8sResourceDeleteStatus, *util.APIHandleError) {
	targets := make([]dbmodel.K8sResourceDeleteTarget, 0, len(resources))
	for _, resource := range resources {
		targets = append(targets, dbmodel.K8sResourceDeleteTarget{ResourceID: resource.ResourceID, Name: resource.Name, Kind: resource.Kind})
	}
	resolved, newlyAccepted, err := db.GetManager().K8sResourceDao().AcceptK8sResourceDeletions(appID, targets)
	if err != nil {
		return nil, &util.APIHandleError{Code: 409, Err: fmt.Errorf("accept k8s resource deletion: %v", err)}
	}
	if len(newlyAccepted) > 0 {
		c.enqueueK8sResourceDeletion(newlyAccepted)
	}
	acceptedByID := make(map[uint]struct{}, len(newlyAccepted))
	for _, resource := range newlyAccepted {
		acceptedByID[resource.ID] = struct{}{}
	}
	statuses := make([]model.K8sResourceDeleteStatus, 0, len(resolved))
	for _, resource := range resolved {
		status := newK8sResourceDeleteStatus(resource)
		_, status.Accepted = acceptedByID[resource.ID]
		statuses = append(statuses, status)
	}
	return statuses, nil
}

// GetAppK8SResourceDeleteStatus returns identity and lifecycle fields only.
// Missing saved records represent physical deletion completion to Console.
func (c *clusterAction) GetAppK8SResourceDeleteStatus(ctx context.Context, appID string, targets []model.HandleResource) ([]model.K8sResourceDeleteStatus, *util.APIHandleError) {
	deleteTargets := make([]dbmodel.K8sResourceDeleteTarget, 0, len(targets))
	for _, target := range targets {
		deleteTargets = append(deleteTargets, dbmodel.K8sResourceDeleteTarget{ResourceID: target.ResourceID, Name: target.Name, Kind: target.Kind})
	}
	resources, err := db.GetManager().K8sResourceDao().ListK8sResourcesDeleteStatus(appID, deleteTargets)
	if err != nil {
		return nil, &util.APIHandleError{Code: 400, Err: fmt.Errorf("list k8s resource deletion status: %v", err)}
	}
	statuses := make([]model.K8sResourceDeleteStatus, 0, len(resources))
	seenIDs := make(map[uint]struct{}, len(resources))
	for _, resource := range resources {
		if _, duplicate := seenIDs[resource.ID]; duplicate {
			continue
		}
		seenIDs[resource.ID] = struct{}{}
		statuses = append(statuses, newK8sResourceDeleteStatus(resource))
	}
	return statuses, nil
}

func (c *clusterAction) enqueueK8sResourceDeletion(resources []dbmodel.K8sResource) {
	targets := make([]map[string]interface{}, 0, len(resources))
	for _, resource := range resources {
		targets = append(targets, map[string]interface{}{
			"resource_id":       resource.ID,
			"delete_generation": resource.DeleteGeneration,
		})
	}
	if c.mqclient == nil {
		logrus.Errorf("k8s resource deletion accepted but worker MQ client is unavailable")
		return
	}
	if err := c.mqclient.SendBuilderTopic(client.TaskStruct{
		Topic:    client.WorkerTopic,
		TaskType: "delete_k8s_resource",
		TaskBody: map[string]interface{}{"resources": targets},
	}); err != nil {
		logrus.Errorf("k8s resource deletion accepted but worker task publish failed: %v", err)
	}
}

func newK8sResourceDeleteStatus(resource dbmodel.K8sResource) model.K8sResourceDeleteStatus {
	return model.K8sResourceDeleteStatus{
		ResourceID:       resource.ID,
		Name:             resource.Name,
		Kind:             resource.Kind,
		DeleteStatus:     resource.DeleteStatus,
		DeleteError:      resource.DeleteError,
		DeleteAttempts:   resource.DeleteAttempts,
		DeleteGeneration: resource.DeleteGeneration,
	}
}

// SyncAppK8SResources -
func (c *clusterAction) SyncAppK8SResources(ctx context.Context, req *model.SyncResources) ([]*dbmodel.K8sResource, *util.APIHandleError) {
	// Only Add
	var resourceList []*dbmodel.K8sResource
	for _, k8sResource := range req.K8sResources {
		if err := c.ensureK8sResourceCreateAllowed(k8sResource.AppID, k8sResource.Namespace, k8sResource.ResourceYaml); err != nil {
			return nil, err
		}
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
				AppID:                 k8sResource.AppID,
				Name:                  k8sResource.Name,
				Kind:                  k8sResource.Kind,
				Content:               rsYaml,
				ResourceUID:           resourceUIDFromBuildResource(resourceObjects[0]),
				ResourceIdentity:      resourceIdentityFromBuildResource(resourceObjects[0]),
				ResourceOrigin:        dbmodel.K8sResourceOriginMarket,
				ForceFinalizerAllowed: true,
				ErrorOverview:         resourceObjects[0].ErrorOverview,
				State:                 resourceObjects[0].State,
			})
		}
	}
	err := db.GetManager().K8sResourceDao().CreateK8sResource(resourceList)
	if err != nil {
		return nil, &util.APIHandleError{Code: 400, Err: fmt.Errorf("SyncK8sResource %v", err)}
	}
	return resourceList, nil
}

func resourceUIDFromBuildResource(resource *model.BuildResource) string {
	if resource == nil || resource.Resource == nil {
		return ""
	}
	return string(resource.Resource.GetUID())
}

func resourceIdentityFromBuildResource(resource *model.BuildResource) string {
	if resource == nil || resource.Resource == nil || resource.ResourceGVR.Resource == "" || resource.Resource.GetName() == "" {
		return ""
	}
	return dbmodel.K8sResourcePhysicalIdentity(resource.ResourceGVR.Group, resource.ResourceGVR.Resource, resource.Resource.GetNamespace(), resource.Resource.GetName())
}

func k8sResourceIdentityFromMapper(mapper meta.RESTMapper, resource *unstructured.Unstructured) (string, error) {
	if resource == nil || resource.GetAPIVersion() == "" || resource.GetKind() == "" || resource.GetName() == "" {
		return "", fmt.Errorf("Kubernetes resource identity requires apiVersion, kind, and metadata.name")
	}
	gvk := schema.FromAPIVersionAndKind(resource.GetAPIVersion(), resource.GetKind())
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return "", err
	}
	return dbmodel.K8sResourcePhysicalIdentity(mapping.Resource.Group, mapping.Resource.Resource, resource.GetNamespace(), resource.GetName()), nil
}

func k8sResourceUIDFromYAML(resourceYAML string) (string, error) {
	decoder := yamlt.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(strings.TrimPrefix(resourceYAML, "\n"))), 1000)
	var objectCount int
	var resourceUID string
	for {
		var rawObj runtime.RawExtension
		if err := decoder.Decode(&rawObj); err != nil {
			if err.Error() == "EOF" {
				break
			}
			return "", err
		}
		if len(rawObj.Raw) == 0 {
			continue
		}
		obj, _, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		if err != nil {
			return "", err
		}
		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return "", err
		}
		objectCount++
		resourceUID = string((&unstructured.Unstructured{Object: unstructuredMap}).GetUID())
	}
	if objectCount != 1 {
		return "", fmt.Errorf("expected exactly one Kubernetes resource object, got %d", objectCount)
	}
	return resourceUID, nil
}

// ensureK8sResourceCreateAllowed prevents a new create from racing a Region
// record whose physical deletion is still running or has failed. Invalid YAML
// deliberately falls through to the existing create path so legacy validation
// and error persistence behavior remain unchanged.
func (c *clusterAction) ensureK8sResourceCreateAllowed(appID, namespace, resourceYAML string) *util.APIHandleError {
	identities, err := c.k8sResourceIdentitiesFromYAML(namespace, resourceYAML)
	if err != nil {
		return nil
	}
	return c.ensureK8sResourceIdentitiesCreateAllowed(appID, identities)
}

func (c *clusterAction) ensureK8sResourceIdentitiesCreateAllowed(appID string, identities []dbmodel.K8sResourceDeleteTarget) *util.APIHandleError {
	if len(identities) == 0 {
		return nil
	}
	if err := db.GetManager().K8sResourceDao().EnsureK8sResourcesCreatable(appID, identities); err != nil {
		return &util.APIHandleError{Code: 409, Err: fmt.Errorf("k8s resource create is blocked by cleanup reservation: %v", err)}
	}
	return nil
}

func (c *clusterAction) k8sResourceIdentitiesFromYAML(namespace, resourceYAML string) ([]dbmodel.K8sResourceDeleteTarget, error) {
	decoder := yamlt.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(strings.TrimPrefix(resourceYAML, "\n"))), 1000)
	identities := make([]dbmodel.K8sResourceDeleteTarget, 0, 1)
	for {
		var rawObj runtime.RawExtension
		if err := decoder.Decode(&rawObj); err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}
		if len(rawObj.Raw) == 0 {
			continue
		}
		obj, _, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		if err != nil {
			return nil, err
		}
		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return nil, err
		}
		unstructuredObj := unstructured.Unstructured{Object: unstructuredMap}
		if unstructuredObj.GetName() == "" || unstructuredObj.GetKind() == "" {
			continue
		}
		mapping, err := c.mapper.RESTMapping(schema.FromAPIVersionAndKind(unstructuredObj.GetAPIVersion(), unstructuredObj.GetKind()).GroupKind(), schema.FromAPIVersionAndKind(unstructuredObj.GetAPIVersion(), unstructuredObj.GetKind()).Version)
		if err != nil {
			if !meta.IsNoMatchError(err) {
				return nil, err
			}
			mapper, refreshErr := RefreshMapper(c.clientset)
			if refreshErr != nil {
				return nil, refreshErr
			}
			c.mapper = mapper
			mapping, err = c.mapper.RESTMapping(schema.FromAPIVersionAndKind(unstructuredObj.GetAPIVersion(), unstructuredObj.GetKind()).GroupKind(), schema.FromAPIVersionAndKind(unstructuredObj.GetAPIVersion(), unstructuredObj.GetKind()).Version)
			if err != nil {
				return nil, err
			}
		}
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace && namespace != "" {
			unstructuredObj.SetNamespace(namespace)
		}
		identities = append(identities, dbmodel.K8sResourceDeleteTarget{
			Name:             unstructuredObj.GetName(),
			Kind:             unstructuredObj.GetKind(),
			ResourceIdentity: dbmodel.K8sResourcePhysicalIdentity(mapping.Resource.Group, mapping.Resource.Resource, unstructuredObj.GetNamespace(), unstructuredObj.GetName()),
		})
	}
	return identities, nil
}

// RefreshMapper -
func RefreshMapper(clientset *kubernetes.Clientset) (meta.RESTMapper, error) {
	gr, err := restmapper.GetAPIGroupResources(clientset)
	if err != nil {
		return nil, err
	}
	mapper := restmapper.NewDiscoveryRESTMapper(gr)
	return mapper, nil
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
			mapper, err := RefreshMapper(c.clientset)
			if err != nil {
				return buildResourceList
			}
			c.mapper = mapper
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
			Resource:    &unstructuredObj,
			Dri:         dri,
			ResourceGVR: mapping.Resource,
		}
		// 在解码资源后，处理之前添加检查
		if os.Getenv("USE_SAAS") == "true" && gvk.Kind != "ConfigMap" && gvk.Kind != "Secret" {
			errMsg := fmt.Sprintf("无效的资源类型: %s. 只允许 ConfigMap、Secret", gvk.Kind)
			logrus.Error(errMsg)
			buildResourceList = append(buildResourceList, &model.BuildResource{
				State:         model.CreateError,
				ErrorOverview: errMsg,
			})
			return buildResourceList
		} else {
			buildResourceList = append(buildResourceList, br)
		}
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

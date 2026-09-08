package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/util/wait"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
)

var (
	// ErrCRDCascadeConfirmationRequired prevents an implicit cluster-wide CR deletion.
	ErrCRDCascadeConfirmationRequired = errors.New("deleting the CRD affects resources outside the current application; explicit cascade confirmation is required")
	// ErrK8sResourceDeletionNotConfirmed indicates that Kubernetes still reports a resource.
	ErrK8sResourceDeletionNotConfirmed = errors.New("Kubernetes resource deletion could not be confirmed")
	// ErrInvalidK8sResourceDeletionRequest identifies malformed user-supplied resource metadata.
	ErrInvalidK8sResourceDeletionRequest = errors.New("invalid Kubernetes resource deletion request")
	crdGVR                               = schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}
)

const appIDLabel = "app_id"

type k8sResourceDeletionOrchestrator struct {
	dynamicClient dynamic.Interface
	mapper        meta.RESTMapper
	refreshMapper func() (meta.RESTMapper, error)
	waitInterval  time.Duration
	waitTimeout   time.Duration
}

type plannedResource struct {
	item       model.HandleResource
	object     *unstructured.Unstructured
	mapping    *meta.RESTMapping
	apiRemoved bool
}

type plannedCRD struct {
	resource   plannedResource
	impact     model.CRDDeletionImpact
	gvr        schema.GroupVersionResource
	namespaced bool
	instances  []unstructured.Unstructured
}

type k8sResourceDeletionPlan struct {
	resources []plannedResource
	crds      []plannedCRD
	impact    model.K8sResourceDeletionImpact
}

func (c *clusterAction) deletionOrchestrator() *k8sResourceDeletionOrchestrator {
	return &k8sResourceDeletionOrchestrator{
		dynamicClient: c.dynamicClient,
		mapper:        c.mapper,
		refreshMapper: func() (meta.RESTMapper, error) {
			mapper, err := RefreshMapper(c.clientset)
			if err == nil {
				c.mapper = mapper
			}
			return mapper, err
		},
	}
}

// PreviewK8SResourceDeletion returns cluster-wide CRD impact without mutation.
func (c *clusterAction) PreviewK8SResourceDeletion(ctx context.Context, req *model.K8sResourceDeletionRequest) (*model.K8sResourceDeletionImpact, *util.APIHandleError) {
	impact, err := c.deletionOrchestrator().Preview(ctx, req)
	if err != nil {
		if errors.Is(err, ErrInvalidK8sResourceDeletionRequest) {
			return nil, util.CreateAPIHandleError(http.StatusBadRequest, err)
		}
		return nil, util.CreateAPIHandleError(http.StatusInternalServerError, err)
	}
	return impact, nil
}

// DeleteK8SResources performs a confirmed, ordered resource deletion and then removes Region metadata.
func (c *clusterAction) DeleteK8SResources(ctx context.Context, req *model.K8sResourceDeletionRequest) (*model.K8sResourceDeletionResult, *util.APIHandleError) {
	result, err := c.deletionOrchestrator().Delete(ctx, req)
	if err != nil {
		if errors.Is(err, ErrInvalidK8sResourceDeletionRequest) {
			return nil, util.CreateAPIHandleError(http.StatusBadRequest, err)
		}
		if errors.Is(err, ErrCRDCascadeConfirmationRequired) || errors.Is(err, ErrK8sResourceDeletionNotConfirmed) {
			return nil, util.CreateAPIHandleError(http.StatusConflict, err)
		}
		return nil, util.CreateAPIHandleError(http.StatusInternalServerError, err)
	}
	if err := cleanupDeletedResourceMetadata(req, result); err != nil {
		return nil, util.CreateAPIHandleError(http.StatusInternalServerError, fmt.Errorf("clean Region Kubernetes resource metadata: %w", err))
	}
	return result, nil
}

// ReconcileK8SResources removes Region metadata only for resources confirmed absent from Kubernetes.
func (c *clusterAction) ReconcileK8SResources(ctx context.Context, req *model.K8sResourceReconcileRequest) (*model.K8sResourceReconcileResult, *util.APIHandleError) {
	result := c.deletionOrchestrator().Reconcile(ctx, req)
	missing := make(map[string]struct{}, len(result.MissingClientIDs))
	for _, clientID := range result.MissingClientIDs {
		missing[clientID] = struct{}{}
	}
	for _, resource := range req.K8sResources {
		if _, ok := missing[resource.ClientID]; !ok {
			continue
		}
		if err := db.GetManager().K8sResourceDao().DeleteK8sResource(req.AppID, resource.Name, resource.Kind); err != nil {
			return nil, util.CreateAPIHandleError(http.StatusInternalServerError, fmt.Errorf("clean Region Kubernetes resource metadata: %w", err))
		}
	}
	return result, nil
}

// Preview builds a deletion plan without mutating Kubernetes resources.
func (o *k8sResourceDeletionOrchestrator) Preview(ctx context.Context, req *model.K8sResourceDeletionRequest) (*model.K8sResourceDeletionImpact, error) {
	plan, err := o.buildPlan(ctx, req)
	if err != nil {
		return nil, err
	}
	return &plan.impact, nil
}

// Delete removes dependent custom resources before ordinary resources and CRDs.
func (o *k8sResourceDeletionOrchestrator) Delete(ctx context.Context, req *model.K8sResourceDeletionRequest) (*model.K8sResourceDeletionResult, error) {
	plan, err := o.buildPlan(ctx, req)
	if err != nil {
		return nil, err
	}
	if plan.impact.RequiresCascade && !req.CascadeCRD {
		return nil, ErrCRDCascadeConfirmationRequired
	}

	for i := range plan.crds {
		if err := o.deleteCustomResources(ctx, &plan.crds[i]); err != nil {
			return nil, err
		}
	}
	for i := range plan.resources {
		resource := &plan.resources[i]
		if resource.apiRemoved || isCRD(resource.object.GroupVersionKind()) || resource.item.State != model.CreateSuccess && resource.item.State != model.UpdateSuccess {
			continue
		}
		if err := o.deleteResourceAndWait(ctx, resource); err != nil {
			return nil, err
		}
	}
	for i := range plan.crds {
		if err := o.deleteCRDAndWait(ctx, &plan.crds[i]); err != nil {
			return nil, err
		}
	}

	result := &model.K8sResourceDeletionResult{
		Status:       "completed",
		CascadedCRDs: plan.impact.CRDs,
	}
	for _, resource := range req.K8sResources {
		result.DeletedClientIDs = append(result.DeletedClientIDs, resource.ClientID)
	}
	return result, nil
}

// Reconcile classifies only confirmed missing resources as safe metadata deletions.
func (o *k8sResourceDeletionOrchestrator) Reconcile(ctx context.Context, req *model.K8sResourceReconcileRequest) *model.K8sResourceReconcileResult {
	result := &model.K8sResourceReconcileResult{}
	for _, item := range req.K8sResources {
		if item.State != model.CreateSuccess && item.State != model.UpdateSuccess {
			continue
		}
		obj, err := decodeSingleResource(item.ResourceYaml)
		if err != nil {
			result.Unknown = append(result.Unknown, model.K8sResourceUnknown{ClientID: item.ClientID, Error: err.Error()})
			continue
		}
		mapping, apiRemoved, err := o.resolveMapping(obj.GroupVersionKind())
		if err != nil {
			result.Unknown = append(result.Unknown, model.K8sResourceUnknown{ClientID: item.ClientID, Error: err.Error()})
			continue
		}
		if apiRemoved {
			result.MissingClientIDs = append(result.MissingClientIDs, item.ClientID)
			continue
		}
		name := item.Name
		if name == "" {
			name = obj.GetName()
		}
		resourceClient := o.resourceClient(mapping, item.Namespace, obj.GetNamespace())
		_, err = resourceClient.Get(ctx, name, metav1.GetOptions{})
		switch {
		case apierrors.IsNotFound(err):
			result.MissingClientIDs = append(result.MissingClientIDs, item.ClientID)
		case err != nil:
			result.Unknown = append(result.Unknown, model.K8sResourceUnknown{ClientID: item.ClientID, Error: err.Error()})
		}
	}
	return result
}

func (o *k8sResourceDeletionOrchestrator) buildPlan(ctx context.Context, req *model.K8sResourceDeletionRequest) (*k8sResourceDeletionPlan, error) {
	if req == nil || strings.TrimSpace(req.AppID) == "" {
		return nil, fmt.Errorf("%w: app_id is required", ErrInvalidK8sResourceDeletionRequest)
	}
	plan := &k8sResourceDeletionPlan{}
	plannedCRDs := make(map[string]struct{})
	for _, item := range req.K8sResources {
		if item.State != model.CreateSuccess && item.State != model.UpdateSuccess {
			plan.resources = append(plan.resources, plannedResource{item: item})
			continue
		}
		obj, err := decodeSingleResource(item.ResourceYaml)
		if err != nil {
			return nil, fmt.Errorf("%w: decode %s/%s: %v", ErrInvalidK8sResourceDeletionRequest, item.Kind, item.Name, err)
		}
		mapping, apiRemoved, err := o.resolveMapping(obj.GroupVersionKind())
		if err != nil {
			return nil, fmt.Errorf("resolve %s/%s: %w", item.Kind, item.Name, err)
		}
		resource := plannedResource{item: item, object: obj, mapping: mapping, apiRemoved: apiRemoved}
		plan.resources = append(plan.resources, resource)
		if !isCRD(obj.GroupVersionKind()) || apiRemoved {
			continue
		}
		if _, ok := plannedCRDs[obj.GetName()]; ok {
			continue
		}
		plannedCRDs[obj.GetName()] = struct{}{}
		crd, err := o.buildCRDPlan(ctx, resource, req.AppID)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		plan.crds = append(plan.crds, *crd)
	}

	plan.impact.CRDCount = len(plan.crds)
	plan.impact.HasCRD = plan.impact.CRDCount > 0
	affectedApps := make(map[string]struct{})
	for _, crd := range plan.crds {
		plan.impact.CRDs = append(plan.impact.CRDs, crd.impact)
		plan.impact.CRCount += crd.impact.CurrentAppCRCount + crd.impact.OtherAppCRCount + crd.impact.UnownedCRCount
		plan.impact.UnownedCRCount += crd.impact.UnownedCRCount
		if crd.impact.OtherAppCRCount > 0 || crd.impact.UnownedCRCount > 0 {
			plan.impact.RequiresCascade = true
		}
		for _, appID := range crd.impact.AffectedRegionAppIDs {
			affectedApps[appID] = struct{}{}
		}
	}
	plan.impact.OtherAppCount = len(affectedApps)
	return plan, nil
}

func (o *k8sResourceDeletionOrchestrator) buildCRDPlan(ctx context.Context, resource plannedResource, currentAppID string) (*plannedCRD, error) {
	live, err := o.dynamicClient.Resource(crdGVR).Get(ctx, resource.object.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	group, found, err := unstructured.NestedString(live.Object, "spec", "group")
	if err != nil || !found || group == "" {
		return nil, fmt.Errorf("CRD %s has no spec.group", live.GetName())
	}
	plural, found, err := unstructured.NestedString(live.Object, "spec", "names", "plural")
	if err != nil || !found || plural == "" {
		return nil, fmt.Errorf("CRD %s has no spec.names.plural", live.GetName())
	}
	kind, found, err := unstructured.NestedString(live.Object, "spec", "names", "kind")
	if err != nil || !found || kind == "" {
		return nil, fmt.Errorf("CRD %s has no spec.names.kind", live.GetName())
	}
	scope, found, err := unstructured.NestedString(live.Object, "spec", "scope")
	if err != nil || !found || scope == "" {
		return nil, fmt.Errorf("CRD %s has no spec.scope", live.GetName())
	}
	version, err := crdStorageVersion(live)
	if err != nil {
		return nil, err
	}
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: plural}
	list, err := o.dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list custom resources for CRD %s: %w", live.GetName(), err)
	}

	impact := model.CRDDeletionImpact{
		Name:    live.GetName(),
		Group:   group,
		Version: version,
		Kind:    kind,
		Plural:  plural,
		Scope:   scope,
	}
	otherApps := make(map[string]struct{})
	for i := range list.Items {
		appID := list.Items[i].GetLabels()[appIDLabel]
		switch {
		case appID == "":
			impact.UnownedCRCount++
		case appID == currentAppID:
			impact.CurrentAppCRCount++
		default:
			impact.OtherAppCRCount++
			otherApps[appID] = struct{}{}
		}
	}
	for appID := range otherApps {
		impact.AffectedRegionAppIDs = append(impact.AffectedRegionAppIDs, appID)
	}
	sort.Strings(impact.AffectedRegionAppIDs)
	return &plannedCRD{
		resource:   resource,
		impact:     impact,
		gvr:        gvr,
		namespaced: strings.EqualFold(scope, "Namespaced"),
		instances:  list.Items,
	}, nil
}

func (o *k8sResourceDeletionOrchestrator) deleteCustomResources(ctx context.Context, crd *plannedCRD) error {
	for i := range crd.instances {
		instance := &crd.instances[i]
		resourceClient := o.dynamicClient.Resource(crd.gvr)
		var client dynamic.ResourceInterface = resourceClient
		if crd.namespaced {
			client = resourceClient.Namespace(instance.GetNamespace())
		}
		if err := client.Delete(ctx, instance.GetName(), metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete custom resource %s/%s for CRD %s: %w", instance.GetNamespace(), instance.GetName(), crd.impact.Name, err)
		}
	}
	var checkErr error
	err := wait.PollUntilContextTimeout(ctx, o.interval(), o.timeout(), true, func(ctx context.Context) (bool, error) {
		list, err := o.dynamicClient.Resource(crd.gvr).List(ctx, metav1.ListOptions{})
		if err != nil {
			checkErr = err
			return false, err
		}
		return len(list.Items) == 0, nil
	})
	if err != nil {
		if checkErr != nil {
			return fmt.Errorf("confirm custom resources for CRD %s: %w", crd.impact.Name, checkErr)
		}
		return fmt.Errorf("%w for CRD %s custom resources: %v", ErrK8sResourceDeletionNotConfirmed, crd.impact.Name, err)
	}
	return nil
}

func (o *k8sResourceDeletionOrchestrator) deleteResourceAndWait(ctx context.Context, resource *plannedResource) error {
	name := resource.item.Name
	if name == "" {
		name = resource.object.GetName()
	}
	client := o.resourceClient(resource.mapping, resource.item.Namespace, resource.object.GetNamespace())
	if err := client.Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete %s/%s: %w", resource.object.GetKind(), name, err)
	}
	var checkErr error
	err := wait.PollUntilContextTimeout(ctx, o.interval(), o.timeout(), true, func(ctx context.Context) (bool, error) {
		_, err := client.Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			checkErr = err
		}
		return false, err
	})
	if err != nil {
		if checkErr != nil {
			return fmt.Errorf("confirm %s/%s deletion: %w", resource.object.GetKind(), name, checkErr)
		}
		return fmt.Errorf("%w for %s/%s: %v", ErrK8sResourceDeletionNotConfirmed, resource.object.GetKind(), name, err)
	}
	return nil
}

func (o *k8sResourceDeletionOrchestrator) deleteCRDAndWait(ctx context.Context, crd *plannedCRD) error {
	if err := o.dynamicClient.Resource(crdGVR).Delete(ctx, crd.impact.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete CRD %s: %w", crd.impact.Name, err)
	}
	var checkErr error
	err := wait.PollUntilContextTimeout(ctx, o.interval(), o.timeout(), true, func(ctx context.Context) (bool, error) {
		_, err := o.dynamicClient.Resource(crdGVR).Get(ctx, crd.impact.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			checkErr = err
		}
		return false, err
	})
	if err != nil {
		if checkErr != nil {
			return fmt.Errorf("confirm CRD %s deletion: %w", crd.impact.Name, checkErr)
		}
		return fmt.Errorf("%w for CRD %s: %v", ErrK8sResourceDeletionNotConfirmed, crd.impact.Name, err)
	}
	return nil
}

func (o *k8sResourceDeletionOrchestrator) resolveMapping(gvk schema.GroupVersionKind) (*meta.RESTMapping, bool, error) {
	mapping, err := o.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err == nil {
		return mapping, false, nil
	}
	if !meta.IsNoMatchError(err) {
		return nil, false, err
	}
	if o.refreshMapper == nil {
		return nil, false, err
	}
	mapper, refreshErr := o.refreshMapper()
	if refreshErr != nil {
		return nil, false, refreshErr
	}
	o.mapper = mapper
	mapping, err = mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if meta.IsNoMatchError(err) {
		return nil, true, nil
	}
	return mapping, false, err
}

func (o *k8sResourceDeletionOrchestrator) resourceClient(mapping *meta.RESTMapping, requestedNamespace, objectNamespace string) dynamic.ResourceInterface {
	resourceClient := o.dynamicClient.Resource(mapping.Resource)
	if mapping.Scope.Name() != meta.RESTScopeNameNamespace {
		return resourceClient
	}
	namespace := requestedNamespace
	if namespace == "" {
		namespace = objectNamespace
	}
	return resourceClient.Namespace(namespace)
}

func (o *k8sResourceDeletionOrchestrator) interval() time.Duration {
	if o.waitInterval <= 0 {
		return time.Second
	}
	return o.waitInterval
}

func (o *k8sResourceDeletionOrchestrator) timeout() time.Duration {
	if o.waitTimeout <= 0 {
		return time.Minute
	}
	return o.waitTimeout
}

func decodeSingleResource(content string) (*unstructured.Unstructured, error) {
	decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(content)), 4096)
	var raw runtime.RawExtension
	if err := decoder.Decode(&raw); err != nil {
		return nil, err
	}
	obj, _, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(raw.Raw, nil, nil)
	if err != nil {
		return nil, err
	}
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	resource := &unstructured.Unstructured{Object: unstructuredMap}
	if resource.GetAPIVersion() == "" || resource.GetKind() == "" || resource.GetName() == "" {
		return nil, fmt.Errorf("apiVersion, kind, and metadata.name are required")
	}
	return resource, nil
}

func crdStorageVersion(crd *unstructured.Unstructured) (string, error) {
	versions, found, err := unstructured.NestedSlice(crd.Object, "spec", "versions")
	if err != nil {
		return "", err
	}
	var firstServed string
	if found {
		for _, rawVersion := range versions {
			version, ok := rawVersion.(map[string]interface{})
			if !ok {
				continue
			}
			storage, _, _ := unstructured.NestedBool(version, "storage")
			served, _, _ := unstructured.NestedBool(version, "served")
			name, _, _ := unstructured.NestedString(version, "name")
			if served && firstServed == "" {
				firstServed = name
			}
			if storage && served && name != "" {
				return name, nil
			}
		}
		if firstServed != "" {
			return firstServed, nil
		}
	}
	version, found, err := unstructured.NestedString(crd.Object, "spec", "version")
	if err == nil && found && version != "" {
		return version, nil
	}
	return "", fmt.Errorf("CRD %s has no storage version", crd.GetName())
}

func isCRD(gvk schema.GroupVersionKind) bool {
	return gvk.Group == crdGVR.Group && gvk.Kind == "CustomResourceDefinition"
}

func cleanupDeletedResourceMetadata(req *model.K8sResourceDeletionRequest, result *model.K8sResourceDeletionResult) error {
	dao := db.GetManager().K8sResourceDao()
	appResources, err := dao.ListByAppID(req.AppID)
	if err != nil {
		return err
	}
	selected := make(map[string]struct{}, len(req.K8sResources))
	for _, resource := range req.K8sResources {
		selected[resource.Name+"\x00"+resource.Kind] = struct{}{}
	}
	ids := make(map[uint]struct{})
	for _, resource := range appResources {
		if _, ok := selected[resource.Name+"\x00"+resource.Kind]; ok {
			ids[resource.ID] = struct{}{}
		}
	}
	for _, impact := range result.CascadedCRDs {
		resources, err := dao.ListByKind(impact.Kind)
		if err != nil {
			return err
		}
		for _, id := range matchingCRDMetadataIDs(resources, []model.CRDDeletionImpact{impact}) {
			ids[id] = struct{}{}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	deleteIDs := make([]uint, 0, len(ids))
	for id := range ids {
		deleteIDs = append(deleteIDs, id)
	}
	sort.Slice(deleteIDs, func(i, j int) bool { return deleteIDs[i] < deleteIDs[j] })
	return dao.DeleteK8sResourceByIDs(deleteIDs)
}

func matchingCRDMetadataIDs(resources []dbmodel.K8sResource, impacts []model.CRDDeletionImpact) []uint {
	var ids []uint
	for _, resource := range resources {
		group, kind, err := resourceGroupKind(resource.Content)
		if err != nil {
			continue
		}
		for _, impact := range impacts {
			if group == impact.Group && kind == impact.Kind {
				ids = append(ids, resource.ID)
				break
			}
		}
	}
	return ids
}

func resourceGroupKind(content string) (string, string, error) {
	obj, err := decodeSingleResource(content)
	if err != nil {
		return "", "", err
	}
	return obj.GroupVersionKind().Group, obj.GetKind(), nil
}

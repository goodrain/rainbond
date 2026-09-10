package handler

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	ktesting "k8s.io/client-go/testing"
)

var (
	testCRDGVR    = schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}
	testWidgetGVR = schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "widgets"}
	testConfigGVR = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
)

func TestK8sResourceDeletionPreviewRequiresCascadeForOtherApplications(t *testing.T) {
	orchestrator, _ := newDeletionTestOrchestrator(t,
		newTestWidget("owned", "team-a", "app-a"),
		newTestWidget("shared", "team-b", "app-b"),
		newTestWidget("unowned", "team-c", ""),
	)

	impact, err := orchestrator.Preview(context.Background(), newCRDDeletionRequest(false))
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}
	if !impact.HasCRD || !impact.RequiresCascade {
		t.Fatalf("Preview() impact = %#v, want CRD requiring cascade", impact)
	}
	if impact.CRDCount != 1 || impact.CRCount != 3 || impact.OtherAppCount != 1 || impact.UnownedCRCount != 1 {
		t.Fatalf("Preview() counts = %#v", impact)
	}
	crd := impact.CRDs[0]
	if crd.CurrentAppCRCount != 1 || crd.OtherAppCRCount != 1 || crd.UnownedCRCount != 1 {
		t.Fatalf("Preview() CRD counts = %#v", crd)
	}
	if len(crd.AffectedRegionAppIDs) != 1 || crd.AffectedRegionAppIDs[0] != "app-b" {
		t.Fatalf("Preview() affected apps = %v, want [app-b]", crd.AffectedRegionAppIDs)
	}
}

func TestK8sResourceDeletionBlocksCascadeBeforeMutation(t *testing.T) {
	orchestrator, client := newDeletionTestOrchestrator(t, newTestWidget("shared", "team-b", "app-b"))

	_, err := orchestrator.Delete(context.Background(), newCRDDeletionRequest(false))
	if !errors.Is(err, ErrCRDCascadeConfirmationRequired) {
		t.Fatalf("Delete() error = %v, want ErrCRDCascadeConfirmationRequired", err)
	}
	for _, action := range client.Actions() {
		if action.GetVerb() == "delete" {
			t.Fatalf("Delete() mutated Kubernetes before confirmation: %#v", action)
		}
	}
}

// capability_id: rainbond.k8s-resource.failed-delete-metadata-only
func TestK8sResourceDeletionSkipsFailedResourceKubernetesDeletion(t *testing.T) {
	orchestrator, client := newDeletionTestOrchestrator(t)
	req := newCRDDeletionRequest(false)
	req.K8sResources[0].State = model.CreateError

	result, err := orchestrator.Delete(context.Background(), req)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if result.Status != "completed" || len(result.DeletedClientIDs) != 1 || result.DeletedClientIDs[0] != "crd-row" {
		t.Fatalf("Delete() result = %#v, want metadata-only completion", result)
	}
	for _, action := range client.Actions() {
		if action.GetVerb() == "delete" {
			t.Fatalf("Delete() mutated Kubernetes for failed resource: %#v", action)
		}
	}
}

// capability_id: rainbond.k8s-resource.crd-cascade-delete
func TestK8sResourceDeletionDeletesCustomResourcesBeforeDefinition(t *testing.T) {
	orchestrator, client := newDeletionTestOrchestrator(t,
		newTestWidget("owned", "team-a", "app-a"),
		newTestWidget("shared", "team-b", "app-b"),
	)

	result, err := orchestrator.Delete(context.Background(), newCRDDeletionRequest(true))
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if result.Status != "completed" || len(result.CascadedCRDs) != 1 {
		t.Fatalf("Delete() result = %#v", result)
	}

	var deletedResources []string
	for _, action := range client.Actions() {
		if action.GetVerb() == "delete" {
			deletedResources = append(deletedResources, action.GetResource().Resource)
		}
	}
	if len(deletedResources) != 3 {
		t.Fatalf("delete actions = %v, want two CRs and one CRD", deletedResources)
	}
	if deletedResources[0] != "widgets" || deletedResources[1] != "widgets" || deletedResources[2] != "customresourcedefinitions" {
		t.Fatalf("delete order = %v, want CRs before CRD", deletedResources)
	}
}

func TestK8sResourceDeletionDoesNotDeleteDefinitionWhenCustomResourceDeleteFails(t *testing.T) {
	orchestrator, client := newDeletionTestOrchestrator(t, newTestWidget("blocked", "team-a", "app-a"))
	client.PrependReactor("delete", "widgets", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("webhook unavailable")
	})

	_, err := orchestrator.Delete(context.Background(), newCRDDeletionRequest(false))
	if err == nil {
		t.Fatal("Delete() error = nil, want CR deletion failure")
	}
	for _, action := range client.Actions() {
		if action.GetVerb() == "delete" && action.GetResource() == testCRDGVR {
			t.Fatalf("Delete() removed CRD after CR deletion failure: %#v", action)
		}
	}
}

// capability_id: rainbond.k8s-resource.deletion-timeout-final-check
func TestK8sResourceDeletionFinalCheckAvoidsTimeoutRace(t *testing.T) {
	orchestrator, client := newDeletionTestOrchestrator(t, newTestWidget("owned", "team-a", "app-a"))
	orchestrator.waitInterval = 100 * time.Millisecond
	orchestrator.waitTimeout = time.Millisecond

	listCalls := 0
	client.PrependReactor("list", "widgets", func(action ktesting.Action) (bool, runtime.Object, error) {
		listCalls++
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "WidgetList"})
		if listCalls <= 2 {
			list.Items = []unstructured.Unstructured{*newTestWidget("owned", "team-a", "app-a")}
		}
		return true, list, nil
	})

	result, err := orchestrator.Delete(context.Background(), newCRDDeletionRequest(false))
	if err != nil {
		t.Fatalf("Delete() error = %v, want final live check to confirm deletion", err)
	}
	if result.Status != "completed" {
		t.Fatalf("Delete() result = %#v, want completed", result)
	}
	if listCalls != 3 {
		t.Fatalf("widget list calls = %d, want build, poll, and final check", listCalls)
	}
}

func TestK8sResourceDeletionRejectsInvalidResourceBeforeMutation(t *testing.T) {
	orchestrator, client := newDeletionTestOrchestrator(t)
	req := &model.K8sResourceDeletionRequest{
		AppID: "app-a",
		K8sResources: []model.HandleResource{{
			ClientID:     "bad",
			Name:         "bad",
			Kind:         "Widget",
			ResourceYaml: "metadata:\n  name: bad\n",
			State:        model.CreateSuccess,
		}},
	}

	_, err := orchestrator.Delete(context.Background(), req)
	if !errors.Is(err, ErrInvalidK8sResourceDeletionRequest) {
		t.Fatalf("Delete() error = %v, want ErrInvalidK8sResourceDeletionRequest", err)
	}
	for _, action := range client.Actions() {
		if action.GetVerb() == "delete" {
			t.Fatalf("Delete() mutated Kubernetes for invalid input: %#v", action)
		}
	}
}

func TestK8sResourceDeletionDeletesAndConfirmsOrdinaryResource(t *testing.T) {
	configMap := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      "settings",
			"namespace": "team-a",
		},
	}}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{testConfigGVR: "ConfigMapList"},
		configMap,
	)
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)
	orchestrator := &k8sResourceDeletionOrchestrator{
		dynamicClient: client,
		mapper:        mapper,
		refreshMapper: func() (meta.RESTMapper, error) { return mapper, nil },
		waitInterval:  time.Millisecond,
		waitTimeout:   50 * time.Millisecond,
	}

	result, err := orchestrator.Delete(context.Background(), &model.K8sResourceDeletionRequest{
		AppID: "app-a",
		K8sResources: []model.HandleResource{{
			ClientID:     "config-row",
			AppID:        "app-a",
			Namespace:    "team-a",
			Name:         "settings",
			Kind:         "ConfigMap",
			ResourceYaml: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: settings\n  namespace: team-a\n",
			State:        model.CreateSuccess,
		}},
	})
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if len(result.DeletedClientIDs) != 1 || result.DeletedClientIDs[0] != "config-row" {
		t.Fatalf("Delete() result = %#v", result)
	}
	if _, err := client.Resource(testConfigGVR).Namespace("team-a").Get(context.Background(), "settings", metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("ConfigMap still exists, Get() error = %v", err)
	}
}

// capability_id: rainbond.k8s-resource.metadata-reconcile
func TestK8sResourceReconcileDistinguishesMissingAndUnknown(t *testing.T) {
	orchestrator, client := newDeletionTestOrchestrator(t, newTestWidget("present", "team-a", "app-a"))
	client.PrependReactor("get", "widgets", func(action ktesting.Action) (bool, runtime.Object, error) {
		getAction := action.(ktesting.GetAction)
		if getAction.GetName() == "unknown" {
			return true, nil, apierrors.NewForbidden(
				schema.GroupResource{Group: testWidgetGVR.Group, Resource: testWidgetGVR.Resource},
				getAction.GetName(),
				fmt.Errorf("denied"),
			)
		}
		return false, nil, nil
	})

	result := orchestrator.Reconcile(context.Background(), &model.K8sResourceReconcileRequest{
		AppID: "app-a",
		K8sResources: []model.HandleResource{
			{ClientID: "1", Namespace: "team-a", Name: "present", Kind: "Widget", ResourceYaml: testWidgetYAML("present", "team-a"), State: model.CreateSuccess},
			{ClientID: "2", Namespace: "team-a", Name: "missing", Kind: "Widget", ResourceYaml: testWidgetYAML("missing", "team-a"), State: model.CreateSuccess},
			{ClientID: "3", Namespace: "team-a", Name: "unknown", Kind: "Widget", ResourceYaml: testWidgetYAML("unknown", "team-a"), State: model.CreateSuccess},
		},
	})

	if len(result.MissingClientIDs) != 1 || result.MissingClientIDs[0] != "2" {
		t.Fatalf("Reconcile() missing = %v, want [2]", result.MissingClientIDs)
	}
	if len(result.Unknown) != 1 || result.Unknown[0].ClientID != "3" {
		t.Fatalf("Reconcile() unknown = %#v, want client 3", result.Unknown)
	}
}

func TestK8sResourceReconcileTreatsConfirmedNoMatchAsMissing(t *testing.T) {
	client := newDeletionTestDynamicClient(t)
	mapper := meta.NewDefaultRESTMapper(nil)
	orchestrator := &k8sResourceDeletionOrchestrator{
		dynamicClient: client,
		mapper:        mapper,
		refreshMapper: func() (meta.RESTMapper, error) {
			return meta.NewDefaultRESTMapper(nil), nil
		},
		waitInterval: time.Millisecond,
		waitTimeout:  50 * time.Millisecond,
	}

	result := orchestrator.Reconcile(context.Background(), &model.K8sResourceReconcileRequest{
		AppID: "app-a",
		K8sResources: []model.HandleResource{{
			ClientID:     "gone-api",
			Namespace:    "team-a",
			Name:         "missing",
			Kind:         "Widget",
			ResourceYaml: testWidgetYAML("missing", "team-a"),
			State:        model.CreateSuccess,
		}},
	})
	if len(result.MissingClientIDs) != 1 || result.MissingClientIDs[0] != "gone-api" {
		t.Fatalf("Reconcile() missing = %v, want confirmed removed API", result.MissingClientIDs)
	}
}

func TestK8sResourceMetadataMatchesCRDByGroupAndKind(t *testing.T) {
	resources := []dbmodel.K8sResource{
		{Model: dbmodel.Model{ID: 1}, Kind: "Widget", Content: testWidgetYAML("one", "team-a")},
		{Model: dbmodel.Model{ID: 2}, Kind: "Widget", Content: "apiVersion: other.example.com/v1\nkind: Widget\nmetadata:\n  name: two\n"},
		{Model: dbmodel.Model{ID: 3}, Kind: "Other", Content: "apiVersion: example.com/v1\nkind: Other\nmetadata:\n  name: three\n"},
		{Model: dbmodel.Model{ID: 4}, Kind: "Widget", Content: ": invalid"},
	}
	impacts := []model.CRDDeletionImpact{{Group: "example.com", Kind: "Widget"}}

	ids := matchingCRDMetadataIDs(resources, impacts)
	if len(ids) != 1 || ids[0] != 1 {
		t.Fatalf("matchingCRDMetadataIDs() = %v, want [1]", ids)
	}
}

func TestCleanupDeletedResourceMetadataIncludesSelectedAndCascadedRows(t *testing.T) {
	k8sDAO := &deletionTestK8sResourceDAO{
		byApp: []dbmodel.K8sResource{
			{Model: dbmodel.Model{ID: 10}, AppID: "app-a", Name: "widgets.example.com", Kind: "CustomResourceDefinition", Content: testCRDYAML()},
			{Model: dbmodel.Model{ID: 11}, AppID: "app-a", Name: "keep", Kind: "ConfigMap", Content: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: keep\n"},
		},
		byKind: []dbmodel.K8sResource{
			{Model: dbmodel.Model{ID: 20}, AppID: "app-b", Name: "shared", Kind: "Widget", Content: testWidgetYAML("shared", "team-b")},
			{Model: dbmodel.Model{ID: 21}, AppID: "app-c", Name: "other", Kind: "Widget", Content: "apiVersion: other.example.com/v1\nkind: Widget\nmetadata:\n  name: other\n"},
		},
	}
	db.SetTestManager(deletionTestDBManager{k8sDAO: k8sDAO})
	defer db.SetTestManager(nil)

	err := cleanupDeletedResourceMetadata(newCRDDeletionRequest(true), &model.K8sResourceDeletionResult{
		CascadedCRDs: []model.CRDDeletionImpact{{Group: "example.com", Kind: "Widget"}},
	})
	if err != nil {
		t.Fatalf("cleanupDeletedResourceMetadata() error = %v", err)
	}
	if len(k8sDAO.deletedIDs) != 2 || k8sDAO.deletedIDs[0] != 10 || k8sDAO.deletedIDs[1] != 20 {
		t.Fatalf("cleanupDeletedResourceMetadata() deleted = %v, want [10 20]", k8sDAO.deletedIDs)
	}
}

func TestClusterActionReconcileCleansOnlyConfirmedMissingRegionMetadata(t *testing.T) {
	orchestrator, _ := newDeletionTestOrchestrator(t)
	k8sDAO := &deletionTestK8sResourceDAO{}
	db.SetTestManager(deletionTestDBManager{k8sDAO: k8sDAO})
	defer db.SetTestManager(nil)
	action := &clusterAction{dynamicClient: orchestrator.dynamicClient, mapper: orchestrator.mapper}

	result, handleErr := action.ReconcileK8SResources(context.Background(), &model.K8sResourceReconcileRequest{
		AppID: "app-a",
		K8sResources: []model.HandleResource{{
			ClientID:     "missing-row",
			Name:         "missing",
			Kind:         "Widget",
			Namespace:    "team-a",
			ResourceYaml: testWidgetYAML("missing", "team-a"),
			State:        model.CreateSuccess,
		}},
	})
	if handleErr != nil {
		t.Fatalf("ReconcileK8SResources() error = %v", handleErr)
	}
	if len(result.MissingClientIDs) != 1 || len(k8sDAO.deletedResources) != 1 {
		t.Fatalf("ReconcileK8SResources() result = %#v, deleted = %v", result, k8sDAO.deletedResources)
	}
	if k8sDAO.deletedResources[0] != "app-a/missing/Widget" {
		t.Fatalf("deleted metadata = %v", k8sDAO.deletedResources)
	}
}

func newDeletionTestOrchestrator(t *testing.T, widgets ...*unstructured.Unstructured) (*k8sResourceDeletionOrchestrator, *dynamicfake.FakeDynamicClient) {
	t.Helper()
	objects := []runtime.Object{newTestCRD()}
	for _, widget := range widgets {
		objects = append(objects, widget)
	}
	client := newDeletionTestDynamicClient(t, objects...)
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{
		{Group: "apiextensions.k8s.io", Version: "v1"},
		{Group: "example.com", Version: "v1"},
	})
	mapper.Add(schema.GroupVersionKind{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"}, meta.RESTScopeRoot)
	mapper.Add(schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Widget"}, meta.RESTScopeNamespace)
	orchestrator := &k8sResourceDeletionOrchestrator{
		dynamicClient: client,
		mapper:        mapper,
		refreshMapper: func() (meta.RESTMapper, error) { return mapper, nil },
		waitInterval:  time.Millisecond,
		waitTimeout:   100 * time.Millisecond,
	}
	return orchestrator, client
}

func newDeletionTestDynamicClient(t *testing.T, objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	t.Helper()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{
			testCRDGVR:    "CustomResourceDefinitionList",
			testWidgetGVR: "WidgetList",
		},
		objects...,
	)
	return client
}

func newTestCRD() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata": map[string]interface{}{
			"name": "widgets.example.com",
			"uid":  string(types.UID("crd-uid")),
		},
		"spec": map[string]interface{}{
			"group": "example.com",
			"scope": "Namespaced",
			"names": map[string]interface{}{
				"kind":   "Widget",
				"plural": "widgets",
			},
			"versions": []interface{}{
				map[string]interface{}{"name": "v1", "served": true, "storage": true},
			},
		},
	}}
}

func newTestWidget(name, namespace, appID string) *unstructured.Unstructured {
	labels := map[string]interface{}{}
	if appID != "" {
		labels["app_id"] = appID
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "example.com/v1",
		"kind":       "Widget",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"labels":    labels,
		},
	}}
}

func newCRDDeletionRequest(cascade bool) *model.K8sResourceDeletionRequest {
	return &model.K8sResourceDeletionRequest{
		AppID:      "app-a",
		CascadeCRD: cascade,
		K8sResources: []model.HandleResource{{
			ClientID:     "crd-row",
			AppID:        "app-a",
			Name:         "widgets.example.com",
			Kind:         "CustomResourceDefinition",
			ResourceYaml: testCRDYAML(),
			State:        model.CreateSuccess,
		}},
	}
}

func testCRDYAML() string {
	return `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
spec:
  group: example.com
  scope: Namespaced
  names:
    kind: Widget
    plural: widgets
  versions:
  - name: v1
    served: true
    storage: true
`
}

func testWidgetYAML(name, namespace string) string {
	return fmt.Sprintf(`apiVersion: example.com/v1
kind: Widget
metadata:
  name: %s
  namespace: %s
`, name, namespace)
}

// Compile-time checks keep the fake test dependencies honest across client-go upgrades.
var _ dynamic.Interface = (*dynamicfake.FakeDynamicClient)(nil)

type deletionTestDBManager struct {
	db.Manager
	k8sDAO dao.K8sResourceDao
}

func (m deletionTestDBManager) K8sResourceDao() dao.K8sResourceDao {
	return m.k8sDAO
}

type deletionTestK8sResourceDAO struct {
	dao.K8sResourceDao
	byApp            []dbmodel.K8sResource
	byKind           []dbmodel.K8sResource
	deletedIDs       []uint
	deletedResources []string
}

func (d *deletionTestK8sResourceDAO) ListByAppID(string) ([]dbmodel.K8sResource, error) {
	return d.byApp, nil
}

func (d *deletionTestK8sResourceDAO) ListByKind(string) ([]dbmodel.K8sResource, error) {
	return d.byKind, nil
}

func (d *deletionTestK8sResourceDAO) DeleteK8sResourceByIDs(ids []uint) error {
	d.deletedIDs = append(d.deletedIDs, ids...)
	return nil
}

func (d *deletionTestK8sResourceDAO) DeleteK8sResource(appID, name, kind string) error {
	d.deletedResources = append(d.deletedResources, appID+"/"+name+"/"+kind)
	return nil
}

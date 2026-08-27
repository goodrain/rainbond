// RAINBOND, Application Management Platform
// Copyright (C) 2026 Goodrain Co., Ltd.

package handle

import (
	"context"
	"fmt"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/fake"
)

// capability_id: rainbond.k8s-resource.force-delete-options
func TestK8sResourceDeleteUsesForceBackgroundOptions(t *testing.T) {
	options := forceDeleteOptions(types.UID("resource-uid"))
	if options.GracePeriodSeconds == nil || *options.GracePeriodSeconds != 0 {
		t.Fatalf("expected forced deletion grace period 0, got %#v", options.GracePeriodSeconds)
	}
	if options.PropagationPolicy == nil || *options.PropagationPolicy != metav1.DeletePropagationBackground {
		t.Fatalf("expected background propagation, got %#v", options.PropagationPolicy)
	}
	if options.Preconditions == nil || options.Preconditions.UID == nil || *options.Preconditions.UID != types.UID("resource-uid") {
		t.Fatalf("expected UID precondition, got %#v", options.Preconditions)
	}
}

// capability_id: rainbond.k8s-resource.uid-force-delete
func TestK8sResourceDeleteRequiresObservedUIDAndActualGetNotFound(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "example.io", Version: "v1", Resource: "widgets"}
	client := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{gvr: "WidgetList"})
	resourceClient := client.Resource(gvr).Namespace("default")

	deleted, err := waitForK8sResourceDeletion(context.Background(), resourceClient, "missing", types.UID("expected-uid"), dbmodel.K8sResourceOriginUser, true)
	if err != nil || !deleted {
		t.Fatalf("only an actual Get NotFound may complete deletion: deleted=%v err=%v", deleted, err)
	}

	if _, err := resourceClient.Create(context.Background(), &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "example.io/v1",
		"kind":       "Widget",
		"metadata": map[string]interface{}{
			"name":      "recreated",
			"namespace": "default",
			"uid":       "new-object-uid",
		},
	}}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create fake recreated object: %v", err)
	}
	deleted, err = waitForK8sResourceDeletion(context.Background(), resourceClient, "recreated", types.UID("old-object-uid"), dbmodel.K8sResourceOriginUser, true)
	if deleted {
		t.Fatal("UID mismatch must never report deletion complete")
	}
	if !isPermanentK8sResourceDeleteError(err) {
		t.Fatalf("UID mismatch must be terminal rather than retrying: %v", err)
	}
}

// capability_id: rainbond.k8s-resource.finalizer-force-policy
func TestK8sResourceDeleteFinalizerPolicyHasExplicitOriginBoundary(t *testing.T) {
	testCases := []struct {
		name     string
		resource dbmodel.K8sResource
		allowed  bool
	}{
		{name: "user", resource: dbmodel.K8sResource{ResourceOrigin: dbmodel.K8sResourceOriginUser, ForceFinalizerAllowed: true}, allowed: true},
		{name: "market", resource: dbmodel.K8sResource{ResourceOrigin: dbmodel.K8sResourceOriginMarket, ForceFinalizerAllowed: true}, allowed: true},
		{name: "imported", resource: dbmodel.K8sResource{ResourceOrigin: dbmodel.K8sResourceOriginImported, ForceFinalizerAllowed: true}, allowed: true},
		{name: "governance", resource: dbmodel.K8sResource{ResourceOrigin: dbmodel.K8sResourceOriginGovernance, ForceFinalizerAllowed: true}, allowed: false},
		{name: "gateway", resource: dbmodel.K8sResource{ResourceOrigin: dbmodel.K8sResourceOriginGateway, ForceFinalizerAllowed: true}, allowed: false},
		{name: "legacy", resource: dbmodel.K8sResource{ResourceOrigin: dbmodel.K8sResourceOriginLegacy, ForceFinalizerAllowed: true}, allowed: false},
		{name: "flag disabled", resource: dbmodel.K8sResource{ResourceOrigin: dbmodel.K8sResourceOriginUser, ForceFinalizerAllowed: false}, allowed: false},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := testCase.resource.AllowsForceFinalizerRemoval(); got != testCase.allowed {
				t.Fatalf("expected finalizer removal allowed=%t, got %t", testCase.allowed, got)
			}
		})
	}
}

// capability_id: rainbond.k8s-resource.fixed-delete-worker-pool
func TestK8sResourceDeleteWorkerCountIsBounded(t *testing.T) {
	if got := k8sResourceDeleteWorkerCount(0); got != 0 {
		t.Fatalf("expected no worker for empty batch, got %d", got)
	}
	if got := k8sResourceDeleteWorkerCount(3); got != 3 {
		t.Fatalf("expected 3 workers for 3 targets, got %d", got)
	}
	if got := k8sResourceDeleteWorkerCount(100); got != k8sResourceDeleteConcurrency {
		t.Fatalf("expected cap %d, got %d", k8sResourceDeleteConcurrency, got)
	}
}

// capability_id: rainbond.k8s-resource.mapper-refresh-delete
func TestK8sResourceDeleteRefreshesMapperAndRetriesResolution(t *testing.T) {
	groupKind := schema.GroupKind{Group: "example.io", Kind: "Widget"}
	groupVersionKind := schema.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "Widget"}
	resource := schema.GroupVersionResource{Group: "example.io", Version: "v1", Resource: "widgets"}
	refreshed := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "example.io", Version: "v1"}})
	refreshed.Add(groupVersionKind, meta.RESTScopeNamespace)

	first := &noMatchRESTMapper{}
	mapping, err := resolveK8sResourceMapping(first, groupKind, "v1", func() (meta.RESTMapper, error) {
		return refreshed, nil
	})
	if err != nil {
		t.Fatalf("expected mapper refresh to retry resolution, got %v", err)
	}
	if mapping.Resource != resource {
		t.Fatalf("expected refreshed resource mapping %#v, got %#v", resource, mapping.Resource)
	}
	if first.calls != 1 {
		t.Fatalf("expected one initial no-match lookup, got %d", first.calls)
	}
}

// capability_id: rainbond.k8s-resource.mapper-refresh-delete
func TestK8sResourceDeleteFallsBackToServedCRDVersion(t *testing.T) {
	groupKind := schema.GroupKind{Group: "extensions.kubeblocks.io", Kind: "Addon"}
	served := schema.GroupVersionKind{Group: "extensions.kubeblocks.io", Version: "v1alpha2", Kind: "Addon"}
	refreshed := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: served.Group, Version: served.Version}})
	refreshed.Add(served, meta.RESTScopeNamespace)

	mapping, err := resolveK8sResourceMapping(&noMatchRESTMapper{}, groupKind, "v1alpha1", func() (meta.RESTMapper, error) {
		return refreshed, nil
	})
	if err != nil {
		t.Fatalf("expected deletion to resolve the currently served CRD version, got %v", err)
	}
	if mapping.Resource != (schema.GroupVersionResource{Group: served.Group, Version: served.Version, Resource: "addons"}) {
		t.Fatalf("expected Addon to resolve through served version %q, got %#v", served.Version, mapping.Resource)
	}
}

// capability_id: rainbond.k8s-resource.missing-crd-delete-completion
func TestK8sResourceDeleteCompletesOnlyWhenMatchingCRDIsAbsent(t *testing.T) {
	groupKind := schema.GroupKind{Group: "extensions.kubeblocks.io", Kind: "Addon"}
	client := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		customResourceDefinitionGVR: "CustomResourceDefinitionList",
	})

	missing, err := isK8sCustomResourceDefinitionMissing(context.Background(), client, groupKind)
	if err != nil {
		t.Fatalf("verify absent CRD: %v", err)
	}
	if !missing {
		t.Fatal("a GroupKind with no matching CRD must be identified as removed")
	}

	matchingCRD := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata": map[string]interface{}{
			"name": "addons.extensions.kubeblocks.io",
		},
		"spec": map[string]interface{}{
			"group": groupKind.Group,
			"names": map[string]interface{}{
				"kind":   groupKind.Kind,
				"plural": "addons",
			},
		},
	}}
	client = fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		customResourceDefinitionGVR: "CustomResourceDefinitionList",
	}, matchingCRD)
	missing, err = isK8sCustomResourceDefinitionMissing(context.Background(), client, groupKind)
	if err != nil {
		t.Fatalf("verify present CRD: %v", err)
	}
	if missing {
		t.Fatal("a matching CRD must keep deletion in the physical deletion path")
	}
}

type noMatchRESTMapper struct {
	calls int
}

func (m *noMatchRESTMapper) KindFor(schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, fmt.Errorf("not implemented")
}

func (m *noMatchRESTMapper) KindsFor(schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *noMatchRESTMapper) ResourceFor(schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	return schema.GroupVersionResource{}, fmt.Errorf("not implemented")
}

func (m *noMatchRESTMapper) ResourcesFor(schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *noMatchRESTMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	m.calls++
	return nil, &meta.NoKindMatchError{GroupKind: gk, SearchedVersions: versions}
}

func (m *noMatchRESTMapper) RESTMappings(schema.GroupKind, ...string) ([]*meta.RESTMapping, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *noMatchRESTMapper) ResourceSingularizer(string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

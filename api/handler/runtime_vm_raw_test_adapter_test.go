package handler

import (
	"context"
	"testing"

	"github.com/goodrain/rainbond/pkg/vmrestore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	ktesting "k8s.io/client-go/testing"
	kv "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
)

// attachRawRuntimeObservation supplies a real dynamic fake alongside legacy
// typed Update assertions. The target-only field must survive the actual raw
// update before the existing typed callback is consulted.
func attachRawRuntimeObservation(t *testing.T, client *kubecli.MockKubevirtClient, vm *kv.VirtualMachine) {
	t.Helper()
	if vm.UID == "" {
		vm.UID = "runtime-test-uid"
	}
	if vm.ResourceVersion == "" {
		vm.ResourceVersion = "7"
	}
	raw, err := vmrestore.EncodeRuntimeVM(vm)
	if err != nil {
		t.Fatal(err)
	}
	_ = unstructured.SetNestedField(raw.Object, "Terminate", "spec", "template", "spec", "domain", "rebootPolicy")
	dynamic := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), raw)
	client.EXPECT().DynamicClient().Return(dynamic).AnyTimes()
	dynamic.PrependReactor("update", "virtualmachines", func(action ktesting.Action) (bool, runtime.Object, error) {
		object := action.(ktesting.UpdateAction).GetObject().(*unstructured.Unstructured)
		value, _, _ := unstructured.NestedString(object.Object, "spec", "template", "spec", "domain", "rebootPolicy")
		if value != "Terminate" {
			t.Fatal("ordinary typed entrypoint lost target-only field")
		}
		var typed kv.VirtualMachine
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, &typed); err != nil {
			t.Fatal(err)
		}
		if _, err := client.VirtualMachine(vm.Namespace).Update(context.Background(), &typed, metav1.UpdateOptions{}); err != nil {
			return true, nil, err
		}
		if err := dynamic.Tracker().Update(schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachines"}, object, object.GetNamespace()); err != nil {
			return true, nil, err
		}
		return true, object, nil
	})
}

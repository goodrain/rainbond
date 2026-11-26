package store

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// capability_id: rainbond.vm-power.runtime-observer-retry
func TestVMAdmissionObserverRechecksUnchangedRuntimeAndVMOnlyUpdates(t *testing.T) {
	vmGVR := schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachines"}
	vmiGVR := schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachineinstances"}
	objects := []runtime.Object{}
	for _, kind := range []string{"VirtualMachine", "VirtualMachineInstance"} {
		objects = append(objects, &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "kubevirt.io/v1", "kind": kind,
			"metadata": map[string]interface{}{"name": "vm", "namespace": "tenant", "resourceVersion": "1"},
		}})
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{vmGVR: "VirtualMachineList", vmiGVR: "VirtualMachineInstanceList"}, objects...)
	stop := make(chan struct{})
	defer close(stop)
	observations := make(chan string, 32)
	startVMRuntimeObservers(client, nil, time.Second, stop, func(value interface{}) {
		object := value.(*unstructured.Unstructured)
		select {
		case observations <- object.GetKind() + ":" + object.GetResourceVersion():
		default:
		}
	})
	counts := map[string]int{}
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for counts["VirtualMachine:1"] < 2 || counts["VirtualMachineInstance:1"] < 2 {
		select {
		case observed := <-observations:
			counts[observed]++
		case <-deadline.C:
			t.Fatalf("unchanged runtime was not retried after an initial pending observation: %#v", counts)
		}
	}
	vm, err := client.Resource(vmGVR).Namespace("tenant").Get(context.Background(), "vm", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	vm.SetResourceVersion("2")
	if _, err := client.Resource(vmGVR).Namespace("tenant").Update(context.Background(), vm, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case observed := <-observations:
			if observed == "VirtualMachine:2" {
				return
			}
		case <-deadline.C:
			t.Fatal("VM-only updates did not trigger power completion observation")
		}
	}
}

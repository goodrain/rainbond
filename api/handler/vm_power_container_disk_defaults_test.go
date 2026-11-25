package handler

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util/bcode"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kv "kubevirt.io/api/core/v1"
)

// capability_id: rainbond.vm-power.container-disk-defaults
func TestRestoredPowerCompletionHandlesContainerDiskDefaults(t *testing.T) {
	for _, tc := range []struct {
		name      string
		image     string
		requested corev1.PullPolicy
		actual    corev1.PullPolicy
		changed   bool
		complete  bool
	}{
		{name: "digest", image: "registry.invalid/virtio@sha256:" + strings.Repeat("b", 64), actual: corev1.PullIfNotPresent, complete: true},
		{name: "version tag", image: "registry.invalid/virtio:v1", actual: corev1.PullIfNotPresent, complete: true},
		{name: "latest tag", image: "registry.invalid/virtio:latest", actual: corev1.PullAlways, complete: true},
		{name: "implicit latest", image: "registry.invalid/virtio", actual: corev1.PullAlways, complete: true},
		{name: "explicit policy", image: "registry.invalid/virtio:v1", requested: corev1.PullNever, actual: corev1.PullNever, complete: true},
		{name: "wrong default policy", image: "registry.invalid/virtio:v1", actual: corev1.PullAlways},
		{name: "changed explicit policy", image: "registry.invalid/virtio:v1", requested: corev1.PullNever, actual: corev1.PullIfNotPresent},
		{name: "changed image", image: "registry.invalid/virtio:v1", actual: corev1.PullIfNotPresent, changed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, store, resolver, scope, dyn := restoredPowerFixture(t)
			ctx := context.Background()
			vmGVR := schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachines"}
			vmiGVR := schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachineinstances"}
			object, err := dyn.Resource(vmGVR).Namespace(scope.Namespace).Get(ctx, "vm", metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			var vm kv.VirtualMachine
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, &vm); err != nil {
				t.Fatal(err)
			}
			vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, kv.Volume{Name: "virtio", VolumeSource: kv.VolumeSource{ContainerDisk: &kv.ContainerDiskSource{Image: tc.image, ImagePullPolicy: tc.requested}}})
			vm.Spec.Template.Spec.Domain.Devices.Disks = append(vm.Spec.Template.Spec.Domain.Devices.Disks, kv.Disk{Name: "virtio", DiskDevice: kv.DiskDevice{CDRom: &kv.CDRomTarget{Bus: kv.DiskBusSATA}}})
			object, err = vmrestore.EncodeRuntimeVM(&vm)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := dyn.Resource(vmGVR).Namespace(scope.Namespace).Update(ctx, object, metav1.UpdateOptions{}); err != nil {
				t.Fatal(err)
			}
			eventID := "container-disk-start"
			if err := a.getDBManager().DB().Create(&dbmodel.ServiceEvent{EventID: eventID, TenantID: scope.TenantID, ServiceID: scope.ServiceID, Status: "checking", FinalStatus: "running"}).Error; err != nil {
				t.Fatal(err)
			}
			err = a.StartOrCreateVM(newVMOperationEventContext(eventID), &apimodel.StartStopStruct{TenantID: scope.TenantID, ServiceID: scope.ServiceID, EventID: eventID, TaskType: "start"}, "")
			var pending *bcode.Code
			if !errors.As(err, &pending) || pending.Status != 202 {
				t.Fatalf("start should await the real instance: %v", err)
			}
			object, err = dyn.Resource(vmGVR).Namespace(scope.Namespace).Get(ctx, "vm", metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, &vm); err != nil {
				t.Fatal(err)
			}
			instance := &kv.VirtualMachineInstance{TypeMeta: metav1.TypeMeta{APIVersion: "kubevirt.io/v1", Kind: "VirtualMachineInstance"}, ObjectMeta: metav1.ObjectMeta{Name: vm.Name, Namespace: scope.Namespace, UID: "defaulted-instance", OwnerReferences: []metav1.OwnerReference{{Kind: "VirtualMachine", Name: vm.Name, UID: vm.UID}}}, Spec: *vm.Spec.Template.Spec.DeepCopy(), Status: kv.VirtualMachineInstanceStatus{Phase: kv.Running}}
			for i := range instance.Spec.Volumes {
				if disk := instance.Spec.Volumes[i].ContainerDisk; disk != nil {
					disk.ImagePullPolicy = tc.actual
					if tc.changed {
						disk.Image = "registry.invalid/virtio:changed"
					}
				}
			}
			kv.SetDefaults_VirtualMachineInstance(instance)
			raw, err := json.Marshal(instance)
			if err != nil {
				t.Fatal(err)
			}
			instanceObject := object.DeepCopy()
			if err := instanceObject.UnmarshalJSON(raw); err != nil {
				t.Fatal(err)
			}
			if _, err := dyn.Resource(vmiGVR).Namespace(scope.Namespace).Create(ctx, instanceObject, metav1.CreateOptions{}); err != nil {
				t.Fatal(err)
			}
			err = store.ObserveOrdinaryVMCompletion(ctx, resolver, dyn, scope)
			current, readErr := store.ReadOnly(scope, "")
			if readErr != nil {
				t.Fatal(readErr)
			}
			var event dbmodel.ServiceEvent
			if readErr := a.getDBManager().DB().Where("event_id = ?", eventID).Take(&event).Error; readErr != nil {
				t.Fatal(readErr)
			}
			if tc.complete {
				if err != nil || current.State.Admission != nil || event.Status != "success" || event.FinalStatus != "complete" || event.EndTime == "" {
					t.Fatalf("defaulted running instance did not close the event and release snapshot admission: %v, %#v, %#v", err, current.State.Admission, event)
				}
			} else if err == nil || current.State.Admission == nil || event.Status != "checking" {
				t.Fatalf("changed media incorrectly completed the operation: %v, %#v, %#v", err, current.State.Admission, event)
			}
			stored, err := dyn.Resource(vmGVR).Namespace(scope.Namespace).Get(ctx, "vm", metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			var unchanged kv.VirtualMachine
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(stored.Object, &unchanged); err != nil {
				t.Fatal(err)
			}
			for _, volume := range unchanged.Spec.Template.Spec.Volumes {
				if volume.ContainerDisk != nil && volume.ContainerDisk.ImagePullPolicy != tc.requested {
					t.Fatal("completion observation rewrote the VM template")
				}
			}
		})
	}
}

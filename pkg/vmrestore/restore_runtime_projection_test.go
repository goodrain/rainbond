package vmrestore

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubefake "k8s.io/client-go/kubernetes/fake"
	kv "kubevirt.io/api/core/v1"
)

// capability_id: rainbond.vm-restore.runtime-projection
func TestRestoredProjectionPreservesAdvancedMediaAndNetworkWithoutBInitialization(t *testing.T) {
	f, _, _ := resourceFixture(t)
	before := testStartupPolicy("platform")
	before.GeneratedMedia = []RestoreGeneratedMedia{{VolumeName: "generated-env", DiskName: "generated-env", ProducerUID: "producer-uid", Source: ObjectRef{Kind: "ConfigMap", Namespace: f.scope.Namespace, Name: "b-generated-env", UID: "b-env-uid"}}}
	core := kubefake.NewSimpleClientset(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: f.scope.Namespace, Name: "b-generated-env", UID: "b-env-uid"}})
	vm := &kv.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "vm-a", Namespace: f.scope.Namespace, UID: "vm-b"}, Spec: kv.VirtualMachineSpec{Template: &kv.VirtualMachineInstanceTemplateSpec{Spec: kv.VirtualMachineInstanceSpec{Domain: kv.DomainSpec{Devices: kv.Devices{Interfaces: []kv.Interface{{Name: "default", MacAddress: "02:00:00:00:00:42"}}, Disks: []kv.Disk{{Name: "manual7"}, {Name: "manual8"}, {Name: "manual9"}, {Name: before.NoCloudDiskName}, {Name: "generated-env"}, {Name: "user-secret"}}}}, Volumes: []kv.Volume{{Name: "manual7", VolumeSource: kv.VolumeSource{PersistentVolumeClaim: &kv.PersistentVolumeClaimVolumeSource{PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: "manual7"}}}}, {Name: "manual8"}, {Name: "manual9"}, {Name: before.NoCloudVolumeName, VolumeSource: kv.VolumeSource{CloudInitNoCloud: &kv.CloudInitNoCloudSource{UserData: "B_INITIALIZATION_SENTINEL"}}}, {Name: "generated-env", VolumeSource: kv.VolumeSource{ConfigMap: &kv.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "b-generated-env"}}}}, {Name: "user-secret", VolumeSource: kv.VolumeSource{Secret: &kv.SecretVolumeSource{SecretName: "keep-user-advanced"}}}}, Networks: []kv.Network{{Name: "default", NetworkSource: kv.NetworkSource{Pod: &kv.PodNetwork{}}}}}}}}
	bindings, err := BuildRestoreBindings(f.scope, f.expect.OperationID, f.intent.SourceSnapshot.UID, f.old, f.after, f.targets)
	if err != nil {
		t.Fatal(err)
	}
	selection := RuntimeSelection{Scope: f.scope, BindingGeneration: 1, Bindings: bindings, Config: f.after, Startup: testStartupPolicy("restored"), BeforeStartup: before}
	projected, err := ProjectRestoredRuntime(context.Background(), core, vm, selection)
	if err != nil {
		t.Fatal(err)
	}
	if projected.Spec.Template.Spec.Domain.Devices.Interfaces[0].MacAddress != "02:00:00:00:00:42" || len(projected.Spec.Template.Spec.Networks) != 1 {
		t.Fatal("projection changed current network")
	}
	seen := map[string]kv.Volume{}
	for _, v := range projected.Spec.Template.Spec.Volumes {
		seen[v.Name] = v
	}
	if seen["manual7"].PersistentVolumeClaim == nil || seen["manual7"].PersistentVolumeClaim.ClaimName != "restore-root-a" || seen["user-secret"].Secret == nil {
		t.Fatal("projection lost active A or user advanced device")
	}
	if _, exists := seen["manual9"]; exists {
		t.Fatal("detached B-only disk was reattached")
	}
	if _, exists := seen["generated-env"]; exists {
		t.Fatal("B generated media was reinjected")
	}
	cloud := seen[selection.Startup.NoCloudVolumeName].CloudInitNoCloud
	if cloud == nil || strings.Contains(cloud.UserData, "B_INITIALIZATION_SENTINEL") {
		t.Fatal("B authentication initialization survived A projection")
	}
	if projected.Spec.Template.Spec.Domain.Firmware == nil || projected.Spec.Template.Spec.Domain.Firmware.UUID != types.UID(selection.Startup.FirmwareUUID) {
		t.Fatal("A firmware identity was replaced")
	}
	if !strings.Contains(vm.Spec.Template.Spec.Volumes[3].CloudInitNoCloud.UserData, "B_INITIALIZATION_SENTINEL") {
		t.Fatal("projection mutated original input")
	}
}

func TestRestoredProjectionRejectsUnprovenGeneratedMedia(t *testing.T) {
	f, _, _ := resourceFixture(t)
	before := testStartupPolicy("platform")
	before.GeneratedMedia = []RestoreGeneratedMedia{{VolumeName: "generated-env", DiskName: "generated-env", ProducerUID: "producer-uid", Source: ObjectRef{Kind: "ConfigMap", Namespace: f.scope.Namespace, Name: "env", UID: "original-env-uid"}}}
	core := kubefake.NewSimpleClientset(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: f.scope.Namespace, Name: "env", UID: "replacement-uid"}})
	vm := &kv.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Namespace: f.scope.Namespace}, Spec: kv.VirtualMachineSpec{Template: &kv.VirtualMachineInstanceTemplateSpec{Spec: kv.VirtualMachineInstanceSpec{Volumes: []kv.Volume{{Name: "generated-env", VolumeSource: kv.VolumeSource{ConfigMap: &kv.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "env"}}}}}}}}}
	if _, err := ProjectRestoredRuntime(context.Background(), core, vm, RuntimeSelection{Scope: f.scope, Config: f.after, Startup: testStartupPolicy("restored"), BeforeStartup: before, Bindings: f.old}); err == nil {
		t.Fatal("replacement advanced media was silently treated as platform generated")
	}
}

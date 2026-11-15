package handler

import (
	"context"
	"encoding/json"
	"testing"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/pkg/vmstorage"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamic "k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	v1 "kubevirt.io/api/core/v1"
)

var (
	testSnapshotClassGVR  = schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshotclasses"}
	testVolumeSnapshotGVR = schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"}
	testVMSnapshotGVR     = schema.GroupVersionResource{Group: "snapshot.kubevirt.io", Version: "v1beta1", Resource: "virtualmachinesnapshots"}
	testKubeVirtGVR       = schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "kubevirts"}
)

func snapshotCapabilityReadyPod(name string, containers ...corev1.Container) *corev1.Pod {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "infra"}, Spec: corev1.PodSpec{Containers: containers}, Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}}
	for _, container := range containers {
		pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, corev1.ContainerStatus{Name: container.Name, Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}})
	}
	return pod
}

func snapshotCapabilityClass(name, driver string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "snapshot.storage.k8s.io/v1",
		"kind":       "VolumeSnapshotClass",
		"metadata": map[string]interface{}{
			"name": name,
			"annotations": map[string]interface{}{
				"snapshot.storage.kubernetes.io/is-default-class": "true",
			},
		},
		"driver": driver, "deletionPolicy": "Delete",
	}}
}

func longhornSnapshotCapabilityFixture(t *testing.T) (*ServiceAction, *k8sfake.Clientset, *dynamicfake.FakeDynamicClient, string, string) {
	t.Helper()
	action, _, _, scope := snapshotFacadeFixture(t)
	ctx := context.Background()
	driver, storageClass := vmstorage.LonghornSnapshotDriver, "longhorn-vm-snapshot"
	dvUID, pvcUID := types.UID("dv-uid"), types.UID("pvc-uid")
	controller := true
	core := k8sfake.NewSimpleClientset(
		&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: storageClass}, Provisioner: driver},
		&storagev1.CSIDriver{ObjectMeta: metav1.ObjectMeta{Name: driver}},
		snapshotCapabilityReadyPod("snapshot-controller", corev1.Container{Name: "controller", Image: "registry/snapshot-controller:v1"}),
		snapshotCapabilityReadyPod("longhorn-csi", corev1.Container{Name: "driver", Args: []string{"--drivername=" + driver}}, corev1.Container{Name: "snapshotter", Image: "registry/csi-snapshotter:v1"}),
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "manual382", Namespace: scope.Namespace, UID: pvcUID, OwnerReferences: []metav1.OwnerReference{{Kind: "DataVolume", UID: dvUID, Controller: &controller}}}, Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &storageClass, VolumeName: "pv-manual382"}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}},
		&corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "pv-manual382"}, Spec: corev1.PersistentVolumeSpec{ClaimRef: &corev1.ObjectReference{Namespace: scope.Namespace, Name: "manual382", UID: pvcUID}, PersistentVolumeSource: corev1.PersistentVolumeSource{CSI: &corev1.CSIPersistentVolumeSource{Driver: driver, VolumeHandle: "longhorn-volume"}}}},
	)
	vm := &v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "windows", Namespace: scope.Namespace, UID: "vm-uid", ResourceVersion: "1", Labels: map[string]string{"service_id": scope.ServiceID, "tenant_id": scope.TenantID}}, Spec: v1.VirtualMachineSpec{Template: &v1.VirtualMachineInstanceTemplateSpec{Spec: v1.VirtualMachineInstanceSpec{Volumes: []v1.Volume{
		{Name: "manual382", VolumeSource: v1.VolumeSource{DataVolume: &v1.DataVolumeSource{Name: "manual382"}}},
		{Name: "vmimage", VolumeSource: v1.VolumeSource{ContainerDisk: &v1.ContainerDiskSource{Image: "windows-installer"}}},
		{Name: "virtio", VolumeSource: v1.VolumeSource{ContainerDisk: &v1.ContainerDiskSource{Image: "virtio-drivers"}}},
		{Name: "rainbond-env", VolumeSource: v1.VolumeSource{ConfigMap: &v1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "env"}}}},
	}, Domain: v1.DomainSpec{Devices: v1.Devices{Disks: []v1.Disk{
		{Name: "manual382", DiskDevice: v1.DiskDevice{Disk: &v1.DiskTarget{Bus: v1.DiskBusSCSI}}},
		{Name: "vmimage", DiskDevice: v1.DiskDevice{CDRom: &v1.CDRomTarget{}}},
		{Name: "virtio", DiskDevice: v1.DiskDevice{CDRom: &v1.CDRomTarget{}}},
	}}}}}}}
	vmObject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
	if err != nil {
		t.Fatal(err)
	}
	vmObject["apiVersion"], vmObject["kind"] = "kubevirt.io/v1", "VirtualMachine"
	dv := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "cdi.kubevirt.io/v1beta1", "kind": "DataVolume", "metadata": map[string]interface{}{"name": "manual382", "namespace": scope.Namespace, "uid": string(dvUID)}, "status": map[string]interface{}{"claimName": "manual382"}}}
	kubeVirt := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "kubevirt.io/v1", "kind": "KubeVirt", "metadata": map[string]interface{}{"name": "virt", "namespace": "infra"}, "status": map[string]interface{}{"phase": "Deployed", "conditions": []interface{}{map[string]interface{}{"type": "Available", "status": "True"}, map[string]interface{}{"type": "Degraded", "status": "False"}}}}}
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		vmDiskVMGVR: "VirtualMachineList", dataVolumeGVR: "DataVolumeList", testSnapshotClassGVR: "VolumeSnapshotClassList", testVolumeSnapshotGVR: "VolumeSnapshotList", testVMSnapshotGVR: "VirtualMachineSnapshotList", testKubeVirtGVR: "KubeVirtList",
	}, &unstructured.Unstructured{Object: vmObject}, dv, kubeVirt, snapshotCapabilityClass("longhorn-vm-snapshot", driver))
	action.kubeClient = core
	original := vmHotplugDynamicClient
	vmHotplugDynamicClient = func() dynamic.Interface { return dynamicClient }
	t.Cleanup(func() { vmHotplugDynamicClient = original })
	if _, err := core.CoreV1().PersistentVolumeClaims(scope.Namespace).Get(ctx, "manual382", metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}
	return action, core, dynamicClient, scope.TenantID, scope.ServiceID
}

// capability_id: rainbond.vm-storage.existing-snapshot-capability
// capability_id: rainbond.vm-storage.managed-snapshot-policy
func TestExistingSnapshotCapabilitySupportsLonghornAndExcludesInstallMedia(t *testing.T) {
	action, _, _, tenantID, serviceID := longhornSnapshotCapabilityFixture(t)
	result, err := action.GetExistingVMSnapshotCapability(context.Background(), tenantID, serviceID)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Supported || result.Status != "supported" || !result.Complete || len(result.Volumes) != 1 || len(result.ExcludedVolumes) != 3 {
		t.Fatalf("unexpected capability: %+v", result)
	}
	if result.Volumes[0].Key != "manual382" || result.Volumes[0].Driver != vmstorage.LonghornSnapshotDriver || result.Volumes[0].StorageClass != "longhorn-vm-snapshot" {
		t.Fatalf("Longhorn disk identity lost: %+v", result.Volumes)
	}
	encoded, err := json.Marshal(result)
	if err != nil || !json.Valid(encoded) {
		t.Fatal("capability response is not valid JSON", err)
	}
}

// capability_id: rainbond.vm-storage.snapshot-options
func TestVolumeOptionsExposeManagedLonghornCapability(t *testing.T) {
	_, core, dynamicClient, _, _ := longhornSnapshotCapabilityFixture(t)
	options := []*apimodel.VolumeTypeStruct{{VolumeType: "longhorn-vm-snapshot", NameShow: "Longhorn"}}
	annotateSnapshotCapabilities(context.Background(), options, vmstorage.NewInspector(core, dynamicClient))
	if options[0].SnapshotCapability == nil || !options[0].SnapshotCapability.Supported || options[0].ManagedSnapshotCapability == nil || !options[0].ManagedSnapshotCapability.Supported {
		t.Fatalf("Longhorn option is not snapshot capable: %+v", options[0])
	}
}

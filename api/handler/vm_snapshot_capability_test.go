package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"

	dbdao "github.com/goodrain/rainbond/db/dao"
	mysqldao "github.com/goodrain/rainbond/db/mysql/dao"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	csi "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	csifake "kubevirt.io/client-go/externalsnapshotter/fake"

	"github.com/golang/mock/gomock"
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/pkg/vmstorage"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
)

var (
	storageCapClassGVR          = schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshotclasses"}
	storageCapVolumeSnapshotGVR = schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"}
	storageCapVMSnapshotGVR     = schema.GroupVersionResource{Group: "snapshot.kubevirt.io", Version: "v1beta1", Resource: "virtualmachinesnapshots"}
	storageCapKubeVirtGVR       = schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "kubevirts"}
)

func storageCapReadyPod(name string, containers ...corev1.Container) *corev1.Pod {
	p := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "infra"}, Spec: corev1.PodSpec{Containers: containers}, Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}}
	for _, c := range containers {
		p.Status.ContainerStatuses = append(p.Status.ContainerStatuses, corev1.ContainerStatus{Name: c.Name, Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}})
	}
	return p
}

func storageCapFixture() (*kubefake.Clientset, *fake.FakeDynamicClient) {
	core := kubefake.NewSimpleClientset(&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "fast"}, Provisioner: "example.csi.io"}, &storagev1.CSIDriver{ObjectMeta: metav1.ObjectMeta{Name: "example.csi.io"}}, storageCapReadyPod("controller", corev1.Container{Name: "snapshot-controller", Image: "registry/snapshot-controller:v1"}), storageCapReadyPod("provider", corev1.Container{Name: "plugin", Args: []string{"--drivername=$(DRIVER_NAME)"}, Env: []corev1.EnvVar{{Name: "DRIVER_NAME", Value: "example.csi.io"}}}, corev1.Container{Name: "snapshotter", Image: "registry/csi-snapshotter:v1"}))
	kv := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "kubevirt.io/v1", "kind": "KubeVirt", "metadata": map[string]interface{}{"name": "virt", "namespace": "infra"}, "status": map[string]interface{}{"phase": "Deployed", "conditions": []interface{}{map[string]interface{}{"type": "Available", "status": "True"}, map[string]interface{}{"type": "Degraded", "status": "False"}}}}}
	class := storageCapSnapshotClass("class-one", "example.csi.io", false)
	dynamic := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{storageCapClassGVR: "VolumeSnapshotClassList", storageCapVolumeSnapshotGVR: "VolumeSnapshotList", storageCapVMSnapshotGVR: "VirtualMachineSnapshotList", storageCapKubeVirtGVR: "KubeVirtList"}, kv, class)
	return core, dynamic
}

func storageCapSnapshotClass(name, driver string, def bool) *unstructured.Unstructured {
	a := map[string]interface{}{}
	if def {
		a["snapshot.storage.kubernetes.io/is-default-class"] = "true"
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "snapshot.storage.k8s.io/v1", "kind": "VolumeSnapshotClass", "metadata": map[string]interface{}{"name": name, "annotations": a}, "driver": driver, "deletionPolicy": "Delete"}}
}

func storageCapBoundVMFixture(t *testing.T) (*kubefake.Clientset, *fake.FakeDynamicClient, *v1.VirtualMachine) {
	t.Helper()
	c, d := storageCapFixture()
	sc := "fast"
	uid := types.UID("pvc-root")
	_, err := c.CoreV1().PersistentVolumeClaims("demo-ns").Create(context.Background(), &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "root", Namespace: "demo-ns", UID: uid}, Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &sc, VolumeName: "pv-root"}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.CoreV1().PersistentVolumes().Create(context.Background(), &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "pv-root"}, Spec: corev1.PersistentVolumeSpec{ClaimRef: &corev1.ObjectReference{Name: "root", Namespace: "demo-ns", UID: uid}, PersistentVolumeSource: corev1.PersistentVolumeSource{CSI: &corev1.CSIPersistentVolumeSource{Driver: "example.csi.io", VolumeHandle: "handle-root"}}}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	vm := &v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "demo-vm", Namespace: "demo-ns", UID: "vm-uid"}, Spec: v1.VirtualMachineSpec{Template: &v1.VirtualMachineInstanceTemplateSpec{Spec: v1.VirtualMachineInstanceSpec{Volumes: []v1.Volume{{Name: "root", VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: "root"}}}}}}}}}
	return c, d, vm
}

// capability_id: rainbond.vm-storage.snapshot-options
func TestVolumeOptionsSnapshotCapabilityPreservesUnknown(t *testing.T) {
	options := []*apimodel.VolumeTypeStruct{{VolumeType: "fast", NameShow: "chosen"}, {VolumeType: "missing"}}
	core, dyn := storageCapFixture()
	annotateSnapshotCapabilities(context.Background(), options, vmstorage.NewInspector(core, dyn))
	if !options[0].SnapshotCapability.Supported || options[1].SnapshotCapability.Supported || options[1].SnapshotCapability.Status != "unknown" || options[0].ManagedSnapshotCapability.Supported || options[0].ManagedSnapshotCapability.ReasonCode != "managed_snapshot_driver_unsupported" || options[1].ManagedSnapshotCapability.Status != "unknown" {
		t.Fatalf("%+v", options)
	}
	b, err := json.Marshal(options)
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]interface{}
	if json.Unmarshal(b, &rows) != nil || rows[0]["snapshot_capability"] == nil || rows[0]["managed_snapshot_capability"] == nil || rows[0]["name_show"] != "chosen" {
		t.Fatalf("JSON contract lost: %s", b)
	}
	annotateSnapshotCapabilities(context.Background(), options, nil)
	if options[0].SnapshotCapability.Supported || options[0].SnapshotCapability.Status != "unknown" || options[0].ManagedSnapshotCapability.Supported || options[0].ManagedSnapshotCapability.Status != "unknown" {
		t.Fatal("nil evidence accepted")
	}
}

// capability_id: rainbond.vm-storage.snapshot-create-preflight
func TestCreateVMSnapshotRejectsPartialAndUnreadableStorage(t *testing.T) {
	for _, unreadable := range []bool{false, true} {
		t.Run(map[bool]string{false: "unsupported data", true: "read failure"}[unreadable], func(t *testing.T) {
			f, list := managedSnapshotPreflightFixture(t)
			vm := &list.Items[0]
			if unreadable {
				f.core.PrependReactor("get", "storageclasses", func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, errors.New("read denied") })
			} else {
				vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, v1.Volume{Name: "data", VolumeSource: v1.VolumeSource{EmptyDisk: &v1.EmptyDiskSource{}}})
				setExistingSnapshotVM(t, f, vm)
				capability, err := f.action.GetExistingVMSnapshotCapability(context.Background(), f.scope.TenantID, f.scope.ServiceID)
				if err != nil || capability.Supported || len(capability.Volumes) != 2 || capability.Complete {
					t.Fatalf("unregistered data disappeared from authoritative preflight: %+v %v", capability, err)
				}
			}
			before, err := f.store.Read(f.scope, "")
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.action.CreateVMSnapshot(f.scope.ServiceID, &VMSnapshotRequest{Name: "snap", RequestID: "preflight-op", ManifestID: "preflight-manifest"})
			if unreadable && !errors.Is(err, vmrestore.ErrReferenceUnavailable) || !unreadable && !errors.Is(err, vmrestore.ErrSnapshotConfig) && !errors.Is(err, vmrestore.ErrSnapshotIncomplete) {
				t.Fatalf("managed preflight did not reject before acceptance: %v", err)
			}
			after, readErr := f.store.Read(f.scope, "")
			if readErr != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("failed preflight acquired an operation", readErr)
			}
		})
	}
}

// capability_id: rainbond.vm-storage.snapshot-create-preflight
func TestCreateVMSnapshotUnknownVMIdentityRejectsWithoutCreate(t *testing.T) {
	for _, count := range []int{0, 2} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			f, list := managedSnapshotPreflightFixture(t)
			list.Items = make([]v1.VirtualMachine, count)
			before, err := f.store.Read(f.scope, "")
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.action.CreateVMSnapshot(f.scope.ServiceID, &VMSnapshotRequest{Name: "snap", RequestID: "identity-op", ManifestID: "identity-manifest"})
			if !errors.Is(err, vmrestore.ErrConflict) {
				t.Fatalf("ambiguous source identity was accepted: %v", err)
			}
			after, readErr := f.store.Read(f.scope, "")
			if readErr != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("unknown identity acquired an operation", readErr)
			}
		})
	}
}

type managedSnapshotPreflightManager struct{ bindingConsumerDB }

func (m managedSnapshotPreflightManager) TenantDao() dbdao.TenantDao {
	return &mysqldao.TenantDaoImpl{DB: m.database}
}

// Exercise the actual scoped ServiceAction -> Engine -> KubeSnapshotBackend
// preflight. No native Create expectation exists: it must remain unreachable.
func managedSnapshotPreflightFixture(t *testing.T) (bindingConsumerFixture, *v1.VirtualMachineList) {
	t.Helper()
	f, vm := existingSnapshotFixture(t)
	f.volume.VolumeName = "disk"
	f.volume.VolumePath = "/disk"
	if err := f.database.Save(f.volume).Error; err != nil {
		t.Fatal(err)
	}
	current, err := f.store.Read(f.scope, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		b := c.State.Bindings["42"]
		b.DiskKey = "disk"
		b.Role = "root"
		c.State.Bindings["42"] = b
		c.State.BindingGeneration++
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	vm.Spec.Template.Spec.Domain.CPU = &v1.CPU{Cores: 2}
	vm.Spec.Template.Spec.Domain.Resources.Requests = corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")}
	setExistingSnapshotVM(t, f, vm)
	list := &v1.VirtualMachineList{Items: []v1.VirtualMachine{*vm}}
	ctrl := gomock.NewController(t)
	client := kubecli.NewMockKubevirtClient(ctrl)
	vms := kubecli.NewMockVirtualMachineInterface(ctrl)
	client.EXPECT().DynamicClient().Return(f.dynamic).AnyTimes()
	client.EXPECT().CoreV1().Return(f.core.CoreV1()).AnyTimes()
	client.EXPECT().StorageV1().Return(f.core.StorageV1()).AnyTimes()
	snapshotClient := csifake.NewSimpleClientset(&csi.VolumeSnapshotClass{ObjectMeta: metav1.ObjectMeta{Name: "cephfs", Annotations: map[string]string{"snapshot.storage.kubernetes.io/is-default-class": "true"}}, Driver: "cephfs.csi.ceph.com", DeletionPolicy: csi.VolumeSnapshotContentDelete})
	client.EXPECT().KubernetesSnapshotClient().Return(snapshotClient).AnyTimes()
	client.EXPECT().VirtualMachine(f.scope.Namespace).Return(vms).AnyTimes()
	vms.EXPECT().List(gomock.Any(), metav1.ListOptions{LabelSelector: "service_id=" + f.scope.ServiceID}).DoAndReturn(func(context.Context, metav1.ListOptions) (*v1.VirtualMachineList, error) { return list, nil }).AnyTimes()
	f.action.kubevirtClient = client
	f.action.dbmanager = managedSnapshotPreflightManager{bindingConsumerDB{database: f.database}}
	return f, list
}

// capability_id: rainbond.vm-storage.existing-snapshot-capability
// capability_id: rainbond.vm-storage.managed-snapshot-policy
func TestExistingSnapshotCapabilitySeparatesNativeCSIAndManagedPolicy(t *testing.T) {
	ctx := context.Background()
	f, _ := managedSnapshotPreflightFixture(t)
	driver := "rbd.csi.ceph.com"
	sc, err := f.core.StorageV1().StorageClasses().Get(ctx, "fast", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sc.Provisioner = driver
	if _, err = f.core.StorageV1().StorageClasses().Update(ctx, sc, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.core.StorageV1().CSIDrivers().Create(ctx, &storagev1.CSIDriver{ObjectMeta: metav1.ObjectMeta{Name: driver}}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	pv, err := f.core.CoreV1().PersistentVolumes().Get(ctx, "pv-active-a", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pv.Spec.CSI.Driver = driver
	if _, err = f.core.CoreV1().PersistentVolumes().Update(ctx, pv, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	pod := storageCapReadyPod("rbd-csi", corev1.Container{Name: "driver", Args: []string{"--drivername=" + driver}}, corev1.Container{Name: "snapshotter", Image: "registry/csi-snapshotter:v1"})
	if _, err = f.core.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.dynamic.Resource(storageCapClassGVR).Create(ctx, storageCapSnapshotClass("rbd-class", driver, true), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	claim, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(ctx, "active-a", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	native := vmstorage.NewInspector(f.core, f.dynamic).PersistentVolumeClaim(ctx, claim)
	if !native.Supported || native.Driver != driver {
		t.Fatalf("generic CSI evidence changed: %+v", native)
	}
	before, err := f.store.Read(f.scope, "")
	if err != nil {
		t.Fatal(err)
	}
	capability, err := f.action.GetExistingVMSnapshotCapability(ctx, f.scope.TenantID, f.scope.ServiceID)
	if err != nil {
		t.Fatal(err)
	}
	if capability.Supported || capability.Status != "unsupported" || capability.ReasonCode != "managed_snapshot_driver_unsupported" || !capability.Complete || len(capability.Volumes) != 1 || capability.Volumes[0].Driver != driver || capability.Volumes[0].ReasonCode != "managed_snapshot_driver_unsupported" {
		t.Fatalf("native capability promoted into managed capability: %+v", capability)
	}
	_, createErr := f.action.CreateVMSnapshot(f.scope.ServiceID, &VMSnapshotRequest{Name: "policy-proof", RequestID: "policy-op", ManifestID: "policy-manifest"})
	if !errors.Is(createErr, vmrestore.ErrSnapshotStorage) {
		t.Fatalf("managed policy drifted from its existing backend restriction: %v", createErr)
	}
	after, err := f.store.Read(f.scope, "")
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("policy query/refusal changed business state", err)
	}
	if path := os.Getenv("VM_COMBINATION_POLICY_EVIDENCE_AFTER"); path != "" {
		raw, _ := json.MarshalIndent(map[string]interface{}{"generic_native_capability": native, "managed_capability": capability, "managed_error_is_ErrSnapshotStorage": true}, "", "  ")
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

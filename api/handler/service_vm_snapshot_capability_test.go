package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	ktesting "k8s.io/client-go/testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	v1 "kubevirt.io/api/core/v1"
)

func existingSnapshotFixture(t *testing.T) (bindingConsumerFixture, *v1.VirtualMachine) {
	t.Helper()
	f := newBindingConsumerFixture(t)
	ctx := context.Background()
	_, err := f.core.StorageV1().CSIDrivers().Create(ctx, &storagev1.CSIDriver{ObjectMeta: metav1.ObjectMeta{Name: "cephfs.csi.ceph.com"}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []*corev1.Pod{storageCapReadyPod("snapshot-controller", corev1.Container{Name: "controller", Image: "registry/snapshot-controller:v1"}), storageCapReadyPod("csi", corev1.Container{Name: "driver", Args: []string{"--drivername=cephfs.csi.ceph.com"}}, corev1.Container{Name: "snapshotter", Image: "registry/csi-snapshotter:v1"})} {
		if _, err = f.core.CoreV1().Pods(p.Namespace).Create(ctx, p, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	for _, entry := range []struct{ name, uid string }{{"active-a", "a-uid"}, {"manual42", "b-uid"}} {
		p, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(ctx, entry.name, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		p.Spec.VolumeName = "pv-" + entry.name
		_, _ = f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Update(ctx, p, metav1.UpdateOptions{})
		_, err = f.core.CoreV1().PersistentVolumes().Create(ctx, &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: p.Spec.VolumeName}, Spec: corev1.PersistentVolumeSpec{ClaimRef: &corev1.ObjectReference{Namespace: p.Namespace, Name: p.Name, UID: types.UID(entry.uid)}, PersistentVolumeSource: corev1.PersistentVolumeSource{CSI: &corev1.CSIPersistentVolumeSource{Driver: "cephfs.csi.ceph.com", VolumeHandle: entry.name}}}}, metav1.CreateOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}
	vm := &v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "vm", Namespace: f.scope.Namespace, UID: "vm-uid", ResourceVersion: "1", Labels: map[string]string{"service_id": f.scope.ServiceID, "tenant_id": f.scope.TenantID}}, Spec: v1.VirtualMachineSpec{Template: &v1.VirtualMachineInstanceTemplateSpec{Spec: v1.VirtualMachineInstanceSpec{Volumes: []v1.Volume{{Name: "manual42", VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: "active-a"}}}}}, Domain: v1.DomainSpec{Devices: v1.Devices{Disks: []v1.Disk{{Name: "manual42", DiskDevice: v1.DiskDevice{Disk: &v1.DiskTarget{Bus: v1.DiskBusSCSI}}}}}}}}}}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
	if err != nil {
		t.Fatal(err)
	}
	obj["apiVersion"] = "kubevirt.io/v1"
	obj["kind"] = "VirtualMachine"
	kv := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "kubevirt.io/v1", "kind": "KubeVirt", "metadata": map[string]interface{}{"name": "virt", "namespace": "infra"}, "status": map[string]interface{}{"phase": "Deployed", "conditions": []interface{}{map[string]interface{}{"type": "Available", "status": "True"}}}}}
	dc := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{dataVolumeGVR: "DataVolumeList", vmDiskVMGVR: "VirtualMachineList", storageCapClassGVR: "VolumeSnapshotClassList", storageCapVolumeSnapshotGVR: "VolumeSnapshotList", storageCapVMSnapshotGVR: "VirtualMachineSnapshotList", storageCapKubeVirtGVR: "KubeVirtList"}, &unstructured.Unstructured{Object: obj}, kv, storageCapSnapshotClass("class", "cephfs.csi.ceph.com", true))
	vmHotplugDynamicClient = func() dynamic.Interface { return dc }
	f.dynamic = dc
	f.resolver = vmrestore.NewResolver(f.store, f.core, dc)
	return f, vm
}
func setExistingSnapshotVM(t *testing.T, f bindingConsumerFixture, vm *v1.VirtualMachine) {
	t.Helper()
	// Use Kubernetes' JSON decoder, as the real dynamic client does. Direct
	// typed conversion can retain uint64 CPU fields unsupported by fake DeepCopy.
	wire := vm.DeepCopy()
	wire.APIVersion, wire.Kind = "kubevirt.io/v1", "VirtualMachine"
	raw, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	object := &unstructured.Unstructured{}
	if err := object.UnmarshalJSON(raw); err != nil {
		t.Fatal(err)
	}
	object.SetAPIVersion("kubevirt.io/v1")
	object.SetKind("VirtualMachine")
	if _, err := f.dynamic.Resource(vmDiskVMGVR).Namespace(f.scope.Namespace).Update(context.Background(), object, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.vm-storage.existing-snapshot-capability
func TestExistingSnapshotCapabilityMissingAttachedVolumeIsIncomplete(t *testing.T) {
	f, vm := existingSnapshotFixture(t)
	vm.Spec.Template.Spec.Volumes = nil
	vm.Spec.Template.Spec.Domain.Devices.Disks = nil
	setExistingSnapshotVM(t, f, vm)
	result, err := f.action.GetExistingVMSnapshotCapability(context.Background(), f.scope.TenantID, f.scope.ServiceID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Complete || result.Supported || result.Status != "unknown" || len(result.Volumes) != 1 || len(result.ExcludedVolumes) != 0 {
		t.Fatalf("missing attached participant disappeared or remained complete: %+v", result)
	}
	if result.Volumes[0].VolumeID != f.volume.ID || result.Volumes[0].ReasonCode != "participant_not_attached" {
		t.Fatal("expected participant and its reason were not retained", result.Volumes)
	}
}

// capability_id: rainbond.vm-storage.existing-snapshot-capability
func TestExistingSnapshotCapabilityUsesCurrentAAndReadOnlyData(t *testing.T) {
	f, _ := existingSnapshotFixture(t)
	// B exists with an unsupported class. It must not replace current mapped A.
	p, _ := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "manual42", metav1.GetOptions{})
	other := "unregistered"
	p.Spec.StorageClassName = &other
	_, _ = f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Update(context.Background(), p, metav1.UpdateOptions{})
	f.volume.IsReadOnly = true
	if err := f.database.Save(f.volume).Error; err != nil {
		t.Fatal(err)
	}
	result, err := f.action.GetExistingVMSnapshotCapability(context.Background(), f.scope.TenantID, f.scope.ServiceID)
	if err != nil || !result.Supported || !result.Complete || len(result.Volumes) != 1 || result.Volumes[0].VolumeID != 42 || result.Volumes[0].StorageClass != "fast" || result.Operation != "create_snapshot" {
		t.Fatalf("A/readonly data not assessed: %+v %v", result, err)
	}
}

// capability_id: rainbond.vm-storage.existing-snapshot-capability
func TestExistingSnapshotCapabilityMixedAndInstaller(t *testing.T) {
	t.Run("unsupported data stays in complete set", func(t *testing.T) {
		f, vm := existingSnapshotFixture(t)
		v := &dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 43}, ServiceID: f.scope.ServiceID, VolumeName: "data-2", VolumePath: "/disk-2", VolumeType: "not-csi", VolumeCapacity: 1}
		if err := f.database.Create(v).Error; err != nil {
			t.Fatal(err)
		}
		sc := "not-csi"
		_, _ = f.core.StorageV1().StorageClasses().Create(context.Background(), &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: sc}, Provisioner: "legacy.example"}, metav1.CreateOptions{})
		p := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "manual43", Namespace: f.scope.Namespace, UID: "data-uid", Labels: map[string]string{"service_id": f.scope.ServiceID, "tenant_id": f.scope.TenantID}}, Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &sc}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}
		_, _ = f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Create(context.Background(), p, metav1.CreateOptions{})
		vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, v1.Volume{Name: "manual43", VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: "manual43"}}}})
		vm.Spec.Template.Spec.Domain.Devices.Disks = append(vm.Spec.Template.Spec.Domain.Devices.Disks, v1.Disk{Name: "manual43", DiskDevice: v1.DiskDevice{Disk: &v1.DiskTarget{}}})
		setExistingSnapshotVM(t, f, vm)
		r, err := f.action.GetExistingVMSnapshotCapability(context.Background(), f.scope.TenantID, f.scope.ServiceID)
		if err != nil || r.Supported || r.Status != "unsupported" || len(r.Volumes) != 2 || !r.Complete {
			t.Fatalf("data omitted: %+v %v", r, err)
		}
	})
	t.Run("real readonly installer is explicitly excluded", func(t *testing.T) {
		f, vm := existingSnapshotFixture(t)
		vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, v1.Volume{Name: "vmimage", VolumeSource: v1.VolumeSource{ContainerDisk: &v1.ContainerDiskSource{Image: "installer"}}})
		vm.Spec.Template.Spec.Domain.Devices.Disks = append(vm.Spec.Template.Spec.Domain.Devices.Disks, v1.Disk{Name: "vmimage", DiskDevice: v1.DiskDevice{CDRom: &v1.CDRomTarget{}}})
		setExistingSnapshotVM(t, f, vm)
		r, err := f.action.GetExistingVMSnapshotCapability(context.Background(), f.scope.TenantID, f.scope.ServiceID)
		if err != nil || !r.Supported || len(r.Volumes) != 1 || len(r.ExcludedVolumes) != 1 || r.ExcludedVolumes[0].ReasonCode != "read_only_installation_media" {
			t.Fatalf("installer treated as data: %+v %v", r, err)
		}
	})
}

// capability_id: rainbond.vm-storage.existing-snapshot-capability
func TestExistingSnapshotCapabilityUnknownAndScopeBoundaries(t *testing.T) {
	cases := []struct {
		name, reason string
		change       func(bindingConsumerFixture, *v1.VirtualMachine)
	}{
		{"mapped A UID replacement", "binding_view_unavailable", func(f bindingConsumerFixture, v *v1.VirtualMachine) {
			p, _ := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "active-a", metav1.GetOptions{})
			p.UID = "replacement"
			_, _ = f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Update(context.Background(), p, metav1.UpdateOptions{})
		}},
		{"VM still mounts B", "participating_volume_unverified", func(f bindingConsumerFixture, v *v1.VirtualMachine) {
			v.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = "manual42"
			setExistingSnapshotVM(t, f, v)
		}},
		{"default is not guessed for bound PVC", "participating_volume_unverified", func(f bindingConsumerFixture, v *v1.VirtualMachine) {
			p, _ := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "active-a", metav1.GetOptions{})
			p.Spec.StorageClassName = nil
			_, _ = f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Update(context.Background(), p, metav1.UpdateOptions{})
			sc, _ := f.core.StorageV1().StorageClasses().Get(context.Background(), "fast", metav1.GetOptions{})
			sc.Annotations = map[string]string{"storageclass.kubernetes.io/is-default-class": "true"}
			_, _ = f.core.StorageV1().StorageClasses().Update(context.Background(), sc, metav1.UpdateOptions{})
		}},
		{"storage read failure", "participating_volume_unverified", func(f bindingConsumerFixture, v *v1.VirtualMachine) {
			f.core.PrependReactor("get", "storageclasses", func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, fmt.Errorf("denied") })
		}},
		{"missing VM", "vm_identity_unavailable", func(f bindingConsumerFixture, v *v1.VirtualMachine) {
			_ = f.dynamic.Resource(vmDiskVMGVR).Namespace(f.scope.Namespace).Delete(context.Background(), v.Name, metav1.DeleteOptions{})
		}},
		{"VM has no UID", "vm_identity_unavailable", func(f bindingConsumerFixture, v *v1.VirtualMachine) {
			v.UID = ""
			setExistingSnapshotVM(t, f, v)
		}},
		{"duplicate actual volume key", "volume_set_invalid", func(f bindingConsumerFixture, v *v1.VirtualMachine) {
			v.Spec.Template.Spec.Volumes = append(v.Spec.Template.Spec.Volumes, v.Spec.Template.Spec.Volumes[0])
			setExistingSnapshotVM(t, f, v)
		}},
		{"fresh PVC read fails", "participating_volume_unverified", func(f bindingConsumerFixture, v *v1.VirtualMachine) {
			reads := 0
			f.core.PrependReactor("get", "persistentvolumeclaims", func(a ktesting.Action) (bool, runtime.Object, error) {
				if a.(ktesting.GetAction).GetName() == "active-a" {
					reads++
					if reads > 1 {
						return true, nil, fmt.Errorf("denied")
					}
				}
				return false, nil, nil
			})
		}},
		{"multiple VM identities", "vm_identity_unavailable", func(f bindingConsumerFixture, v *v1.VirtualMachine) {
			copy := v.DeepCopy()
			copy.Name = "duplicate"
			copy.UID = "duplicate-uid"
			obj, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(copy)
			obj["apiVersion"] = "kubevirt.io/v1"
			obj["kind"] = "VirtualMachine"
			_, _ = f.dynamic.Resource(vmDiskVMGVR).Namespace(f.scope.Namespace).Create(context.Background(), &unstructured.Unstructured{Object: obj}, metav1.CreateOptions{})
		}},
		{"foreign operation admission", "component_operation_in_progress", func(f bindingConsumerFixture, v *v1.VirtualMachine) {
			current, _ := f.store.Read(f.scope, "")
			_, err := f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
				if err := c.State.AdmitMutation("foreign-op", "expand"); err != nil {
					return vmrestore.Mutation{}, err
				}
				return vmrestore.Mutation{State: c.State}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}},
		{"generation change during read", "binding_view_changed", func(f bindingConsumerFixture, v *v1.VirtualMachine) {
			changed := false
			f.core.PrependReactor("get", "storageclasses", func(ktesting.Action) (bool, runtime.Object, error) {
				if !changed {
					changed = true
					c, _ := f.store.Read(f.scope, "")
					_, err := f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: c.State.ControlRevision, BindingGeneration: c.State.BindingGeneration}, func(v vmrestore.Current) (vmrestore.Mutation, error) { return vmrestore.Mutation{State: v.State}, nil })
					if err != nil {
						t.Fatal(err)
					}
				}
				return false, nil, nil
			})
		}},
		{"VM change during read", "vm_changed_during_check", func(f bindingConsumerFixture, v *v1.VirtualMachine) {
			changed := false
			f.core.PrependReactor("get", "storageclasses", func(ktesting.Action) (bool, runtime.Object, error) {
				if !changed {
					changed = true
					v.ResourceVersion = "2"
					setExistingSnapshotVM(t, f, v)
				}
				return false, nil, nil
			})
		}},
		{"extra unregistered participant", "participating_volume_unverified", func(f bindingConsumerFixture, v *v1.VirtualMachine) {
			v.Spec.Template.Spec.Volumes = append(v.Spec.Template.Spec.Volumes, v1.Volume{Name: "unregistered-data", VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: "other"}}}})
			setExistingSnapshotVM(t, f, v)
		}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			f, v := existingSnapshotFixture(t)
			tt.change(f, v)
			r, err := f.action.GetExistingVMSnapshotCapability(context.Background(), f.scope.TenantID, f.scope.ServiceID)
			if err != nil || r.Supported || r.Status != "unknown" || r.ReasonCode != tt.reason {
				t.Fatalf("got %+v %v; want unknown/%s", r, err, tt.reason)
			}
		})
	}
	t.Run("foreign tenant and read-only ledger", func(t *testing.T) {
		f, _ := existingSnapshotFixture(t)
		before, err := f.store.Read(f.scope, "")
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.action.GetExistingVMSnapshotCapability(context.Background(), "other-tenant", f.scope.ServiceID)
		if err == nil {
			t.Fatal("cross tenant allowed")
		}
		r, err := f.action.GetExistingVMSnapshotCapability(context.Background(), f.scope.TenantID, f.scope.ServiceID)
		if err != nil || !r.Supported {
			t.Fatalf("%+v %v", r, err)
		}
		after, err := f.store.Read(f.scope, "")
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatal("GET changed authoritative state")
		}
	})
}

// capability_id: rainbond.vm-storage.existing-snapshot-capability
func TestExistingSnapshotCapabilityRetainedDetachedDiskNotCurrent(t *testing.T) {
	f, vm := existingSnapshotFixture(t)
	ctx := context.Background()
	root := &dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 41}, ServiceID: f.scope.ServiceID, VolumeName: "disk", VolumePath: "/disk", VolumeType: "fast", VolumeCapacity: 1}
	if err := f.database.Create(root).Error; err != nil {
		t.Fatal(err)
	}
	p, _ := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(ctx, "active-a", metav1.GetOptions{})
	p = p.DeepCopy()
	p.Name = "manual41"
	p.UID = "root-uid"
	p.Spec.VolumeName = "pv-root"
	_, _ = f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Create(ctx, p, metav1.CreateOptions{})
	_, _ = f.core.CoreV1().PersistentVolumes().Create(ctx, &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "pv-root"}, Spec: corev1.PersistentVolumeSpec{ClaimRef: &corev1.ObjectReference{Namespace: f.scope.Namespace, Name: p.Name, UID: p.UID}, PersistentVolumeSource: corev1.PersistentVolumeSource{CSI: &corev1.CSIPersistentVolumeSource{Driver: "cephfs.csi.ceph.com", VolumeHandle: "root"}}}}, metav1.CreateOptions{})
	current, _ := f.store.Read(f.scope, "")
	_, err := f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		b := c.State.Bindings["42"]
		b.Attached = false
		b.PreviousPVC = &vmrestore.ObjectRef{Kind: "PersistentVolumeClaim", Namespace: f.scope.Namespace, Name: "manual42", UID: "b-uid"}
		c.State.Bindings["42"] = b
		c.State.BindingGeneration++
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	vm.Spec.Template.Spec.Volumes = []v1.Volume{{Name: "manual41", VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: "manual41"}}}}}
	vm.Spec.Template.Spec.Domain.Devices.Disks = []v1.Disk{{Name: "manual41", DiskDevice: v1.DiskDevice{Disk: &v1.DiskTarget{}}}}
	setExistingSnapshotVM(t, f, vm)
	result, err := f.action.GetExistingVMSnapshotCapability(ctx, f.scope.TenantID, f.scope.ServiceID)
	if err != nil || !result.Supported || len(result.Volumes) != 1 || result.Volumes[0].VolumeID != 41 || len(result.ExcludedVolumes) != 1 || result.ExcludedVolumes[0].ReasonCode != "detached_retained_volume" {
		t.Fatalf("retained B counted: %+v %v", result, err)
	}
}

// capability_id: rainbond.vm-storage.existing-snapshot-capability
func TestExistingSnapshotCapabilityInstallationMediaRequiresCurrentIdentity(t *testing.T) {
	f, vm := existingSnapshotFixture(t)
	ctx := context.Background()
	media := &dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 43}, ServiceID: f.scope.ServiceID, VolumeName: "installer", VolumePath: "/cdrom-1", VolumeType: "fast", VolumeCapacity: 1}
	if err := f.database.Create(media).Error; err != nil {
		t.Fatal(err)
	}
	p := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "manual43", Namespace: f.scope.Namespace, UID: "media-uid", Labels: map[string]string{"service_id": f.scope.ServiceID, "tenant_id": f.scope.TenantID}}, Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: stringPtrForBinding("fast")}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}
	if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Create(ctx, p, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, v1.Volume{Name: "manual43", VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: "manual42"}}}})
	vm.Spec.Template.Spec.Domain.Devices.Disks = append(vm.Spec.Template.Spec.Domain.Devices.Disks, v1.Disk{Name: "manual43", DiskDevice: v1.DiskDevice{CDRom: &v1.CDRomTarget{}}})
	setExistingSnapshotVM(t, f, vm)
	r, err := f.action.GetExistingVMSnapshotCapability(ctx, f.scope.TenantID, f.scope.ServiceID)
	if err != nil || r.Status != "unknown" || r.Complete || len(r.Volumes) != 2 || r.Volumes[1].ReasonCode != "binding_mount_mismatch" || len(r.ExcludedVolumes) != 0 {
		t.Fatalf("mismatched media was excluded: %+v %v", r, err)
	}
	vm.Spec.Template.Spec.Volumes[1].PersistentVolumeClaim.ClaimName = "manual43"
	setExistingSnapshotVM(t, f, vm)
	f.core.ClearActions()
	f.dynamic.ClearActions()
	r, err = f.action.GetExistingVMSnapshotCapability(ctx, f.scope.TenantID, f.scope.ServiceID)
	if err != nil || !r.Supported || !r.Complete || len(r.Volumes) != 1 || len(r.ExcludedVolumes) != 1 || r.ExcludedVolumes[0].ReasonCode != "read_only_installation_media" {
		t.Fatalf("verified media was not excluded: %+v %v", r, err)
	}
	for _, a := range append(f.core.Actions(), f.dynamic.Actions()...) {
		if a.GetVerb() != "get" && a.GetVerb() != "list" {
			t.Fatalf("GET wrote Kubernetes: %s %s", a.GetVerb(), a.GetResource().Resource)
		}
	}
}

// capability_id: rainbond.vm-storage.existing-snapshot-capability
func TestExistingSnapshotCapabilityPreconditions(t *testing.T) {
	for _, name := range []string{"missing scope", "missing component", "non VM", "database failure", "missing Kubernetes client", "nil handler"} {
		t.Run(name, func(t *testing.T) {
			f, _ := existingSnapshotFixture(t)
			tenant, service := f.scope.TenantID, f.scope.ServiceID
			wantError := true
			switch name {
			case "missing scope":
				tenant = ""
			case "missing component":
				service = "absent"
			case "non VM":
				f.service.ExtendMethod = "stateless"
				if err := f.database.Save(f.service).Error; err != nil {
					t.Fatal(err)
				}
			case "database failure":
				f.database.Close()
			case "missing Kubernetes client":
				f.action.kubeClient = nil
				wantError = false
			case "nil handler":
				f.action = nil
				wantError = false
			}
			r, err := f.action.GetExistingVMSnapshotCapability(context.Background(), tenant, service)
			if wantError {
				if err == nil {
					t.Fatalf("unsafe scope accepted: %+v", r)
				}
			} else if err != nil || r.Status != "unknown" || r.Complete || r.Supported {
				t.Fatalf("unavailable client reported ready: %+v %v", r, err)
			}
		})
	}
}

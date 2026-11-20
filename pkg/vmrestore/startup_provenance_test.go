package vmrestore

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	kv "kubevirt.io/api/core/v1"
)

// capability_id: rainbond.vm-restore.private-startup-provenance
func TestStartupPublicationRejectsUnprovenNoCloudSecretReferences(t *testing.T) {
	for _, stage := range []string{"prepare", "seal"} {
		for _, field := range []string{"user", "network"} {
			t.Run(stage+"/"+field, func(t *testing.T) {
				store, database, scope := fixtureStore(t)
				vm := &kv.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "vm", Namespace: scope.Namespace}, Spec: kv.VirtualMachineSpec{Template: &kv.VirtualMachineInstanceTemplateSpec{}}}
				vm.Spec.Template.Spec.Volumes = []kv.Volume{{Name: "cloud", VolumeSource: kv.VolumeSource{CloudInitNoCloud: &kv.CloudInitNoCloudSource{UserData: "inline"}}}}
				vm.Spec.Template.Spec.Domain.Devices.Disks = []kv.Disk{{Name: "cloud"}}
				hints := VMStartupHints{NoCloudVolumeName: "cloud", NoCloudDiskName: "cloud"}
				var publication *VMStartupPublication
				if stage == "seal" {
					var err error
					publication, err = PrepareNewVMStartup(scope, vm, hints)
					if err != nil {
						t.Fatal(err)
					}
					vm.UID = "created-vm-uid"
				}
				cloud := vm.Spec.Template.Spec.Volumes[0].CloudInitNoCloud
				ref := &corev1.LocalObjectReference{Name: "unproven-initialization"}
				if field == "user" {
					cloud.UserDataSecretRef = ref
				} else {
					cloud.NetworkDataSecretRef = ref
				}
				before := vm.DeepCopy()
				core := kubefake.NewSimpleClientset()
				if stage == "prepare" {
					p, err := PrepareNewVMStartup(scope, vm, hints)
					if !errors.Is(err, ErrRestoreCapability) || p != nil {
						t.Fatal("unproven source received publication identity", err)
					}
				} else if err := store.SealNewVMStartup(context.Background(), core, vm, *publication); !errors.Is(err, ErrConflict) {
					t.Fatal("unproven source received private authority", err)
				}
				if !reflect.DeepEqual(before, vm) || len(core.Actions()) != 0 {
					t.Fatal("rejection mutated the VM or fetched initialization data")
				}
				state, err := store.Read(scope, "")
				if err != nil || state.State.StartupOperationID != "" || state.State.ControlRevision != 0 {
					t.Fatal("rejection changed private authority", err)
				}
				var count int
				if err := database.Model(&dbmodel.ComponentK8sAttributes{}).Count(&count).Error; err != nil || count != 0 {
					t.Fatal("rejection persisted private rows", count, err)
				}
				if stage == "seal" {
					cloud.UserDataSecretRef, cloud.NetworkDataSecretRef = nil, nil
					if err := store.SealNewVMStartup(context.Background(), core, vm, *publication); err != nil {
						t.Fatal("inline publication could not retry", err)
					}
					if _, err := store.ReadVMStartup(context.Background(), core, scope, vm); err != nil || len(core.Actions()) != 0 {
						t.Fatal("inline retry read arbitrary initialization data", err)
					}
				}
			})
		}
	}
}

// capability_id: rainbond.vm-restore.private-startup-provenance
func TestStartupSourceRequiresPrivateExactUIDPublicationAndExcludesGuestData(t *testing.T) {
	store, _, scope := fixtureStore(t)
	vm := &kv.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "vm", Namespace: scope.Namespace}, Spec: kv.VirtualMachineSpec{Template: &kv.VirtualMachineInstanceTemplateSpec{Spec: kv.VirtualMachineInstanceSpec{Volumes: []kv.Volume{{Name: "cloud", VolumeSource: kv.VolumeSource{CloudInitNoCloud: &kv.CloudInitNoCloudSource{UserData: "B_BOOTSTRAP_SENTINEL"}}}, {Name: "env", VolumeSource: kv.VolumeSource{ConfigMap: &kv.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "generated-env"}}}}}}}}}
	vm.Spec.Template.Spec.Domain.Devices.Disks = []kv.Disk{{Name: "cloud"}, {Name: "env"}}
	hints := VMStartupHints{NoCloudVolumeName: "cloud", NoCloudDiskName: "cloud", ConfigMaps: []VMStartupConfigMap{{VolumeName: "env", DiskName: "env", Name: "generated-env"}}}
	publication, err := PrepareNewVMStartup(scope, vm, hints)
	if err != nil {
		t.Fatal(err)
	}
	vm.UID = "created-vm-uid"
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "generated-env", Namespace: scope.Namespace, UID: "generated-env-uid"}, Data: map[string]string{"env": "B_ENV_SENTINEL"}}
	if !publication.MarkConfigMap(cm) {
		t.Fatal("renderer-owned media was not identified")
	}
	core := kubefake.NewSimpleClientset(cm)
	if _, err := store.ReadVMStartup(context.Background(), core, scope, vm); !errors.Is(err, ErrRestoreCapability) {
		t.Fatal("a copied annotation was accepted without private source receipt", err)
	}
	if err := store.SealNewVMStartup(context.Background(), core, vm, *publication); err != nil {
		t.Fatal(err)
	}
	policy, err := store.ReadVMStartup(context.Background(), core, scope, vm)
	if err != nil {
		t.Fatal(err)
	}
	if policy.FirmwareUUID == "" || policy.DataSourceKind != "CloudInitNoCloud" || len(policy.GeneratedMedia) != 1 || policy.GeneratedMedia[0].Source.UID != "generated-env-uid" {
		t.Fatal("exact producer identities missing")
	}
	c, err := store.Read(scope, publication.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(c.Receipt)
	if strings.Contains(string(raw), "B_ENV_SENTINEL") || strings.Contains(string(raw), "B_BOOTSTRAP_SENTINEL") || strings.Contains(string(raw), "userData") {
		t.Fatal("private proof persisted guest contents")
	}
	replacement := vm.DeepCopy()
	replacement.UID = "replacement-vm"
	if _, err := store.ReadVMStartup(context.Background(), core, scope, replacement); !errors.Is(err, ErrConflict) {
		t.Fatal("replacement VM used original source authority", err)
	}
	actual, _ := core.CoreV1().ConfigMaps(scope.Namespace).Get(context.Background(), cm.Name, metav1.GetOptions{})
	actual.UID = "replacement-media"
	_, _ = core.CoreV1().ConfigMaps(scope.Namespace).Update(context.Background(), actual, metav1.UpdateOptions{})
	if _, err := store.ReadVMStartup(context.Background(), core, scope, vm); !errors.Is(err, ErrConflict) {
		t.Fatal("replacement media used original source authority", err)
	}
}

// capability_id: rainbond.vm-restore.private-startup-provenance
func TestKnownAbsentDatasourceDoesNotInventCloudInit(t *testing.T) {
	store, _, scope := fixtureStore(t)
	vm := &kv.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "plain-vm", Namespace: scope.Namespace}, Spec: kv.VirtualMachineSpec{Template: &kv.VirtualMachineInstanceTemplateSpec{}}}
	p, err := PrepareNewVMStartup(scope, vm, VMStartupHints{})
	if err != nil {
		t.Fatal(err)
	}
	vm.UID = "plain-vm-uid"
	core := kubefake.NewSimpleClientset()
	if err := store.SealNewVMStartup(context.Background(), core, vm, *p); err != nil {
		t.Fatal(err)
	}
	policy, err := store.ReadVMStartup(context.Background(), core, scope, vm)
	if err != nil || policy.DataSourceKind != "None" || policy.NoCloudVolumeName != "" {
		t.Fatal("known absence became invented datasource", policy, err)
	}
}

// capability_id: rainbond.vm-restore.private-startup-provenance
func TestStartupAuthorityCannotSurviveMissingPrivateReceipt(t *testing.T) {
	store, database, scope := fixtureStore(t)
	vm := &kv.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "vm", Namespace: scope.Namespace}, Spec: kv.VirtualMachineSpec{Template: &kv.VirtualMachineInstanceTemplateSpec{}}}
	p, err := PrepareNewVMStartup(scope, vm, VMStartupHints{})
	if err != nil {
		t.Fatal(err)
	}
	vm.UID = "vm-uid"
	core := kubefake.NewSimpleClientset()
	if err := store.SealNewVMStartup(context.Background(), core, vm, *p); err != nil {
		t.Fatal(err)
	}
	key, _ := ReceiptKey(p.OperationID)
	if err := database.Where("component_id = ? AND name = ?", scope.ServiceID, key).Delete(&dbmodel.ComponentK8sAttributes{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.Read(scope, ""); !errors.Is(err, ErrCorrupt) {
		t.Fatal("startup receipt loss silently became unclaimed authority", err)
	}
}

// capability_id: rainbond.vm-restore.private-startup-provenance
func TestStartupProofCannotAuthorizeChangedInitializationDevices(t *testing.T) {
	for _, change := range []string{"env-reference", "extra-configdrive", "missing-disk", "duplicate-cloud"} {
		t.Run(change, func(t *testing.T) {
			store, _, scope := fixtureStore(t)
			vm := &kv.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "vm", Namespace: scope.Namespace}, Spec: kv.VirtualMachineSpec{Template: &kv.VirtualMachineInstanceTemplateSpec{Spec: kv.VirtualMachineInstanceSpec{Domain: kv.DomainSpec{Devices: kv.Devices{Disks: []kv.Disk{{Name: "cloud"}, {Name: "env"}}}}, Volumes: []kv.Volume{{Name: "cloud", VolumeSource: kv.VolumeSource{CloudInitNoCloud: &kv.CloudInitNoCloudSource{}}}, {Name: "env", VolumeSource: kv.VolumeSource{ConfigMap: &kv.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "env-source"}}}}}}}}}
			p, err := PrepareNewVMStartup(scope, vm, VMStartupHints{NoCloudVolumeName: "cloud", NoCloudDiskName: "cloud", ConfigMaps: []VMStartupConfigMap{{VolumeName: "env", DiskName: "env", Name: "env-source"}}})
			if err != nil {
				t.Fatal(err)
			}
			vm.UID = "vm-uid"
			cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "env-source", Namespace: scope.Namespace, UID: "env-uid"}}
			p.MarkConfigMap(cm)
			core := kubefake.NewSimpleClientset(cm)
			if err := store.SealNewVMStartup(context.Background(), core, vm, *p); err != nil {
				t.Fatal(err)
			}
			switch change {
			case "env-reference":
				vm.Spec.Template.Spec.Volumes[1].ConfigMap.Name = "user-advanced"
			case "extra-configdrive":
				vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, kv.Volume{Name: "other-init", VolumeSource: kv.VolumeSource{CloudInitConfigDrive: &kv.CloudInitConfigDriveSource{}}})
			case "missing-disk":
				vm.Spec.Template.Spec.Domain.Devices.Disks = vm.Spec.Template.Spec.Domain.Devices.Disks[:1]
			case "duplicate-cloud":
				vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, vm.Spec.Template.Spec.Volumes[0])
			}
			if _, err := store.ReadVMStartup(context.Background(), core, scope, vm); !errors.Is(err, ErrConflict) {
				t.Fatal("modified initialization acquired old source authority", change, err)
			}
		})
	}
}

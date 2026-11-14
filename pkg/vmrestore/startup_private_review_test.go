package vmrestore

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	kv "kubevirt.io/api/core/v1"
)

func independentStartupVM(scope Scope, cloud bool) *kv.VirtualMachine {
	vm := &kv.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "review-vm", Namespace: scope.Namespace}, Spec: kv.VirtualMachineSpec{Template: &kv.VirtualMachineInstanceTemplateSpec{}}}
	if cloud {
		vm.Spec.Template.Spec.Volumes = []kv.Volume{{Name: "cloud", VolumeSource: kv.VolumeSource{CloudInitNoCloud: &kv.CloudInitNoCloudSource{}}}}
		vm.Spec.Template.Spec.Domain.Devices.Disks = []kv.Disk{{Name: "cloud"}}
	}
	return vm
}
func TestIndependentV121StartupReceiptSQLFailureRollsBackAuthority(t *testing.T) {
	store, database, scope := fixtureStore(t)
	vm := independentStartupVM(scope, false)
	publication, err := PrepareNewVMStartup(scope, vm, VMStartupHints{})
	if err != nil {
		t.Fatal(err)
	}
	vm.UID = "review-created-uid"
	key, _ := ReceiptKey(publication.OperationID)
	if err := database.Exec("CREATE TRIGGER review_abort_startup BEFORE INSERT ON component_k8s_attributes WHEN NEW.name='" + key + "' BEGIN SELECT RAISE(ABORT,'PUBLIC_STARTUP_ABORT'); END").Error; err != nil {
		t.Fatal(err)
	}
	core := kubefake.NewSimpleClientset()
	err = store.SealNewVMStartup(context.Background(), core, vm, *publication)
	if err == nil {
		t.Fatal("real private publication abort did not fire")
	}
	if err := database.Exec("DROP TRIGGER review_abort_startup").Error; err != nil {
		t.Fatal(err)
	}
	state, err := store.Read(scope, "")
	if err != nil || state.State.StartupOperationID != "" || state.State.ControlRevision != 0 {
		t.Fatal("failed private publication retained authority", err)
	}
	var count int
	database.Model(&dbmodel.ComponentK8sAttributes{}).Count(&count)
	if count != 0 {
		t.Fatal("partial startup rows committed", count)
	}
	if err := store.SealNewVMStartup(context.Background(), core, vm, *publication); err != nil {
		t.Fatal("exact publication could not retry after rollback", err)
	}
	if _, err := store.ReadVMStartup(context.Background(), core, scope, vm); err != nil {
		t.Fatal(err)
	}
}
func TestIndependentV121PrivateProofCannotAuthorizeNewNoCloudSecretReferences(t *testing.T) {
	for _, field := range []string{"user", "network"} {
		t.Run(field, func(t *testing.T) {
			store, _, scope := fixtureStore(t)
			vm := independentStartupVM(scope, true)
			publication, err := PrepareNewVMStartup(scope, vm, VMStartupHints{NoCloudVolumeName: "cloud", NoCloudDiskName: "cloud"})
			if err != nil {
				t.Fatal(err)
			}
			vm.UID = "review-created-uid"
			core := kubefake.NewSimpleClientset()
			if err := store.SealNewVMStartup(context.Background(), core, vm, *publication); err != nil {
				t.Fatal(err)
			}
			if _, err := store.ReadVMStartup(context.Background(), core, scope, vm); err != nil {
				t.Fatal("positive publication failed", err)
			}
			ref := &corev1.LocalObjectReference{Name: "review-unknown-initialization"}
			if field == "user" {
				vm.Spec.Template.Spec.Volumes[0].CloudInitNoCloud.UserDataSecretRef = ref
			} else {
				vm.Spec.Template.Spec.Volumes[0].CloudInitNoCloud.NetworkDataSecretRef = ref
			}
			policy, err := store.ReadVMStartup(context.Background(), core, scope, vm)
			if !errors.Is(err, ErrConflict) && !errors.Is(err, ErrRestoreCapability) {
				if directory := os.Getenv("STARTUP_REVIEW_EVIDENCE"); directory != "" {
					os.MkdirAll(directory, 0700)
					data, _ := json.MarshalIndent(map[string]interface{}{"field": field, "private_producer": policy.ProducerUID, "accepted_datasource": policy.DataSourceKind, "introduced_reference_name": ref.Name, "kubernetes_actions": len(core.Actions()), "error_is_nil": err == nil}, "", "  ")
					os.WriteFile(filepath.Join(directory, "unknown-nocloud-"+field+"-reference.json"), data, 0600)
				}
				t.Fatalf("old private proof authorized a new unsealed NoCloud %s SecretRef: err=%v", field, err)
			}
		})
	}
}
func TestIndependentV121AdvancedUserDevicesRemainOutsideGeneratedMediaAuthority(t *testing.T) {
	store, _, scope := fixtureStore(t)
	vm := independentStartupVM(scope, false)
	vm.Spec.Template.Spec.Volumes = []kv.Volume{{Name: "user-config", VolumeSource: kv.VolumeSource{ConfigMap: &kv.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "user-config"}}}}, {Name: "user-volume", VolumeSource: kv.VolumeSource{Secret: &kv.SecretVolumeSource{SecretName: "user-volume"}}}}
	vm.Spec.Template.Spec.Domain.Devices.Disks = []kv.Disk{{Name: "user-config"}, {Name: "user-volume"}}
	before := vm.Spec.Template.Spec.DeepCopy()
	publication, err := PrepareNewVMStartup(scope, vm, VMStartupHints{})
	if err != nil {
		t.Fatal(err)
	}
	vm.UID = "review-created-uid"
	core := kubefake.NewSimpleClientset()
	if err := store.SealNewVMStartup(context.Background(), core, vm, *publication); err != nil {
		t.Fatal(err)
	}
	policy, err := store.ReadVMStartup(context.Background(), core, scope, vm)
	if err != nil || policy.DataSourceKind != "None" || len(policy.GeneratedMedia) != 0 {
		t.Fatal("advanced user devices acquired generated ownership", err)
	}
	if !reflect.DeepEqual(before.Volumes, vm.Spec.Template.Spec.Volumes) || len(core.Actions()) != 0 {
		t.Fatal("private proof mutated or fetched arbitrary user media")
	}
}
func TestIndependentV121NativeMetadataRejectsDuplicateBackupReplacementUIDAndFractionalQuota(t *testing.T) {
	for _, change := range []string{"duplicate", "replacement-uid", "fractional-quota"} {
		t.Run(change, func(t *testing.T) {
			f := snapshotProjectionFixture(t)
			f.content.Spec.VolumeBackups[0].PersistentVolumeClaim.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
			dto, err := f.project(SnapshotProof{Sealed: true, SnapshotUID: string(f.snapshot.UID), ContentUID: string(f.content.UID), VolumeSnapshotUIDs: map[string]string{"volume-snapshot-a": "volume-snapshot-uid"}})
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "duplicate":
				f.content.Spec.VolumeBackups = append(f.content.Spec.VolumeBackups, f.content.Spec.VolumeBackups[0])
			case "replacement-uid":
				f.content.Spec.VolumeBackups[0].PersistentVolumeClaim.UID = "replacement-pvc"
			case "fractional-quota":
				f.content.Spec.VolumeBackups[0].PersistentVolumeClaim.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("1536Mi")
			}
			if _, err := NativeRestoreConfig(dto, f.content); err == nil {
				t.Fatal("unsafe native metadata accepted", change)
			}
		})
	}
}

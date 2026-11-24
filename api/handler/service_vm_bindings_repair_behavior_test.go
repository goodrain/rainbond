package handler

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util/bcode"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ktesting "k8s.io/client-go/testing"
)

// capability_id: rainbond.vm-disk.active-binding-repair
func TestBindingRepairConfigCreationUsesSameAdmissionBeforeSQL(t *testing.T) {
	for _, scenario := range []string{"mapped", "busy", "fence", "normal", "non-vm"} {
		t.Run(scenario, func(t *testing.T) {
			f := newBindingConsumerFixture(t)
			f.action.dbmanager = independentAddManager{bindingConsumerDB{database: f.database}}
			if scenario != "mapped" && scenario != "non-vm" {
				current, _ := f.store.Read(f.scope, "")
				_, err := f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
					c.State.Bindings = map[string]vmrestore.Binding{}
					c.State.BindingGeneration++
					if scenario == "busy" {
						if err := c.State.AdmitMutation("different-operation", "other"); err != nil {
							return vmrestore.Mutation{}, err
						}
					}
					return vmrestore.Mutation{State: c.State}, nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "fence" {
				current, _ := f.store.Read(f.scope, "")
				hash := strings.Repeat("a", 64)
				_, err := f.store.Mutate(f.scope, vmVolumeExpectation(current, "restore-proof", hash, "prepare"), func(c vmrestore.Current) (vmrestore.Mutation, error) {
					c.State.Fence = &vmrestore.Fence{OperationID: "restore-proof", Kind: "restore", Phase: "prepare", ManifestHash: hash, TargetVMAbsent: true}
					receipt := vmrestore.NewReceipt(f.scope, "restore-proof", hash)
					return vmrestore.Mutation{State: c.State, Receipt: &receipt}, nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "non-vm" {
				if err := f.database.Model(&dbmodel.TenantServices{}).Where("service_id = ?", f.scope.ServiceID).UpdateColumn("extend_method", "stateless").Error; err != nil {
					t.Fatal(err)
				}
			}
			syncs := 0
			f.action.syncVirtualMachineSpecHook = func(string) error {
				syncs++
				if scenario == "normal" {
					current, err := f.store.Read(f.scope, "")
					if err != nil || current.State.Admission == nil || current.State.Admission.Action != "add-config:new-config" {
						t.Errorf("config creation sync ran without its admission: %v", err)
					}
				}
				return nil
			}
			request := &dbmodel.TenantServiceVolume{ServiceID: f.scope.ServiceID, VolumeName: "new-config", VolumeType: dbmodel.ConfigFileVolumeType.String(), VolumePath: "/etc/new-config"}
			failure := f.action.VolumnVar(request, f.scope.TenantID, "fixture-only", "add")
			var volumes, configs int
			f.database.Model(&dbmodel.TenantServiceVolume{}).Where("service_id = ? AND volume_name = ?", f.scope.ServiceID, request.VolumeName).Count(&volumes)
			f.database.Model(&dbmodel.TenantServiceConfigFile{}).Where("service_id = ? AND volume_name = ?", f.scope.ServiceID, request.VolumeName).Count(&configs)
			if scenario == "normal" || scenario == "non-vm" {
				if failure != nil || syncs != 1 || volumes != 1 || configs != 1 {
					t.Fatalf("normal config creation changed behavior: failure=%v sync=%d volume=%d config=%d", failure, syncs, volumes, configs)
				}
				if scenario == "normal" {
					current, err := f.store.Read(f.scope, "")
					if err != nil || current.State.Admission != nil {
						t.Fatal("successful config creation retained admission")
					}
				}
			} else if failure == nil || failure.Code != 409 || syncs != 0 || volumes != 0 || configs != 0 {
				t.Fatalf("protected config creation had side effects: failure=%v sync=%d volume=%d config=%d", failure, syncs, volumes, configs)
			}
		})
	}
}

// capability_id: rainbond.vm-disk.active-binding-repair
func TestBindingRepairRepeatedUnknownResizeReclaimsOnlyItsExecution(t *testing.T) {
	f := newBindingConsumerFixture(t)
	failures := 2
	f.core.PrependReactor("update", "persistentvolumeclaims", func(ktesting.Action) (bool, runtime.Object, error) {
		if failures > 0 {
			failures--
			return true, nil, fmt.Errorf("fixture update response unavailable")
		}
		return false, nil, nil
	})
	request := &apimodel.UpdVolumeReq{VolumeName: f.volume.VolumeName, VolumePath: f.volume.VolumePath, VolumeType: f.volume.VolumeType, VolumeCapacity: int64PtrForBinding(2)}
	if err := f.action.UpdVolume(f.scope.ServiceID, request); err == nil {
		t.Fatal("expected first unknown update")
	}
	initial, _ := f.store.Read(f.scope, "")
	if initial.State.Admission == nil || !initial.State.Admission.OutcomeUnknown {
		t.Fatal("first unknown update lost its permit")
	}
	operation := initial.State.Admission.Token
	entered, release := make(chan struct{}), make(chan struct{})
	updates := &atomic.Int32{}
	f.action.kubeClient = independentBlockCore{f.core, updates, entered, release}
	done := make(chan error, 1)
	go func() { done <- f.action.UpdVolume(f.scope.ServiceID, request) }()
	select {
	case <-entered:
	case first := <-done:
		t.Fatalf("retry failed before update: %v", first)
	case <-time.After(10 * time.Second):
		t.Fatal("retry did not reach update")
	}
	second := f.action.UpdVolume(f.scope.ServiceID, request)
	current, readErr := f.store.Read(f.scope, "")
	close(release)
	retry := <-done
	if code, ok := second.(bcode.Coder); !ok || code.GetStatus() != 409 || updates.Load() != 1 {
		t.Errorf("duplicate unknown retry entered external IO: %v, updates=%d", second, updates.Load())
	}
	if readErr != nil || current.State.Admission == nil || current.State.Admission.OutcomeUnknown || current.State.Admission.Token != operation {
		t.Errorf("retry was not atomically claimed: %v", readErr)
	}
	if retry == nil {
		t.Fatal("expected second unknown update")
	}
	current, _ = f.store.Read(f.scope, "")
	if current.State.Admission == nil || !current.State.Admission.OutcomeUnknown || current.State.Admission.Token != operation {
		t.Fatal("second unknown update did not retain the same retryable operation")
	}
	f.action.kubeClient = f.core
	if err := f.action.UpdVolume(f.scope.ServiceID, request); err != nil {
		t.Fatalf("third request could not reconcile known capacity: %v", err)
	}
	current, _ = f.store.Read(f.scope, "")
	if current.State.Admission != nil {
		t.Fatal("known completed retry did not finish admission")
	}
	for name, expected := range map[string]string{"active-a": "2Gi", "manual42": "1Gi"} {
		pvc, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		quantity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		if quantity.String() != expected {
			t.Errorf("unexpected %s request=%s", name, quantity.String())
		}
	}
}

// capability_id: rainbond.vm-disk.active-binding-repair
func TestBindingRepairCompletedDeleteReplayRequiresUnambiguousEvidence(t *testing.T) {
	for _, scenario := range []string{"recreated-target", "unknown-name", "duplicate-receipt", "wrong-resource"} {
		t.Run(scenario, func(t *testing.T) {
			f := newBindingConsumerFixture(t)
			original, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "active-a", metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			request := &dbmodel.TenantServiceVolume{ServiceID: f.scope.ServiceID, VolumeName: f.volume.VolumeName}
			if failure := f.action.VolumnVar(request, f.scope.TenantID, "", "delete"); failure != nil {
				t.Fatal(failure)
			}
			if scenario == "recreated-target" {
				replacement := original.DeepCopy()
				replacement.UID = "unrelated-replacement-uid"
				if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Create(context.Background(), replacement, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "unknown-name" {
				request.VolumeName = "never-recorded"
			}
			if scenario == "duplicate-receipt" || scenario == "wrong-resource" {
				var rows []dbmodel.ComponentK8sAttributes
				if err := f.database.Where("component_id = ? AND name LIKE ?", f.scope.ServiceID, vmrestore.ReceiptPrefix+"%").Find(&rows).Error; err != nil || len(rows) != 1 {
					t.Fatal("expected one completed receipt")
				}
				var old vmrestore.Receipt
				if err := vmrestore.DecodeRecord(rows[0].AttributeValue, &old); err != nil {
					t.Fatal(err)
				}
				current, _ := f.store.Read(f.scope, "")
				_, err := f.store.Mutate(f.scope, vmVolumeExpectation(current, "disk-delete-ambiguous", old.ManifestHash, "complete"), func(c vmrestore.Current) (vmrestore.Mutation, error) {
					receipt := vmrestore.NewReceipt(f.scope, "disk-delete-ambiguous", old.ManifestHash)
					receipt.OldBindings, receipt.Resources = old.OldBindings, old.Resources
					if scenario == "wrong-resource" {
						receipt.Resources = append([]vmrestore.ResourceRef(nil), old.Resources...)
						receipt.Resources[0].UID = "different-unrelated-uid"
					}
					return vmrestore.Mutation{State: c.State, Receipt: &receipt}, nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			f.core.ClearActions()
			failure := f.action.VolumnVar(request, f.scope.TenantID, "", "delete")
			expected := 409
			if scenario == "unknown-name" {
				expected = 404
			}
			if failure == nil || failure.Code != expected {
				t.Errorf("ambiguous replay was incorrectly accepted: %v", failure)
			}
			for _, action := range f.core.Actions() {
				if action.GetVerb() == "delete" {
					t.Fatal("replay issued a new destructive operation")
				}
			}
		})
	}
}

// capability_id: rainbond.vm-disk.active-binding-repair
func TestBindingRepairDeleteCommitFailureKeepsSameReceiptRetryable(t *testing.T) {
	f := newBindingConsumerFixture(t)
	f.database.DB().SetMaxOpenConns(1)
	for _, statement := range []string{
		"PRAGMA foreign_keys = ON",
		"CREATE TABLE consumer_commit_guard (id INTEGER PRIMARY KEY, volume_id INTEGER REFERENCES tenant_services_volume(ID) DEFERRABLE INITIALLY DEFERRED)",
		"INSERT INTO consumer_commit_guard (id, volume_id) VALUES (1, 42)",
	} {
		if err := f.database.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	request := &dbmodel.TenantServiceVolume{ServiceID: f.scope.ServiceID, VolumeName: f.volume.VolumeName}
	if failure := f.action.VolumnVar(request, f.scope.TenantID, "", "delete"); failure == nil {
		t.Fatal("expected deferred constraint to reject SQL COMMIT")
	}
	current, err := f.store.Read(f.scope, "")
	if err != nil || current.State.Admission == nil || !current.State.Admission.OutcomeUnknown {
		t.Errorf("failed COMMIT did not retain a retryable unknown outcome: %v", err)
	}
	var rows int
	f.database.Model(&dbmodel.TenantServiceVolume{}).Where("ID = ?", 42).Count(&rows)
	if rows != 1 || len(current.State.Bindings) == 0 {
		t.Fatal("failed COMMIT lost logical disk or binding")
	}
	if err := f.database.Exec("DELETE FROM consumer_commit_guard WHERE id = 1").Error; err != nil {
		t.Fatal(err)
	}
	if failure := f.action.VolumnVar(request, f.scope.TenantID, "", "delete"); failure != nil {
		t.Errorf("same operation could not finish after COMMIT failure: %v", failure)
	}
}

// capability_id: rainbond.vm-disk.active-binding-repair
func TestBindingRepairResizeSQLFailureRetriesKnownExternalCapacity(t *testing.T) {
	for _, phase := range []string{"update", "commit"} {
		t.Run(phase, func(t *testing.T) {
			f := newBindingConsumerFixture(t)
			f.database.DB().SetMaxOpenConns(1)
			statements := []string{"CREATE TRIGGER consumer_resize_guard BEFORE UPDATE OF volume_capacity ON tenant_services_volume BEGIN SELECT RAISE(ABORT, 'fixture SQL update rejected'); END"}
			cleanup := "DROP TRIGGER consumer_resize_guard"
			if phase == "commit" {
				statements = []string{
					"PRAGMA foreign_keys = ON",
					"CREATE UNIQUE INDEX consumer_capacity_key ON tenant_services_volume(volume_capacity)",
					"CREATE TABLE consumer_resize_guard (id INTEGER PRIMARY KEY, capacity INTEGER REFERENCES tenant_services_volume(volume_capacity) DEFERRABLE INITIALLY DEFERRED)",
					"INSERT INTO consumer_resize_guard (id, capacity) VALUES (1, 1)",
				}
				cleanup = "DELETE FROM consumer_resize_guard WHERE id = 1"
			}
			for _, statement := range statements {
				if err := f.database.Exec(statement).Error; err != nil {
					t.Fatal(err)
				}
			}
			request := &apimodel.UpdVolumeReq{VolumeName: f.volume.VolumeName, VolumePath: f.volume.VolumePath, VolumeType: f.volume.VolumeType, VolumeCapacity: int64PtrForBinding(2)}
			if err := f.action.UpdVolume(f.scope.ServiceID, request); err == nil {
				t.Fatal("expected SQL completion failure after PVC update")
			}
			current, err := f.store.Read(f.scope, "")
			if err != nil || current.State.Admission == nil || !current.State.Admission.OutcomeUnknown || len(current.State.Bindings) != 1 {
				t.Fatalf("SQL failure lost the original retryable operation: %v", err)
			}
			operation := current.State.Admission.Token
			receipt, err := f.store.Read(f.scope, operation)
			if err != nil || receipt.Receipt == nil || !receipt.Receipt.Steps["issued"].Applied || receipt.Receipt.Steps["complete"].Applied {
				t.Fatalf("failed SQL transaction left a completed receipt: %v", err)
			}
			var stored dbmodel.TenantServiceVolume
			if err := f.database.Where("ID = ?", 42).First(&stored).Error; err != nil || stored.VolumeCapacity != 1 {
				t.Fatalf("failed SQL transaction changed logical capacity: %v", err)
			}
			for name, expected := range map[string]string{"active-a": "2Gi", "manual42": "1Gi"} {
				pvc, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), name, metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				quantity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
				if quantity.String() != expected {
					t.Fatalf("SQL failure changed the wrong external capacity: %s=%s", name, quantity.String())
				}
			}
			if err := f.database.Exec(cleanup).Error; err != nil {
				t.Fatal(err)
			}
			f.core.ClearActions()
			if err := f.action.UpdVolume(f.scope.ServiceID, request); err != nil {
				t.Fatalf("same request could not reconcile its known PVC capacity: %v", err)
			}
			current, err = f.store.Read(f.scope, operation)
			if err != nil || current.State.Admission != nil || current.Receipt == nil || !current.Receipt.Steps["complete"].Applied {
				t.Fatalf("successful retry did not complete the original operation: %v", err)
			}
			if err := f.database.Where("ID = ?", 42).First(&stored).Error; err != nil || stored.VolumeCapacity != 2 {
				t.Fatalf("retry did not commit logical capacity: %v", err)
			}
			for _, action := range f.core.Actions() {
				if action.GetVerb() == "update" || action.GetVerb() == "delete" {
					t.Fatal("SQL-only retry repeated an external mutation")
				}
			}
		})
	}
}

func partialBindingReclaimFixture(t *testing.T) (bindingConsumerFixture, *dbmodel.TenantServiceVolume) {
	t.Helper()
	f := newBindingConsumerFixture(t)
	dv := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cdi.kubevirt.io/v1beta1", "kind": "DataVolume",
		"metadata": map[string]interface{}{"name": "a-dv", "namespace": f.scope.Namespace, "uid": "a-dv-uid",
			"labels": map[string]interface{}{"service_id": f.scope.ServiceID, "tenant_id": f.scope.TenantID}},
		"status": map[string]interface{}{"claimName": "active-a"},
	}}
	if _, err := f.dynamic.Resource(dataVolumeGVR).Namespace(f.scope.Namespace).Create(context.Background(), dv, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	current, err := f.store.Read(f.scope, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		binding := c.State.Bindings["42"]
		binding.ActiveDV = &vmrestore.ObjectRef{Kind: "DataVolume", Namespace: f.scope.Namespace, Name: "a-dv", UID: "a-dv-uid"}
		c.State.Bindings["42"] = binding
		c.State.BindingGeneration++
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	fail := true
	f.core.PrependReactor("delete", "persistentvolumeclaims", func(ktesting.Action) (bool, runtime.Object, error) {
		if fail {
			return true, nil, fmt.Errorf("fixture partial PVC delete failure")
		}
		return false, nil, nil
	})
	request := &dbmodel.TenantServiceVolume{ServiceID: f.scope.ServiceID, VolumeName: f.volume.VolumeName}
	if failure := f.action.VolumnVar(request, f.scope.TenantID, "", "delete"); failure == nil {
		t.Fatal("expected first delete to leave a surviving PVC")
	}
	fail = false
	if _, err := f.dynamic.Resource(dataVolumeGVR).Namespace(f.scope.Namespace).Get(context.Background(), "a-dv", metav1.GetOptions{}); err == nil {
		t.Fatal("partial reclaim setup did not delete DV")
	}
	if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "active-a", metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}
	return f, request
}

// capability_id: rainbond.vm-disk.active-binding-repair
func TestBindingRepairPartialReclaimRetainsIdentityAndOwnershipGuards(t *testing.T) {
	for _, scenario := range []string{"pvc-replacement", "dv-replacement", "generation", "admission", "unreadable-references"} {
		t.Run(scenario, func(t *testing.T) {
			f, request := partialBindingReclaimFixture(t)
			current, err := f.store.Read(f.scope, "")
			if err != nil || current.State.Admission == nil {
				t.Fatal("missing partial operation")
			}
			switch scenario {
			case "pvc-replacement":
				pvc, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "active-a", metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				if err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Delete(context.Background(), pvc.Name, metav1.DeleteOptions{}); err != nil {
					t.Fatal(err)
				}
				pvc.UID = "replacement-a-uid"
				if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Create(context.Background(), pvc, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
			case "dv-replacement":
				dv := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "cdi.kubevirt.io/v1beta1", "kind": "DataVolume", "metadata": map[string]interface{}{"name": "a-dv", "namespace": f.scope.Namespace, "uid": "replacement-dv-uid"}}}
				if _, err := f.dynamic.Resource(dataVolumeGVR).Namespace(f.scope.Namespace).Create(context.Background(), dv, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
			case "generation", "admission":
				_, err := f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration, AdmissionOwner: current.State.Admission.Token}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
					if scenario == "generation" {
						c.State.BindingGeneration++
					} else {
						if err := c.State.FinishMutation(c.State.Admission.Token, true); err != nil {
							return vmrestore.Mutation{}, err
						}
						if err := c.State.AdmitMutation("foreign-owner", "foreign-action"); err != nil {
							return vmrestore.Mutation{}, err
						}
					}
					return vmrestore.Mutation{State: c.State}, nil
				})
				if err != nil {
					t.Fatal(err)
				}
			case "unreadable-references":
				f.core.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("fixture reference list unavailable")
				})
			}
			f.core.ClearActions()
			f.dynamic.ClearActions()
			failure := f.action.VolumnVar(request, f.scope.TenantID, "", "delete")
			if failure == nil || scenario != "unreadable-references" && failure.Code != 409 {
				t.Fatalf("unsafe partial reclaim accepted: %v", failure)
			}
			for _, action := range append(f.core.Actions(), f.dynamic.Actions()...) {
				if action.GetVerb() == "delete" {
					t.Fatal("refused partial reclaim performed an external delete")
				}
			}
			for _, name := range []string{"active-a", "manual42"} {
				if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), name, metav1.GetOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			var rows int
			f.database.Model(&dbmodel.TenantServiceVolume{}).Where("ID = ?", 42).Count(&rows)
			current, err = f.store.Read(f.scope, "")
			if err != nil || rows != 1 || len(current.State.Bindings) != 1 || current.State.Admission == nil {
				t.Fatal("refused reclaim lost logical row, binding or admission")
			}
		})
	}
}

// capability_id: rainbond.vm-disk.active-binding-repair
func TestBindingRepairPartialReclaimRetryExcludesConcurrentIdenticalDelete(t *testing.T) {
	f, request := partialBindingReclaimFixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	var deletes atomic.Int32
	f.core.PrependReactor("delete", "persistentvolumeclaims", func(ktesting.Action) (bool, runtime.Object, error) {
		if deletes.Add(1) == 1 {
			close(entered)
			<-release
		}
		return false, nil, nil
	})
	done := make(chan error, 1)
	go func() {
		if failure := f.action.VolumnVar(request, f.scope.TenantID, "", "delete"); failure != nil {
			done <- failure
		} else {
			done <- nil
		}
	}()
	select {
	case <-entered:
	case err := <-done:
		t.Fatalf("retry did not reach surviving PVC: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("retry did not reach surviving PVC")
	}
	duplicate := f.action.VolumnVar(request, f.scope.TenantID, "", "delete")
	current, readErr := f.store.Read(f.scope, "")
	close(release)
	first := <-done
	if duplicate == nil || duplicate.Code != 409 || deletes.Load() != 1 {
		t.Errorf("concurrent duplicate entered reclaim: failure=%v deletes=%d", duplicate, deletes.Load())
	}
	if readErr != nil || current.State.Admission == nil || current.State.Admission.OutcomeUnknown {
		t.Errorf("executing retry did not retain its claimed admission: %v", readErr)
	}
	if first != nil {
		t.Fatalf("original retry failed after duplicate was refused: %v", first)
	}
	current, readErr = f.store.Read(f.scope, "")
	var rows int
	f.database.Model(&dbmodel.TenantServiceVolume{}).Where("ID = ?", 42).Count(&rows)
	if readErr != nil || rows != 0 || len(current.State.Bindings) != 0 || current.State.Admission != nil {
		t.Fatal("successful retry did not retire exactly its row, binding and admission")
	}
	if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "manual42", metav1.GetOptions{}); err != nil {
		t.Fatal("successful partial retry damaged retained B")
	}
}

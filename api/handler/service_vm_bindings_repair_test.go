package handler

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goodrain/rainbond/api/util/bcode"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/jinzhu/gorm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	typedcore "k8s.io/client-go/kubernetes/typed/core/v1"
	ktesting "k8s.io/client-go/testing"
)

// capability_id: rainbond.vm-disk.active-binding-repair
func TestIndependentBindingPartialReclaimMustRecheckSurvivingA(t *testing.T) {
	f := newBindingConsumerFixture(t)
	dv := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "cdi.kubevirt.io/v1beta1", "kind": "DataVolume", "metadata": map[string]interface{}{"name": "a-dv", "namespace": f.scope.Namespace, "uid": "a-dv-uid", "labels": map[string]interface{}{"service_id": f.scope.ServiceID, "tenant_id": f.scope.TenantID}}, "status": map[string]interface{}{"claimName": "active-a"}}}
	if _, err := f.dynamic.Resource(dataVolumeGVR).Namespace(f.scope.Namespace).Create(context.Background(), dv, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	current, err := f.store.Read(f.scope, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		b := c.State.Bindings["42"]
		b.ActiveDV = &vmrestore.ObjectRef{Kind: "DataVolume", Namespace: f.scope.Namespace, Name: "a-dv", UID: "a-dv-uid"}
		c.State.Bindings["42"] = b
		c.State.BindingGeneration++
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	failPVC := true
	f.core.PrependReactor("delete", "persistentvolumeclaims", func(a ktesting.Action) (bool, runtime.Object, error) {
		if a.(ktesting.DeleteAction).GetName() == "active-a" && failPVC {
			return true, nil, fmt.Errorf("independent temporary PVC failure")
		}
		return false, nil, nil
	})
	request := &dbmodel.TenantServiceVolume{ServiceID: f.scope.ServiceID, VolumeName: f.volume.VolumeName}
	if first := f.action.VolumnVar(request, f.scope.TenantID, "", "delete"); first == nil {
		t.Fatal("setup expected first PVC delete to fail after DV disappeared")
	}
	if _, err := f.dynamic.Resource(dataVolumeGVR).Namespace(f.scope.Namespace).Get(context.Background(), "a-dv", metav1.GetOptions{}); err == nil {
		t.Fatal("setup DV still exists")
	}
	if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "active-a", metav1.GetOptions{}); err != nil {
		t.Fatal("setup lost surviving A")
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "late-consumer", Namespace: f.scope.Namespace, UID: "late-consumer-uid"}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "active-a"}}}}}}
	if _, err := f.core.CoreV1().Pods(f.scope.Namespace).Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	failPVC = false
	f.core.ClearActions()
	retry := f.action.VolumnVar(request, f.scope.TenantID, "", "delete")
	_, aErr := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "active-a", metav1.GetOptions{})
	var rows int
	f.database.Model(&dbmodel.TenantServiceVolume{}).Where("ID = ?", 42).Count(&rows)
	state, readErr := f.store.Read(f.scope, "")
	if readErr != nil {
		t.Fatal(readErr)
	}
	t.Logf("retry=%v A_error=%v logical_rows=%d active_binding=%t admission=%t", retry, aErr, rows, len(state.State.Bindings) > 0, state.State.Admission != nil)
	if retry == nil || retry.Code != 409 || aErr != nil || rows != 1 || len(state.State.Bindings) == 0 {
		t.Errorf("late external reference must preserve surviving A, logical row and binding")
		return
	}
	for _, a := range f.core.Actions() {
		if a.GetVerb() == "delete" {
			t.Errorf("retry issued DELETE while A has a new Pod reference")
		}
	}
	if err := f.core.CoreV1().Pods(f.scope.Namespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	if retry := f.action.VolumnVar(request, f.scope.TenantID, "", "delete"); retry != nil {
		t.Fatalf("retry must finish after the late reference is released, without requiring deleted DV to reappear: %v", retry)
	}
}

type independentBlockCore struct {
	kubernetes.Interface
	count            *atomic.Int32
	entered, release chan struct{}
}

func (c independentBlockCore) CoreV1() typedcore.CoreV1Interface {
	return independentBlockV1{c.Interface.CoreV1(), c.count, c.entered, c.release}
}

type independentBlockV1 struct {
	typedcore.CoreV1Interface
	count            *atomic.Int32
	entered, release chan struct{}
}

func (c independentBlockV1) PersistentVolumeClaims(ns string) typedcore.PersistentVolumeClaimInterface {
	return independentBlockPVC{c.CoreV1Interface.PersistentVolumeClaims(ns), c.count, c.entered, c.release}
}

type independentBlockPVC struct {
	typedcore.PersistentVolumeClaimInterface
	count            *atomic.Int32
	entered, release chan struct{}
}

func (c independentBlockPVC) Update(ctx context.Context, pvc *corev1.PersistentVolumeClaim, opts metav1.UpdateOptions) (*corev1.PersistentVolumeClaim, error) {
	if c.count.Add(1) == 1 {
		close(c.entered)
		<-c.release
	}
	return c.PersistentVolumeClaimInterface.Update(ctx, pvc, opts)
}

// capability_id: rainbond.vm-disk.active-binding-repair
func TestIndependentBindingSameResizeRequestCannotCompleteAnotherInflightCall(t *testing.T) {
	f := newBindingConsumerFixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	counter := &atomic.Int32{}
	f.action.kubeClient = independentBlockCore{f.core, counter, entered, release}
	done := make(chan error, 1)
	go func() { done <- f.action.expandVolumeClaims(context.Background(), f.service, f.volume, 2) }()
	select {
	case <-entered:
	case err := <-done:
		t.Fatalf("first failed before write: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("first never entered write")
	}
	second := f.action.expandVolumeClaims(context.Background(), f.service, f.volume, 2)
	state, readErr := f.store.Read(f.scope, "")
	held := readErr == nil && state.State.Admission != nil
	close(release)
	first := <-done
	t.Logf("second=%v updates=%d first_permit_held=%t first=%v", second, counter.Load(), held, first)
	code, ok := second.(bcode.Coder)
	if !ok || code.GetStatus() != 409 || counter.Load() != 1 || !held {
		t.Error("same request ran concurrently and/or released the first call's live admission")
	}
}

type independentAddManager struct{ bindingConsumerDB }

func (m independentAddManager) TenantServiceVolumeDaoTransactions(tx *gorm.DB) dbdao.TenantServiceVolumeDao {
	return independentAddVolume{bindingConsumerVolumes{database: tx}}
}
func (m independentAddManager) TenantServiceConfigFileDaoTransactions(tx *gorm.DB) dbdao.TenantServiceConfigFileDao {
	return independentAddConfig{bindingConsumerConfig{database: tx}}
}

type independentAddVolume struct{ bindingConsumerVolumes }

func (d independentAddVolume) AddModel(row dbmodel.Interface) error {
	return d.database.Create(row).Error
}

type independentAddConfig struct{ bindingConsumerConfig }

func (d independentAddConfig) AddModel(row dbmodel.Interface) error {
	return d.database.Create(row).Error
}

// capability_id: rainbond.vm-disk.active-binding-repair
func TestIndependentBindingConfigAddMustNotBypassAdmission(t *testing.T) {
	f := newBindingConsumerFixture(t)
	f.action.dbmanager = independentAddManager{bindingConsumerDB{database: f.database}}
	current, _ := f.store.Read(f.scope, "")
	_, err := f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		if err := c.State.AdmitMutation("other-operation", "other-disk-change"); err != nil {
			return vmrestore.Mutation{}, err
		}
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	synced := false
	f.action.syncVirtualMachineSpecHook = func(string) error { synced = true; return nil }
	request := &dbmodel.TenantServiceVolume{ServiceID: f.scope.ServiceID, VolumeName: "new-config", VolumeType: dbmodel.ConfigFileVolumeType.String(), VolumePath: "/test-config"}
	failure := f.action.VolumnVar(request, f.scope.TenantID, "fixture content", "add")
	var rows int
	f.database.Model(&dbmodel.TenantServiceVolume{}).Where("service_id = ? AND volume_name = ?", f.scope.ServiceID, "new-config").Count(&rows)
	t.Logf("failure=%v generic_sync=%t inserted_rows=%d", failure, synced, rows)
	if failure == nil || failure.Code != 409 || synced || rows != 0 {
		t.Error("config add wrote SQL/called generic sync while another mutation held admission and restored A was mapped")
	}
}

// capability_id: rainbond.vm-disk.active-binding-repair
func TestIndependentBindingConfigClaimBlocksReferenceAndRetainsUnknown(t *testing.T) {
	f := newBindingConsumerFixture(t)
	current, _ := f.store.Read(f.scope, "")
	_, err := f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		c.State.Bindings = map[string]vmrestore.Binding{}
		c.State.BindingGeneration++
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	first := f.action.withLegacyVMConfig(f.service, "update-config:test", func() error {
		handled, failure := f.action.addVMSourceRelation(&dbmodel.TenantServiceMountRelation{ServiceID: "foreign", DependServiceID: f.scope.ServiceID, VolumeName: f.volume.VolumeName})
		if !handled || failure == nil || failure.Code != 409 {
			t.Errorf("reference was admitted during config callback: handled=%t failure=%v", handled, failure)
		}
		return fmt.Errorf("independent lost config response")
	})
	if first == nil {
		t.Fatal("expected unknown outcome")
	}
	state, _ := f.store.Read(f.scope, "")
	if state.State.Admission == nil || !state.State.Admission.OutcomeUnknown {
		t.Fatal("unknown config result lost admission")
	}
	err = f.action.withLegacyVMConfig(f.service, "update-config:test", func() error {
		observed, _ := f.store.Read(f.scope, "")
		if observed.State.Admission == nil || observed.State.Admission.OutcomeUnknown {
			t.Error("retry was not claimed before callback")
		}
		second := f.action.withLegacyVMConfig(f.service, "update-config:test", func() error { t.Error("duplicate config callback ran"); return nil })
		if !errors.Is(second, vmrestore.ErrBusy) {
			t.Errorf("concurrent retry was not busy: %v", second)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	state, _ = f.store.Read(f.scope, "")
	if state.State.Admission != nil {
		t.Error("successful config retry did not finish admission")
	}
}

// capability_id: rainbond.vm-disk.active-binding-repair
func TestIndependentBindingUnknownDeleteRemainsAnchoredToPreparedGeneration(t *testing.T) {
	f := newBindingConsumerFixture(t)
	consumer, err := f.action.vmVolumes(f.service)
	if err != nil {
		t.Fatal(err)
	}
	mutation, err := consumer.begin(context.Background(), f.volume, "delete", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := mutation.step("issued", func(*vmrestore.Current) error { return nil }); err != nil {
		t.Fatal(err)
	}
	mutation.pending()
	original, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "active-a", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	next := original.DeepCopy()
	next.Name = "next-a"
	next.UID = "next-a-uid"
	if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Create(context.Background(), next, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	state, _ := f.store.Read(f.scope, "")
	_, err = f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: state.State.ControlRevision, BindingGeneration: state.State.BindingGeneration, AdmissionOwner: state.State.Admission.Token}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		b := c.State.Bindings["42"]
		b.ActivePVC.Name = next.Name
		b.ActivePVC.UID = string(next.UID)
		c.State.Bindings["42"] = b
		c.State.BindingGeneration++
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	f.core.ClearActions()
	failure := f.action.VolumnVar(&dbmodel.TenantServiceVolume{ServiceID: f.scope.ServiceID, VolumeName: f.volume.VolumeName}, f.scope.TenantID, "", "delete")
	if failure == nil || failure.Code != 409 {
		t.Errorf("stale unknown retry not rejected: %v", failure)
	}
	for _, name := range []string{"active-a", "manual42", "next-a"} {
		if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), name, metav1.GetOptions{}); err != nil {
			t.Errorf("resource %s not retained: %v", name, err)
		}
	}
	var rows int
	f.database.Model(&dbmodel.TenantServiceVolume{}).Where("ID = ?", 42).Count(&rows)
	state, _ = f.store.Read(f.scope, "")
	if rows != 1 || state.State.Bindings["42"].ActivePVC.Name != "next-a" {
		t.Error("generation rejection lost logical row or new binding")
	}
	for _, a := range f.core.Actions() {
		if a.GetVerb() == "delete" || a.GetVerb() == "update" {
			t.Error("generation rejection performed a write")
		}
	}
}

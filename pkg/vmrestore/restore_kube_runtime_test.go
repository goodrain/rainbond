package vmrestore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kt "k8s.io/client-go/testing"
	kv "kubevirt.io/api/core/v1"
)

var runtimeMigrationGVR = schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachineinstancemigrations"}

func runtimeInstanceObject(t *testing.T, instance *kv.VirtualMachineInstance) *unstructured.Unstructured {
	t.Helper()
	copy := instance.DeepCopy()
	copy.APIVersion = "kubevirt.io/v1"
	copy.Kind = "VirtualMachineInstance"
	body, err := json.Marshal(copy)
	if err != nil {
		t.Fatal(err)
	}
	u := &unstructured.Unstructured{}
	if err := u.UnmarshalJSON(body); err != nil {
		t.Fatal(err)
	}
	u.SetAPIVersion("kubevirt.io/v1")
	u.SetKind("VirtualMachineInstance")
	return u
}

func kubeRuntimeFixture(t *testing.T) (restoreCommitFixture, *RestoreEngine, *KubeRestoreRuntime) {
	t.Helper()
	f, _, resources := resourceFixture(t, func(f restoreCommitFixture, x *RestoreExecution) {
		hash, err := f.store.NetworkIdentityHash(f.scope)
		if err != nil {
			t.Fatal(err)
		}
		x.NetworkHash = hash
	})
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{restoreVMGVR: "VirtualMachineList", restoreVMIGVR: "VirtualMachineInstanceList", runtimeMigrationGVR: "VirtualMachineInstanceMigrationList"})
	strategy := kv.RunStrategyAlways
	vm := &kv.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: f.intent.CurrentVM.Name, Namespace: f.scope.Namespace, UID: types.UID(f.intent.CurrentVM.UID), ResourceVersion: "1", Generation: 3, Labels: map[string]string{"service_id": f.scope.ServiceID, "tenant_id": f.scope.TenantID}}, Spec: kv.VirtualMachineSpec{RunStrategy: &strategy, Template: &kv.VirtualMachineInstanceTemplateSpec{Spec: kv.VirtualMachineInstanceSpec{Domain: kv.DomainSpec{Devices: kv.Devices{Interfaces: []kv.Interface{{Name: "default", MacAddress: "02:00:00:00:00:42"}}}}}}}}
	u, err := desiredObject(vm, "VirtualMachine")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dyn.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Create(context.Background(), u, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	r := &KubeRestoreRuntime{Store: f.store, Core: resources.Core, Dynamic: dyn}
	e := &RestoreEngine{Store: f.store, Resources: resources, Runtime: r}
	e.WireRestoreResourceGuard()
	return f, e, r
}

// capability_id: rainbond.vm-restore.kubernetes-runtime
func TestKubeRuntimeHaltRequiresIssuedIntentAndObservedCompletion(t *testing.T) {
	f, engine, r := kubeRuntimeFixture(t)
	c, _ := f.store.Read(f.scope, f.expect.OperationID)
	request := engine.runtimeRequest(c, "halt")
	if _, err := r.Apply(context.Background(), request); !errors.Is(err, ErrConflict) {
		t.Fatal("unissued mutation accepted", err)
	}
	if _, err := engine.RuntimeStep(context.Background(), f.scope, f.expect.OperationID, "halt"); !errors.Is(err, ErrRestorePending) {
		t.Fatal("accepted update was not pending", err)
	}
	c, _ = f.store.Read(f.scope, f.expect.OperationID)
	if c.Receipt.Steps["restore-writers-clear"].Applied {
		t.Fatal("write response counted as completion")
	}
	if _, err := engine.RuntimeStep(context.Background(), f.scope, f.expect.OperationID, "halt"); err != nil {
		t.Fatal(err)
	}
	updates := 0
	for _, a := range r.Dynamic.(*dynamicfake.FakeDynamicClient).Actions() {
		if a.GetVerb() == "update" {
			updates++
		}
	}
	if updates != 1 {
		t.Fatal("halt replay repeated update", updates)
	}
	u, _ := r.Dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Get(context.Background(), request.VM.Name, metav1.GetOptions{})
	interfaces, _, _ := unstructured.NestedSlice(u.Object, "spec", "template", "spec", "domain", "devices", "interfaces")
	if len(interfaces) != 1 || textAt(interfaces[0].(map[string]interface{}), "macAddress") != "02:00:00:00:00:42" {
		t.Fatal("network devices changed during halt")
	}
}

// capability_id: rainbond.vm-restore.kubernetes-runtime
func TestKubeRuntimeUnknownUpdateIsObservedWithoutReissue(t *testing.T) {
	f, e, r := kubeRuntimeFixture(t)
	dyn := r.Dynamic.(*dynamicfake.FakeDynamicClient)
	updates := 0
	dyn.PrependReactor("update", "virtualmachines", func(kt.Action) (bool, runtime.Object, error) {
		updates++
		return true, nil, errors.New("fixture timeout")
	})
	for i := 0; i < 2; i++ {
		if _, err := e.RuntimeStep(context.Background(), f.scope, f.expect.OperationID, "halt"); !errors.Is(err, ErrRestoreUnknown) {
			t.Fatal(err)
		}
	}
	if updates != 1 {
		t.Fatal("unknown update was repeated", updates)
	}
	c, _ := f.store.Read(f.scope, f.expect.OperationID)
	if c.State.Fence == nil || c.Receipt.Steps["restore-writers-clear"].Applied {
		t.Fatal("unknown update released fence")
	}
}

// capability_id: rainbond.vm-restore.kubernetes-runtime
func TestKubeRuntimeRejectsVMReplacementAndChangedNetworkBeforeMutation(t *testing.T) {
	for _, change := range []string{"uid", "network"} {
		t.Run(change, func(t *testing.T) {
			f, e, r := kubeRuntimeFixture(t)
			if change == "uid" {
				u, _ := r.Dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Get(context.Background(), f.intent.CurrentVM.Name, metav1.GetOptions{})
				u.SetUID("replacement")
				_, _ = r.Dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Update(context.Background(), u, metav1.UpdateOptions{})
			} else {
				if err := f.db.Create(&dbmodel.TenantServicesPort{TenantID: f.scope.TenantID, ServiceID: f.scope.ServiceID, ContainerPort: 22, MappingPort: 30222, Protocol: "tcp"}).Error; err != nil {
					t.Fatal(err)
				}
			}
			before := len(r.Dynamic.(*dynamicfake.FakeDynamicClient).Actions())
			if _, err := e.RuntimeStep(context.Background(), f.scope, f.expect.OperationID, "halt"); !errors.Is(err, ErrConflict) {
				t.Fatal(err)
			}
			for _, a := range r.Dynamic.(*dynamicfake.FakeDynamicClient).Actions()[before:] {
				if a.GetVerb() == "update" {
					t.Fatal("unsafe mutation")
				}
			}
		})
	}
}

// capability_id: rainbond.vm-restore.kubernetes-runtime
func TestKubeRuntimeWriterScanIncludesUnlabelledDVAliasesPodsAndMigrations(t *testing.T) {
	for _, kind := range []string{"DataVolume", "Pod", "Migration"} {
		t.Run(kind, func(t *testing.T) {
			f, _, r := kubeRuntimeFixture(t)
			c, _ := f.store.Read(f.scope, f.expect.OperationID)
			b := c.Receipt.OldBindings["7"]
			b.ActiveDV = &ObjectRef{Kind: "DataVolume", Namespace: f.scope.Namespace, Name: "old-dv", UID: "old-dv-uid"}
			c.Receipt.OldBindings["7"] = b
			switch kind {
			case "DataVolume":
				vmi := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "kubevirt.io/v1", "kind": "VirtualMachineInstance", "metadata": map[string]interface{}{"name": "foreign-vmi", "namespace": f.scope.Namespace, "uid": "foreign"}, "spec": map[string]interface{}{"volumes": []interface{}{map[string]interface{}{"name": "disk", "dataVolume": map[string]interface{}{"name": "old-dv"}}}}, "status": map[string]interface{}{"phase": "Running"}}}
				_, _ = r.Dynamic.Resource(restoreVMIGVR).Namespace(f.scope.Namespace).Create(context.Background(), vmi, metav1.CreateOptions{})
			case "Pod":
				_, _ = r.Core.CoreV1().Pods(f.scope.Namespace).Create(context.Background(), &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "unlabelled-writer", Namespace: f.scope.Namespace}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "disk", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: b.ActivePVC.Name}}}}}}, metav1.CreateOptions{})
			case "Migration":
				u := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "kubevirt.io/v1", "kind": "VirtualMachineInstanceMigration", "metadata": map[string]interface{}{"name": "migration", "namespace": f.scope.Namespace, "uid": "migration-uid"}, "spec": map[string]interface{}{"vmiName": f.intent.CurrentVM.Name}, "status": map[string]interface{}{"phase": "Running"}}}
				_, _ = r.Dynamic.Resource(runtimeMigrationGVR).Namespace(f.scope.Namespace).Create(context.Background(), u, metav1.CreateOptions{})
			}
			_, clear, err := r.writers(context.Background(), c)
			if err != nil || clear {
				t.Fatal("writer omitted by complete scan", kind, err)
			}
		})
	}
}

// capability_id: rainbond.vm-restore.kubernetes-runtime
func TestCommittedRuntimeSyncAndOrdinaryStopStartKeepSelectedA(t *testing.T) {
	f, e, r := kubeRuntimeFixture(t)
	f.sealCopy(t)
	c, err := f.store.CommitRestoreDB(f.scope, f.expectStep(t, RestoreCommitStep), f.request(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range c.State.Bindings {
		class := "writable"
		pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: b.ActivePVC.Name, Namespace: f.scope.Namespace, UID: types.UID(b.ActivePVC.UID), Labels: map[string]string{"tenant_id": f.scope.TenantID, "service_id": f.scope.ServiceID}}, Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &class, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("20Gi")}}}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}
		if _, err := r.Core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Create(context.Background(), pvc, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	r.Resolver = NewResolver(f.store, r.Core, r.Dynamic)
	_, err = f.store.AcknowledgeRestoreDB(f.scope, f.expectStep(t, "restore-console-ack"), c.Receipt.ConfigCommitHash, strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := r.Dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Get(context.Background(), f.intent.CurrentVM.Name, metav1.GetOptions{})
	_ = unstructured.SetNestedField(u.Object, "Halted", "spec", "runStrategy")
	_ = unstructured.SetNestedField(u.Object, "Terminate", "spec", "template", "spec", "domain", "rebootPolicy")
	_ = unstructured.SetNestedSlice(u.Object, []interface{}{map[string]interface{}{"name": "platform-cloudinit", "cloudInitNoCloud": map[string]interface{}{"userData": "B_INIT_SENTINEL"}}, map[string]interface{}{"name": "user-advanced", "secret": map[string]interface{}{"secretName": "advanced-secret"}}}, "spec", "template", "spec", "volumes")
	_, _ = r.Dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Update(context.Background(), u, metav1.UpdateOptions{})
	if _, err := e.RuntimeStep(context.Background(), f.scope, f.expect.OperationID, "sync-halted"); !errors.Is(err, ErrRestorePending) {
		t.Fatal("first selected sync", err)
	}
	u, _ = r.Dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Get(context.Background(), f.intent.CurrentVM.Name, metav1.GetOptions{})
	if reboot, _, _ := unstructured.NestedString(u.Object, "spec", "template", "spec", "domain", "rebootPolicy"); reboot != "Terminate" {
		t.Fatal("sync-halted discarded the target-only domain policy")
	}

	_ = unstructured.SetNestedField(u.Object, int64(3), "status", "observedGeneration")
	_, _ = r.Dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Update(context.Background(), u, metav1.UpdateOptions{})
	if _, err := e.RuntimeStep(context.Background(), f.scope, f.expect.OperationID, "sync-halted"); err != nil {
		t.Fatal("observed sync", err)
	}
	e.Capacity = &testRestoreCapacity{}
	if _, err := e.Finalize(context.Background(), f.scope, f.expect.OperationID, false, false); !errors.Is(err, ErrConflict) {
		t.Fatal("required power restoration was skipped", err)
	}
	if _, err := e.RuntimeStep(context.Background(), f.scope, f.expect.OperationID, "resume"); !errors.Is(err, ErrRestorePending) {
		t.Fatal(err)
	}
	u, _ = r.Dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Get(context.Background(), f.intent.CurrentVM.Name, metav1.GetOptions{})
	var resumed kv.VirtualMachine
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &resumed); err != nil {
		t.Fatal(err)
	}
	instance := &kv.VirtualMachineInstance{ObjectMeta: metav1.ObjectMeta{Name: resumed.Name, Namespace: resumed.Namespace, UID: "resumed-instance", OwnerReferences: []metav1.OwnerReference{{Kind: "VirtualMachine", Name: resumed.Name, UID: resumed.UID}}}, Spec: resumed.Spec.Template.Spec, Status: kv.VirtualMachineInstanceStatus{Phase: kv.Running}}
	instanceObject := runtimeInstanceObject(t, instance)
	_, _ = r.Dynamic.Resource(restoreVMIGVR).Namespace(f.scope.Namespace).Create(context.Background(), instanceObject, metav1.CreateOptions{})
	if _, err := e.RuntimeStep(context.Background(), f.scope, f.expect.OperationID, "resume"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Finalize(context.Background(), f.scope, f.expect.OperationID, false, false); err != nil {
		t.Fatal(err)
	}
	selection, err := f.store.ReadRestoredRuntimeSelection(context.Background(), r.Resolver, f.scope)
	if err != nil || selection == nil {
		t.Fatal(selection, err)
	}
	for _, action := range []string{"stop", "start", "stop", "start"} {
		u, _ = r.Dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Get(context.Background(), f.intent.CurrentVM.Name, metav1.GetOptions{})
		var actual kv.VirtualMachine
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &actual); err != nil {
			t.Fatal(err)
		}
		_, _, err := f.store.ClaimOrdinaryVM(f.scope, action, refOf("VirtualMachine", &actual))
		if err != nil {
			t.Fatal(err)
		}
		wanted, err := ProjectRestoredRuntime(context.Background(), r.Core, &actual, *selection)
		if err != nil {
			t.Fatal(err)
		}
		strategy := kv.RunStrategyAlways
		if action == "stop" {
			strategy = kv.RunStrategyHalted
		}
		wanted.Spec.RunStrategy = &strategy
		body, err := EncodeRuntimeVM(wanted)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = r.Dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Update(context.Background(), body, metav1.UpdateOptions{})
		if action == "start" {
			instance := &kv.VirtualMachineInstance{ObjectMeta: metav1.ObjectMeta{Name: actual.Name, Namespace: actual.Namespace, UID: "new-instance", OwnerReferences: []metav1.OwnerReference{{Kind: "VirtualMachine", Name: actual.Name, UID: actual.UID}}}, Spec: wanted.Spec.Template.Spec, Status: kv.VirtualMachineInstanceStatus{Phase: kv.Running}}
			object := runtimeInstanceObject(t, instance)
			_, _ = r.Dynamic.Resource(restoreVMIGVR).Namespace(f.scope.Namespace).Create(context.Background(), object, metav1.CreateOptions{})
			bad := object.DeepCopy()
			_ = unstructured.SetNestedField(bad.Object, int64(99), "spec", "domain", "cpu", "cores")
			_, _ = r.Dynamic.Resource(restoreVMIGVR).Namespace(f.scope.Namespace).Update(context.Background(), bad, metav1.UpdateOptions{})
			if err := f.store.ObserveOrdinaryVMCompletion(context.Background(), r.Resolver, r.Dynamic, f.scope); err == nil {
				t.Fatal("a running instance with the wrong selected hardware completed admission")
			}
			_, _ = r.Dynamic.Resource(restoreVMIGVR).Namespace(f.scope.Namespace).Update(context.Background(), object, metav1.UpdateOptions{})
		} else {
			_ = r.Dynamic.Resource(restoreVMIGVR).Namespace(f.scope.Namespace).Delete(context.Background(), actual.Name, metav1.DeleteOptions{})
		}
		if err := f.store.ObserveOrdinaryVMCompletion(context.Background(), r.Resolver, r.Dynamic, f.scope); err != nil {
			t.Fatal(err)
		}
		if string(wanted.Spec.Template.Spec.Domain.Firmware.UUID) != testStartupPolicy("restored").FirmwareUUID {
			t.Fatal("A identity changed")
		}
		if wanted.Spec.Template.Spec.Domain.CPU.Cores != 2 {
			t.Fatal("A CPU config lost")
		}
		for _, v := range wanted.Spec.Template.Spec.Volumes {
			if v.CloudInitNoCloud != nil && strings.Contains(v.CloudInitNoCloud.UserData, "B_INIT_SENTINEL") {
				t.Fatal("B initialization reinjected")
			}
			if v.Name == "manual7" && v.PersistentVolumeClaim.ClaimName != f.targets[7].Name {
				t.Fatal("root fell back to old B")
			}
			if v.Name == "manual9" {
				t.Fatal("detached B-only disk reattached")
			}
		}
	}
}

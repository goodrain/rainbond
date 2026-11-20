package vmrestore

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

type engineTestPreparation struct {
	value RestorePreparation
	err   error
	calls int
}

func (p *engineTestPreparation) Read(context.Context, Scope, string, string, string) (RestorePreparation, error) {
	p.calls++
	return p.value, p.err
}

func enginePrepareFixture(t *testing.T) (restoreCommitFixture, *RestoreEngine, *engineTestPreparation) {
	t.Helper()
	model := newEngineScenario(t)
	f := newRestoreCommitFixture(t)
	f.seedBeforeAttributes(t)
	core := kubefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: f.scope.Namespace, UID: "namespace-uid"}})
	for _, b := range f.old {
		_, err := core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Create(context.Background(), &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: b.ActivePVC.Name, Namespace: f.scope.Namespace, UID: types.UID(b.ActivePVC.UID), Labels: map[string]string{"service_id": f.scope.ServiceID, "tenant_id": f.scope.TenantID}}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}, metav1.CreateOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{dataVolumeResource: "DataVolumeList", virtualMachineResource: "VirtualMachineList", virtualMachineInstanceResource: "VirtualMachineInstanceList", vmSnapshotResource: "VirtualMachineSnapshotList", vmSnapshotContentResource: "VirtualMachineSnapshotContentList", csiSnapshotResource: "VolumeSnapshotList", csiSnapshotContentResource: "VolumeSnapshotContentList", dataSourceResource: "DataSourceList"})
	resources := &RestoreResourceExecutor{Store: f.store, Core: core, Dynamic: dyn, Certification: model.e.Resources.Certification}
	source := &engineTestPreparation{value: model.e.Source.(*engineTestPreparation).value}
	e := &RestoreEngine{Store: f.store, Resources: resources, Resolver: NewResolver(f.store, core, resources.Dynamic), Source: source}
	return f, e, source
}

// capability_id: rainbond.vm-restore.bounded-execution
func TestPrepareMaterializesActualCompleteBAndRejectsOmittedSelectedDataDisk(t *testing.T) {
	for _, broken := range []bool{false, true} {
		f, e, source := enginePrepareFixture(t)
		if broken {
			source.value.Frame.Disks = source.value.Frame.Disks[:1]
			source.value.Frame.FrameHash, _ = RestoreResourceFrameHash(source.value.Frame)
		}
		c, err := e.Prepare(context.Background(), f.scope, f.expect.OperationID, f.intent.SourceSnapshot.Name, f.intent.PowerAfter)
		if broken {
			if !errors.Is(err, ErrConflict) {
				t.Fatal("omitted data disk accepted", err)
			}
			c, _ = f.store.Read(f.scope, f.expect.OperationID)
			if c.State.Fence != nil || c.Receipt != nil {
				t.Fatal("invalid plan acquired mutation authority")
			}
			continue
		}
		if err != nil || c.Receipt.Execution == nil || len(c.Receipt.OldBindings) != 3 {
			t.Fatal("prepare lost complete actual B set", err)
		}
		revision := c.State.ControlRevision
		if c, err = e.Prepare(context.Background(), f.scope, f.expect.OperationID, f.intent.SourceSnapshot.Name, f.intent.PowerAfter); err != nil || c.State.ControlRevision != revision {
			t.Fatal("prepare replay changed selected identities", err)
		}
	}
}

// capability_id: rainbond.vm-restore.bounded-execution
func TestPrepareUnqualifiedSourceOrUnprovenStartupDoesNotAcquireFence(t *testing.T) {
	for _, kind := range []string{"unavailable", "startup", "config"} {
		f, e, source := enginePrepareFixture(t)
		switch kind {
		case "unavailable":
			e.Source = UnqualifiedRestorePreparation{}
		case "startup":
			source.value.SourceStartup.ProducerUID = ""
		case "config":
			source.value.Config.Hardware.MemoryMiB++
		}
		if _, err := e.Prepare(context.Background(), f.scope, f.expect.OperationID, f.intent.SourceSnapshot.Name, f.intent.PowerAfter); err == nil {
			t.Fatal("unqualified preparation accepted", kind)
		}
		c, _ := f.store.Read(f.scope, f.expect.OperationID)
		if c.State.Fence != nil || c.Receipt != nil {
			t.Fatal("failed qualification acquired fence", kind)
		}
	}
}

package handler

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/goodrain/rainbond/pkg/vmrestore/copyio"
	"github.com/goodrain/rainbond/pkg/vmrestore/restoreplan"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"kubevirt.io/client-go/kubecli"
)

type restoreProviderFixture struct {
	calls        int
	dependencies vmrestore.RestoreStorageDependencies
	err          error
}

func (p *restoreProviderFixture) Load(context.Context, vmrestore.Scope) (vmrestore.RestoreStorageDependencies, error) {
	p.calls++
	return p.dependencies, p.err
}

type providerCapacityFixture struct{}

func (providerCapacityFixture) Reserve(context.Context, vmrestore.Scope, string, vmrestore.RestoreResourcePlan, copyio.Report) (restoreplan.Capability, restoreplan.SpaceEvidence, error) {
	return restoreplan.Capability{}, restoreplan.SpaceEvidence{}, vmrestore.ErrRestoreCapability
}
func (providerCapacityFixture) Check(context.Context, vmrestore.Scope, string, restoreplan.Budget) error {
	return vmrestore.ErrRestoreCapability
}
func (providerCapacityFixture) Release(context.Context, vmrestore.Scope, string) error { return nil }

// capability_id: rainbond.vm-restore.provider-entrypoint
func TestRestoreReadsDoNotLoadStorageOrRequireNativeConnectivity(t *testing.T) {
	a, _, scope := restoreFacadeFixture(t)
	a.restoreEngineFactory = nil
	provider := &restoreProviderFixture{err: errors.New("fixture unavailable provider")}
	a.restoreStorageProvider = provider
	if _, err := a.GetVMRestoreControl(context.Background(), scope.ServiceID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.GetVMRestore(context.Background(), scope.ServiceID, "restore-op"); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 0 {
		t.Fatal("read endpoint consulted an execution dependency")
	}
}

// capability_id: rainbond.vm-restore.provider-entrypoint
func TestActualRestoreCommandUsesNativeSourceOnlyWithCompleteServerDependencies(t *testing.T) {
	for _, qualified := range []bool{false, true} {
		a, _, database, scope := snapshotFacadeFixture(t)
		a.snapshotEngineFactory = nil
		ctrl := gomock.NewController(t)
		client := kubecli.NewMockKubevirtClient(ctrl)
		dyn := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
		client.EXPECT().DynamicClient().Return(dyn).AnyTimes()
		a.kubevirtClient = client
		a.kubeClient = kubefake.NewSimpleClientset()
		provider := &restoreProviderFixture{}
		if qualified {
			// These are synthetic server-owned test inputs, not target qualification.
			image := "registry.invalid/copy@sha256:" + strings.Repeat("a", 64)
			provider.dependencies = vmrestore.RestoreStorageDependencies{Capacity: providerCapacityFixture{}, DeadlineSeconds: 60, MaxDisks: 8, Certification: vmrestore.CertifiedRestoreStorage{Capability: restoreplan.Capability{Protocol: copyio.Protocol, WorkerImage: image, ROXVerified: true, SparsePersistenceVerified: true, StorageClassName: "base", StorageClassUID: "base-uid", Driver: "cephfs.csi.ceph.com", Mounter: "kernel", DriverVersion: "test"}, ROXClassName: "rox", ROXClassUID: "rox-uid", TargetClassName: "base", TargetClassUID: "base-uid", RuntimeImageID: "containerd://sha256:" + strings.Repeat("a", 64), DeleteReclaimVerified: true}}
		}
		a.restoreStorageProvider = provider
		engine, actual, err := a.RestoreService(context.Background(), scope.ServiceID)
		if err != nil || actual != scope {
			t.Fatal(actual, err)
		}
		_, native := engine.Source.(*vmrestore.NativeRestorePreparation)
		if native != qualified {
			t.Fatal("actual entrypoint bypassed dependency qualification")
		}
		_, err = a.VMRestoreCommand(context.Background(), scope.ServiceID, "restore-call", "prepare", vmrestore.RestoreCommand{SnapshotName: "missing-managed-source", PowerAfter: "previous"})
		if qualified && !errors.Is(err, vmrestore.ErrNotFound) {
			t.Fatal("real command did not consume the managed native source catalog", err)
		}
		if !qualified && !errors.Is(err, vmrestore.ErrRestoreCapability) {
			t.Fatal("default refusal was bypassed", err)
		}
		state, err := vmrestore.NewStore(database).Read(scope, "")
		if err != nil || state.State.Fence != nil || state.State.ControlRevision != 0 {
			t.Fatal("unavailable source acquired state", err)
		}
	}
}

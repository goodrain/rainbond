package handler

import (
	"context"

	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/goodrain/rainbond/worker/appm/conversion"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kv "kubevirt.io/api/core/v1"
	native "kubevirt.io/api/snapshot/v1beta1"
	"kubevirt.io/client-go/kubecli"
)

type kubeNativeRestoreBackend struct{ client kubecli.KubevirtClient }

func (b *kubeNativeRestoreBackend) VirtualMachine(ctx context.Context, scope vmrestore.Scope) (*kv.VirtualMachine, *kv.VirtualMachineInstance, error) {
	list, err := b.client.VirtualMachine(scope.Namespace).List(ctx, metav1.ListOptions{LabelSelector: "service_id=" + scope.ServiceID})
	if err != nil || len(list.Items) != 1 {
		return nil, nil, vmrestore.ErrReferenceUnavailable
	}
	vm := list.Items[0].DeepCopy()
	vmi, err := b.client.VirtualMachineInstance(scope.Namespace).Get(ctx, vm.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return vm, nil, nil
	}
	if err != nil {
		return nil, nil, vmrestore.ErrReferenceUnavailable
	}
	return vm, vmi, nil
}

func (b *kubeNativeRestoreBackend) Snapshot(ctx context.Context, scope vmrestore.Scope, name string) (*native.VirtualMachineSnapshot, error) {
	result, err := b.client.VirtualMachineSnapshot(scope.Namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, vmrestore.ErrNotFound
	}
	return result, err
}

func (b *kubeNativeRestoreBackend) Restore(ctx context.Context, scope vmrestore.Scope, name string) (*native.VirtualMachineRestore, error) {
	result, err := b.client.VirtualMachineRestore(scope.Namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	return result, err
}

func (b *kubeNativeRestoreBackend) CreateRestore(ctx context.Context, scope vmrestore.Scope, restore *native.VirtualMachineRestore) (*native.VirtualMachineRestore, error) {
	result, err := b.client.VirtualMachineRestore(scope.Namespace).Create(ctx, restore, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return b.client.VirtualMachineRestore(scope.Namespace).Get(ctx, restore.Name, metav1.GetOptions{})
	}
	return result, err
}

func (s *ServiceAction) nativeRestoreEngine(ctx context.Context, serviceID string) (*vmrestore.NativeRestoreEngine, vmrestore.Scope, error) {
	scope, err := s.snapshotScope(ctx, serviceID)
	if err != nil {
		return nil, scope, err
	}
	if s.kubevirtClient == nil {
		return nil, scope, vmrestore.ErrReferenceUnavailable
	}
	snapshots, snapshotScope, err := s.snapshotService(ctx, serviceID)
	if err != nil {
		return nil, scope, err
	}
	if snapshotScope != scope || snapshots.Identity == nil || snapshots.Identity.ProveStartup == nil || s.kubeClient == nil {
		return nil, scope, vmrestore.ErrRestoreCapability
	}
	manager := s.getDBManager()
	if manager == nil || manager.DB() == nil {
		return nil, scope, vmrestore.ErrReferenceUnavailable
	}
	store := vmrestore.NewStore(manager.DB())
	backend := &kubeNativeRestoreBackend{client: s.kubevirtClient}
	engine := vmrestore.NewNativeRestoreEngine(backend)
	engine.Managed = &vmrestore.ManagedNativeRestore{Store: store, Resolver: vmrestore.NewResolver(store, s.kubeClient, s.kubevirtClient.DynamicClient()), Snapshots: snapshots, Backend: backend, Core: s.kubeClient, Dynamic: s.kubevirtClient.DynamicClient(), CurrentStartup: snapshots.Identity.ProveStartup}
	engine.Managed.BuildPlatform = func(ctx context.Context, scope vmrestore.Scope) (*kv.VirtualMachine, error) {
		app, err := conversion.RenderVMForNativeRestore(manager, scope.ServiceID)
		if err != nil {
			return nil, err
		}
		return app.GetVirtualMachine(), nil
	}
	return engine, scope, nil
}

func (s *ServiceAction) StartNativeVMRestore(ctx context.Context, serviceID, snapshotName, requestID string) (vmrestore.NativeRestoreDTO, error) {
	engine, scope, err := s.nativeRestoreEngine(ctx, serviceID)
	if err != nil {
		return vmrestore.NativeRestoreDTO{}, err
	}
	return engine.Start(ctx, scope, snapshotName, requestID)
}

func (s *ServiceAction) GetNativeVMRestore(ctx context.Context, serviceID, snapshotName, requestID string) (vmrestore.NativeRestoreDTO, error) {
	engine, scope, err := s.nativeRestoreEngine(ctx, serviceID)
	if err != nil {
		return vmrestore.NativeRestoreDTO{}, err
	}
	return engine.Status(ctx, scope, snapshotName, requestID)
}

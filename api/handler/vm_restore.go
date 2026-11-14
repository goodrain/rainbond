package handler

import (
	"context"

	"github.com/goodrain/rainbond/pkg/vmrestore"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

// RestoreService uses the existing authenticated component lookup. A client
// cannot choose a namespace, physical resource or trusted provider certificate.
func (s *ServiceAction) RestoreService(ctx context.Context, serviceID string) (*vmrestore.RestoreEngine, vmrestore.Scope, error) {
	if s.restoreEngineFactory != nil {
		e, scope, err := s.restoreEngineFactory(ctx, serviceID)
		if err == nil {
			err = snapshotContextScope(ctx, serviceID, scope)
		}
		if err == nil && e == nil {
			err = vmrestore.ErrReferenceUnavailable
		}
		return e, scope, err
	}
	scope, err := s.snapshotScope(ctx, serviceID)
	if err != nil {
		return nil, scope, err
	}
	manager := s.getDBManager()
	if manager.DB() == nil {
		return nil, scope, vmrestore.ErrReferenceUnavailable
	}
	store := vmrestore.NewStore(manager.DB())
	if s.kubeClient == nil || s.kubevirtClient == nil {
		return &vmrestore.RestoreEngine{Store: store}, scope, nil
	}
	resolver := vmrestore.NewResolver(store, s.kubeClient, s.kubevirtClient.DynamicClient())
	runtime := &vmrestore.KubeRestoreRuntime{Store: store, Resolver: resolver, Core: s.kubeClient, Dynamic: s.kubevirtClient.DynamicClient(), BuildPlatform: func(ctx context.Context, scope vmrestore.Scope) (*kubevirtv1.VirtualMachine, error) {
		return s.buildDesiredVirtualMachine(scope.ServiceID)
	}}
	engine := &vmrestore.RestoreEngine{Store: store, Resolver: resolver, Runtime: runtime, Resources: &vmrestore.RestoreResourceExecutor{Store: store, Core: s.kubeClient, Dynamic: s.kubevirtClient.DynamicClient()}, Capture: &vmrestore.RestoreCapture{Store: store, Core: s.kubeClient, Logs: vmrestore.NewRestoreLogReader(s.kubeClient)}}
	// This boundary is intentionally unavailable until the server-owned space
	// certificate/metrics and native startup provenance producers are configured.
	// No request can turn a client hash or success flag into that qualification.
	engine.Source = vmrestore.UnqualifiedRestorePreparation{}
	engine.WireRestoreResourceGuard()
	return engine, scope, nil
}

// GetVMRestore reads persisted Region facts and never progresses the operation.
func (s *ServiceAction) GetVMRestore(ctx context.Context, serviceID, op string) (vmrestore.RestoreOperationDTO, error) {
	e, scope, err := s.RestoreService(ctx, serviceID)
	if err != nil {
		return vmrestore.RestoreOperationDTO{}, err
	}
	return e.Status(scope, op)
}

// GetVMRestoreControl reads the current component CAS before prepare.
func (s *ServiceAction) GetVMRestoreControl(ctx context.Context, serviceID string) (vmrestore.RestoreControlDTO, error) {
	e, scope, err := s.RestoreService(ctx, serviceID)
	if err != nil {
		return vmrestore.RestoreControlDTO{}, err
	}
	return e.Control(scope)
}

// VMRestoreCommand dispatches only fixed bounded backend actions.
func (s *ServiceAction) VMRestoreCommand(ctx context.Context, serviceID, op, action string, request vmrestore.RestoreCommand) (vmrestore.RestoreOperationDTO, error) {
	e, scope, err := s.RestoreService(ctx, serviceID)
	if err != nil {
		return vmrestore.RestoreOperationDTO{}, err
	}
	return e.Command(ctx, scope, op, action, request)
}

package handler

import (
	"context"
	"errors"

	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/jinzhu/gorm"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

func (s *ServiceAction) snapshotService(ctx context.Context, serviceID string) (*vmrestore.SnapshotEngine, vmrestore.Scope, error) {
	if serviceID == "" {
		return nil, vmrestore.Scope{}, vmrestore.ErrInternalAttribute
	}
	if s.snapshotEngineFactory != nil {
		engine, scope, err := s.snapshotEngineFactory(serviceID)
		if err == nil {
			err = snapshotContextScope(ctx, serviceID, scope)
		}
		if err == nil && engine == nil {
			err = vmrestore.ErrReferenceUnavailable
		}
		return engine, scope, err
	}
	scope, err := s.snapshotScope(ctx, serviceID)
	if err != nil {
		return nil, scope, err
	}
	manager := s.getDBManager()
	if s.kubevirtClient == nil || s.kubeClient == nil || manager.DB() == nil {
		return nil, scope, vmrestore.ErrReferenceUnavailable
	}
	store := vmrestore.NewStore(manager.DB())
	dynamicClient := s.kubevirtClient.DynamicClient()
	resolver := vmrestore.NewResolver(store, s.kubeClient, dynamicClient)
	backend := vmrestore.NewKubeSnapshotBackend(resolver, s.kubevirtClient)
	engine := vmrestore.NewSnapshotEngine(store, backend)
	engine.Identity = &vmrestore.SnapshotFirmwareMaterializer{Store: store, Dynamic: dynamicClient, ProveStartup: func(ctx context.Context, scope vmrestore.Scope, vm *kubevirtv1.VirtualMachine, firmware string) (vmrestore.RestoreStartupPolicy, error) {
		selection, err := store.ReadRestoredRuntimeSelection(ctx, resolver, scope)
		if err != nil {
			return vmrestore.RestoreStartupPolicy{}, err
		}
		if selection != nil {
			if selection.Startup.FirmwareUUID != firmware {
				return vmrestore.RestoreStartupPolicy{}, vmrestore.ErrConflict
			}
			wanted, err := vmrestore.ProjectRestoredRuntime(ctx, s.kubeClient, vm, *selection)
			if err != nil {
				return vmrestore.RestoreStartupPolicy{}, err
			}
			if !vmrestore.RuntimeVMMatches(vm, wanted) {
				return vmrestore.RestoreStartupPolicy{}, vmrestore.ErrConflict
			}
			return selection.Startup, nil
		}
		return store.ReadVMStartup(ctx, s.kubeClient, scope, vm)
	}}
	return engine, scope, nil
}

// snapshotScope resolves ownership without depending on the Kubernetes runtime.
func (s *ServiceAction) snapshotScope(ctx context.Context, serviceID string) (vmrestore.Scope, error) {
	scope := vmrestore.Scope{}
	manager := s.getDBManager()
	if manager == nil {
		return scope, vmrestore.ErrReferenceUnavailable
	}
	service, err := manager.TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = vmrestore.ErrNotFound
		}
		return scope, err
	}
	if service == nil || service.ServiceID != serviceID || !service.IsVM() {
		return scope, vmrestore.ErrNotFound
	}
	if err := snapshotContextScope(ctx, serviceID, vmrestore.Scope{TenantID: service.TenantID, ServiceID: service.ServiceID}); err != nil {
		return scope, err
	}
	tenant, err := manager.TenantDao().GetTenantByUUID(service.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = vmrestore.ErrNotFound
		}
		return scope, err
	}
	if tenant == nil || tenant.UUID != service.TenantID || tenant.Namespace == "" {
		return scope, vmrestore.ErrCorrupt
	}
	scope = vmrestore.Scope{TenantID: service.TenantID, ServiceID: service.ServiceID, Namespace: tenant.Namespace}
	return scope, nil
}

// PrepareVMSnapshot reserves a bounded operation even when the VM is Running.
func (s *ServiceAction) PrepareVMSnapshot(ctx context.Context, serviceID string, request *VMSnapshotRequest) (*VMSnapshotStatus, error) {
	if request == nil {
		return nil, vmrestore.ErrSnapshotInvalid
	}
	engine, scope, err := s.snapshotService(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	status, err := engine.Prepare(ctx, scope, *request)
	if err != nil {
		return nil, err
	}
	return &VMSnapshotStatus{SnapshotName: status.SnapshotName, SnapshotOperationDTO: &status}, nil
}

// ListVMSnapshots returns only safe managed Region DTOs.
func (s *ServiceAction) ListVMSnapshots(ctx context.Context, serviceID string, page, pageSize int) (vmrestore.SnapshotListDTO, error) {
	engine, scope, err := s.snapshotService(ctx, serviceID)
	if err != nil {
		return vmrestore.SnapshotListDTO{}, err
	}
	return engine.List(ctx, scope, page, pageSize)
}

// GetVMSnapshot never ACKs, starts a VM, releases a lock or performs cleanup.
func (s *ServiceAction) GetVMSnapshot(ctx context.Context, serviceID, name string) (vmrestore.SnapshotOperationDTO, error) {
	engine, scope, err := s.snapshotService(ctx, serviceID)
	if err != nil {
		return vmrestore.SnapshotOperationDTO{}, err
	}
	return engine.ByName(ctx, scope, name)
}

// VMSnapshotAction dispatches only explicit bounded commands, not arbitrary patches.
func (s *ServiceAction) VMSnapshotAction(ctx context.Context, serviceID, name, action string, request vmrestore.SnapshotActionRequest) (vmrestore.SnapshotOperationDTO, error) {
	engine, scope, err := s.snapshotService(ctx, serviceID)
	if err != nil {
		return vmrestore.SnapshotOperationDTO{}, err
	}
	current, err := engine.ByName(ctx, scope, name)
	if err != nil {
		return vmrestore.SnapshotOperationDTO{}, err
	}
	if request.OperationID != "" && request.OperationID != current.OperationID || request.CoreHash != "" && request.CoreHash != current.CoreHash {
		return vmrestore.SnapshotOperationDTO{}, vmrestore.ErrConflict
	}
	request.OperationID = current.OperationID
	request.CoreHash = current.CoreHash
	switch action {
	case "halt":
		return engine.Halt(ctx, scope, request)
	case "create":
		return engine.Create(ctx, scope, request)
	case "resume":
		return engine.Resume(ctx, scope, request)
	case "finalize":
		return engine.Finalize(ctx, scope, request)
	case "cancel":
		return engine.Cancel(ctx, scope, request)
	case "delete":
		return engine.Delete(ctx, scope, request)
	default:
		return vmrestore.SnapshotOperationDTO{}, vmrestore.ErrSnapshotInvalid
	}
}

// snapshotContextScope rechecks the middleware tenant against DB-owned service identity.
// Legacy internal callers supply no HTTP context and still resolve their scope from DB.
func snapshotContextScope(ctx context.Context, serviceID string, scope vmrestore.Scope) error {
	if scope.ServiceID != serviceID || scope.TenantID == "" {
		return vmrestore.ErrInternalAttribute
	}
	if value := ctx.Value(ctxutil.ContextKey("tenant_id")); value != nil {
		tenantID, ok := value.(string)
		if !ok || tenantID == "" || tenantID != scope.TenantID {
			return vmrestore.ErrInternalAttribute
		}
	}
	return nil
}

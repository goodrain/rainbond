package conversion

import (
	configs "github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
)

// RenderVMForRestore builds only the selected VM configuration. It does not
// register gateways, reconcile routes, apply plugins or publish any resource.
func RenderVMForRestore(manager db.Manager, serviceID string) (*v1.AppService, error) {
	return renderVMForRestore(manager, serviceID, true)
}

// RenderVMForNativeRestore compares configured fields without resolving physical disk bindings.
func RenderVMForNativeRestore(manager db.Manager, serviceID string) (*v1.AppService, error) {
	return renderVMForRestore(manager, serviceID, false)
}

func renderVMForRestore(manager db.Manager, serviceID string, resolveBindings bool) (*v1.AppService, error) {
	as, err := InitCacheAppService(manager, serviceID, "")
	if err != nil {
		return nil, err
	}
	as.RestoreRender = true
	as.SharedStorageClass = configs.Default().WorkerConfig.SharedStorageClass
	if as.GetVirtualMachine() == nil {
		return nil, vmrestore.ErrRestoreCapability
	}
	if err := TenantServiceConfigGroup(as, manager); err != nil {
		return nil, err
	}
	if err := tenantServiceVersion(as, manager, resolveBindings); err != nil {
		return nil, err
	}
	return as, nil
}

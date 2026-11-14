package controller

import (
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
)

// Legacy queued controllers cannot override a restore or destructively recreate
// a VM whose physical disks have entered the shared binding authority. Ordinary
// power for these VMs is handled by the admitted API path and VMI observer.
func guardLegacyVMLifecycle(app v1.AppService) error {
	if app.GetVirtualMachine() == nil {
		return nil
	}
	manager := db.GetManager()
	if manager == nil || manager.DB() == nil {
		return nil
	}
	c, err := vmrestore.NewStore(manager.DB()).Read(vmrestore.Scope{TenantID: app.TenantID, ServiceID: app.ServiceID, Namespace: app.GetNamespace()}, "")
	if err != nil {
		return err
	}
	if c.State.Fence != nil || c.State.Admission != nil {
		return vmrestore.ErrBusy
	}
	if len(c.State.Bindings) != 0 || c.State.StartupOperationID != "" {
		return vmrestore.ErrConflict
	}
	return nil
}

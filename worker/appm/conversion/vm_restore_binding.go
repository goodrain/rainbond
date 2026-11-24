package conversion

import (
	"context"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
)

func loadVMRestoreBindingView(as *v1.AppService, manager db.Manager) error {
	if as.GetVirtualMachine() == nil || manager == nil || manager.DB() == nil {
		return nil
	}
	scope := vmrestore.Scope{TenantID: as.TenantID, ServiceID: as.ServiceID, Namespace: as.GetNamespace()}
	store := vmrestore.NewStore(manager.DB())
	state, err := store.Read(scope, "")
	if err != nil {
		return err
	}
	if len(state.State.Bindings) == 0 {
		return nil
	}
	clients := k8s.Default()
	if clients == nil || clients.Clientset == nil || clients.DynamicClient == nil {
		return vmrestore.ErrReferenceUnavailable
	}
	resolver := vmrestore.NewResolver(store, clients.Clientset, clients.DynamicClient)
	view, err := resolver.ReadBindingView(context.Background(), scope)
	if err != nil {
		return err
	}
	as.VMStorageView = &view
	selection, err := store.ReadRestoredRuntimeSelection(context.Background(), resolver, scope)
	if err != nil {
		return err
	}
	as.VMRestoredRuntime = selection
	return nil
}

func vmRuntimeExtensionsForRestoredDisk(extension map[string]string) map[string]string {
	result := map[string]string{}
	for k, v := range extension {
		result[k] = v
	}
	for _, k := range []string{"vm_ssh_username", "vm_ssh_authorized_key", "vm_login_password_hash", "vm_login_password"} {
		delete(result, k)
	}
	return result
}

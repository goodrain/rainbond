package conversion

import (
	configs "github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
)

// RenderVMForRestore builds only the selected VM configuration. It does not
// register gateways, reconcile routes, apply plugins or publish any resource.
func RenderVMForRestore(manager db.Manager, serviceID string) (*v1.AppService, error) {
	as := &v1.AppService{AppServiceBase: v1.AppServiceBase{ServiceID: serviceID, ExtensionSet: map[string]string{}, RestoreRender: true, GovernanceMode: dbmodel.GovernanceModeBuildInServiceMesh, SharedStorageClass: configs.Default().WorkerConfig.SharedStorageClass}, UpgradePatch: map[string][]byte{}}
	if err := TenantServiceBase(as, manager); err != nil {
		return nil, err
	}
	if as.GetVirtualMachine() == nil {
		return nil, vmrestore.ErrRestoreCapability
	}
	if err := TenantServiceConfigGroup(as, manager); err != nil {
		return nil, err
	}
	if err := TenantServiceVersion(as, manager); err != nil {
		return nil, err
	}
	return as, nil
}

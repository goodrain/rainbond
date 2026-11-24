package controller

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	kv "kubevirt.io/api/core/v1"
)

type restoreGuardDB struct {
	db.Manager
	database *gorm.DB
}

func (m restoreGuardDB) DB() *gorm.DB { return m.database }

// capability_id: rainbond.vm-restore.stale-worker-lifecycle
func TestQueuedVMLifecycleCannotBypassLiveFenceAdmissionOrMappedDisks(t *testing.T) {
	for _, caseName := range []string{"fence", "admission", "mapped"} {
		t.Run(caseName, func(t *testing.T) {
			database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "guard.db"))
			if err != nil {
				t.Fatal(err)
			}
			database.LogMode(false)
			defer database.Close()
			if err := database.AutoMigrate(&dbmodel.Tenants{}, &dbmodel.TenantServices{}, &dbmodel.ComponentK8sAttributes{}).Error; err != nil {
				t.Fatal(err)
			}
			scope := vmrestore.Scope{TenantID: "tenant", ServiceID: "service", Namespace: "team"}
			if err := database.Create(&dbmodel.Tenants{UUID: scope.TenantID, Namespace: scope.Namespace}).Error; err != nil {
				t.Fatal(err)
			}
			if err := database.Create(&dbmodel.TenantServices{TenantID: scope.TenantID, ServiceID: scope.ServiceID, ExtendMethod: string(dbmodel.ServiceTypeVM)}).Error; err != nil {
				t.Fatal(err)
			}
			previous := db.GetManager()
			db.SetTestManager(restoreGuardDB{database: database})
			defer db.SetTestManager(previous)
			store := vmrestore.NewStore(database)
			hash := strings.Repeat("a", 64)
			_, err = store.Mutate(scope, vmrestore.Expectation{OperationID: "owned-operation", ManifestHash: hash, RequestHash: hash, Step: "prepare"}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
				r := vmrestore.NewReceipt(scope, "owned-operation", hash)
				switch caseName {
				case "fence":
					c.State.Fence = &vmrestore.Fence{Kind: "restore", Phase: "prepared", OperationID: "owned-operation", ManifestHash: hash, TargetVMUID: "vm-uid"}
				case "admission":
					if err := c.State.AdmitMutation("owned-operation", "ordinary-vm-start"); err != nil {
						return vmrestore.Mutation{}, err
					}
				case "mapped":
					c.State.Bindings["7"] = vmrestore.Binding{VolumeID: 7, DiskKey: "disk", CanonicalName: "manual7", Role: "root", Attached: true, ActivePVC: vmrestore.ObjectRef{Kind: "PersistentVolumeClaim", Namespace: scope.Namespace, Name: "active-root", UID: "root-uid"}}
					c.State.BindingGeneration++
				}
				return vmrestore.Mutation{State: c.State, Receipt: &r}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			app := v1.AppService{AppServiceBase: v1.AppServiceBase{TenantID: scope.TenantID, ServiceID: scope.ServiceID}}
			app.SetTenant(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: scope.Namespace}})
			app.SetVirtualMachine(&kv.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "vm", Namespace: scope.Namespace}, Spec: kv.VirtualMachineSpec{Template: &kv.VirtualMachineInstanceTemplateSpec{}}})
			client := kubefake.NewSimpleClientset()
			manager := &Manager{client: client}
			start := &startController{manager: manager, ctx: context.Background()}
			stop := &stopController{manager: manager, ctx: context.Background()}
			for _, err := range []error{start.startOne(app), stop.stopOne(app)} {
				if !errors.Is(err, vmrestore.ErrBusy) && !errors.Is(err, vmrestore.ErrConflict) {
					t.Fatal("obsolete controller was not rejected", err)
				}
			}
			if len(client.Actions()) != 0 {
				t.Fatal("obsolete controller mutated cluster resources before checking the ledger")
			}
		})
	}
}

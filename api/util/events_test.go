package util

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	mysqldao "github.com/goodrain/rainbond/db/mysql/dao"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

type eventScopeManager struct {
	db.Manager
	database *gorm.DB
}

func (manager eventScopeManager) ServiceEventDao() dbdao.EventDao {
	return &mysqldao.EventDaoImpl{DB: manager.database}
}

// capability_id: rainbond.vm-power.scoped-single-event
func TestCreateEventScopesRestoredVMPower(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.AutoMigrate(&dbmodel.ServiceEvent{}, &dbmodel.ComponentK8sAttributes{}, &dbmodel.TenantServices{}, &dbmodel.Tenants{}).Error; err != nil {
		t.Fatal(err)
	}
	previous := db.GetManager()
	db.SetTestManager(eventScopeManager{database: database})
	t.Cleanup(func() { db.SetTestManager(previous) })
	scope := vmrestore.Scope{TenantID: "tenant", ServiceID: "service", Namespace: "team"}
	for _, model := range []interface{}{
		&dbmodel.Tenants{UUID: scope.TenantID, Namespace: scope.Namespace},
		&dbmodel.TenantServices{ServiceID: scope.ServiceID, TenantID: scope.TenantID, Namespace: scope.Namespace, ExtendMethod: string(dbmodel.ServiceTypeVM)},
		&dbmodel.TenantServices{ServiceID: "other-service", TenantID: scope.TenantID, Namespace: scope.Namespace, ExtendMethod: string(dbmodel.ServiceTypeVM)},
	} {
		if err := database.Create(model).Error; err != nil {
			t.Fatal(err)
		}
	}
	event, err := CreateEvent("service", "start-service", scope.ServiceID, scope.TenantID, "{}", "tester", "", "", dbmodel.ASYNEVENTTYPE)
	if err != nil {
		t.Fatal(err)
	}
	if event.ServiceID != scope.ServiceID || event.Target != "service" || event.TargetID != scope.ServiceID || event.TenantID != scope.TenantID {
		t.Fatal("component power event lost its service scope")
	}
	store := vmrestore.NewStore(database)
	vm := vmrestore.ObjectRef{Kind: "VirtualMachine", Namespace: scope.Namespace, Name: "vm", UID: "vm-uid"}
	foreign := scope
	foreign.ServiceID = "other-service"
	if _, _, err := store.ClaimOrdinaryVMWithEvent(foreign, "start", vm, event.EventID); !errors.Is(err, vmrestore.ErrConflict) {
		t.Fatal("another component claimed the power event", err)
	}
	if _, _, err := store.ClaimOrdinaryVMWithEvent(scope, "start", vm, event.EventID); err != nil {
		t.Fatal("a scoped component event cannot authorize restored VM startup", err)
	}
	other, err := CreateEvent("tenant", "update-tenant", "tenant", scope.TenantID, "{}", "tester", "", "", dbmodel.ASYNEVENTTYPE)
	if err != nil || other.ServiceID != "" {
		t.Fatal("non-component event acquired a service scope", err)
	}
}

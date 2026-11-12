package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	mysqldao "github.com/goodrain/rainbond/db/mysql/dao"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

type restoreMissingDeleteServiceDAO struct {
	dbdao.TenantServiceDao
	calls int
}

func (d *restoreMissingDeleteServiceDAO) GetServiceByID(string) (*dbmodel.TenantServices, error) {
	d.calls++
	return nil, errors.New("unexpected_runtime_sync_fixed_code")
}

type restoreMissingDeleteManager struct {
	db.Manager
	database *gorm.DB
	services *restoreMissingDeleteServiceDAO
}

func (m restoreMissingDeleteManager) ComponentK8sAttributeDao() dbdao.ComponentK8sAttributeDao {
	return &mysqldao.ComponentK8sAttributeDaoImpl{DB: m.database}
}
func (m restoreMissingDeleteManager) TenantServiceDao() dbdao.TenantServiceDao {
	return m.services
}

// capability_id: rainbond.vm-restore.public-attribute-delete-retry
func TestDeleteMissingPrivateAttributesReturnsForbiddenWithoutRuntimeSync(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "attributes.db"))
	if err != nil {
		t.Fatal("attribute test database could not open")
	}
	t.Cleanup(func() { database.Close() })
	database.LogMode(false)
	if err := database.AutoMigrate(&dbmodel.ComponentK8sAttributes{}).Error; err != nil {
		t.Fatal("attribute test schema could not initialize")
	}
	services := &restoreMissingDeleteServiceDAO{}
	previous := db.GetManager()
	db.SetTestManager(restoreMissingDeleteManager{database: database, services: services})
	t.Cleanup(func() { db.SetTestManager(previous) })
	controller := &K8sAttributeController{serviceHandler: &handler.ServiceAction{}}
	for _, name := range []string{vmrestore.StateKey, " VM_RESTORE_STATE ", vmrestore.ReceiptPrefix + "operation-a", vmrestore.LegacyBindingsKey} {
		w := httptest.NewRecorder()
		controller.K8sAttributes(w, restoreAttributeHTTPRequest(http.MethodDelete, name))
		if w.Code != http.StatusForbidden {
			t.Fatal("missing private attribute deletion did not return forbidden")
		}
	}
	w := httptest.NewRecorder()
	controller.K8sAttributes(w, restoreAttributeHTTPRequest(http.MethodDelete, "labels"))
	if w.Code != http.StatusOK {
		t.Fatal("missing ordinary attribute deletion stopped being idempotent")
	}
	if services.calls != 0 {
		t.Fatal("missing private or ordinary attribute HTTP deletion triggered VM synchronization")
	}
}

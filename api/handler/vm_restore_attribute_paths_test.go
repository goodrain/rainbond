package handler

import (
	"errors"
	"path/filepath"
	"testing"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	mysqldao "github.com/goodrain/rainbond/db/mysql/dao"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

type restoreAttributePathManager struct {
	db.Manager
	database *gorm.DB
}

func (m restoreAttributePathManager) DB() *gorm.DB { return m.database }
func (m restoreAttributePathManager) ComponentK8sAttributeDao() dbdao.ComponentK8sAttributeDao {
	return &mysqldao.ComponentK8sAttributeDaoImpl{DB: m.database}
}
func (m restoreAttributePathManager) ComponentK8sAttributeDaoTransactions(tx *gorm.DB) dbdao.ComponentK8sAttributeDao {
	return &mysqldao.ComponentK8sAttributeDaoImpl{DB: tx}
}

func restoreAttributePathFixture(t *testing.T) (*ServiceAction, *gorm.DB) {
	t.Helper()
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "attributes.db"))
	if err != nil {
		t.Fatal("attribute test database could not open")
	}
	database.LogMode(false)
	if err := database.AutoMigrate(&dbmodel.ComponentK8sAttributes{}).Error; err != nil {
		t.Fatal("attribute test schema could not initialize")
	}
	manager := restoreAttributePathManager{database: database}
	previous := db.GetManager()
	db.SetTestManager(manager)
	t.Cleanup(func() { db.SetTestManager(previous); database.Close() })
	return &ServiceAction{dbmanager: manager}, database
}

func insertRestoreAttribute(t *testing.T, database *gorm.DB, name, componentID string) *dbmodel.ComponentK8sAttributes {
	t.Helper()
	row := &dbmodel.ComponentK8sAttributes{TenantID: "tenant", ComponentID: componentID, Name: name, SaveType: "string", AttributeValue: "ATTRIBUTE_SENTINEL"}
	if err := database.Create(row).Error; err != nil {
		t.Fatal("attribute fixture could not persist")
	}
	return row
}

// capability_id: rainbond.vm-restore.public-attribute-paths
func TestSyncComponentK8sAttributesPreservesPrivateRows(t *testing.T) {
	for _, commit := range []bool{false, true} {
		t.Run(map[bool]string{false: "rollback", true: "commit"}[commit], func(t *testing.T) {
			action, database := restoreAttributePathFixture(t)
			private := []*dbmodel.ComponentK8sAttributes{}
			for _, name := range []string{vmrestore.StateKey, vmrestore.ReceiptPrefix + "operation-a", "VM_VOLUME_BINDINGS "} {
				private = append(private, insertRestoreAttribute(t, database, name, "component"))
			}
			old := insertRestoreAttribute(t, database, "vm_boot_mode", "component")
			other := insertRestoreAttribute(t, database, "vm_boot_mode", "other-component")
			tx := database.Begin()
			defer tx.Rollback()
			component := &apimodel.Component{ComponentBase: apimodel.ComponentBase{ComponentID: "component"}, ComponentK8sAttributes: []apimodel.ComponentK8sAttribute{{Name: "vm_boot_mode", SaveType: "string", AttributeValue: "uefi"}}}
			if err := action.SyncComponentK8sAttributes(tx, &dbmodel.Application{TenantID: "tenant"}, []*apimodel.Component{component}); err != nil {
				t.Fatal("public component replacement failed")
			}
			for _, row := range private {
				var current dbmodel.ComponentK8sAttributes
				if err := tx.Where("ID = ?", row.ID).Take(&current).Error; err != nil || current.AttributeValue != row.AttributeValue {
					t.Fatal("actual component replacement changed an internal row")
				}
			}
			if commit {
				if err := tx.Commit().Error; err != nil {
					t.Fatal("replacement commit failed")
				}
			} else if err := tx.Rollback().Error; err != nil {
				t.Fatal("replacement rollback failed")
			}
			var current dbmodel.ComponentK8sAttributes
			if err := database.Where("component_id = ? AND name = ?", "component", old.Name).Take(&current).Error; err != nil {
				t.Fatal("public attribute disappeared")
			}
			if commit && (current.ID == old.ID || current.AttributeValue != "uefi" || current.TenantID != "tenant") {
				t.Fatal("public replacement did not derive its target identity")
			}
			if !commit && (current.ID != old.ID || current.AttributeValue != old.AttributeValue) {
				t.Fatal("public replacement ignored caller rollback")
			}
			current = dbmodel.ComponentK8sAttributes{}
			if err := database.Where("ID = ?", other.ID).Take(&current).Error; err != nil || current.AttributeValue != other.AttributeValue {
				t.Fatal("replacement changed another component")
			}
		})
	}
}

func TestSyncComponentK8sAttributesRejectsPrivateInput(t *testing.T) {
	action, database := restoreAttributePathFixture(t)
	row := insertRestoreAttribute(t, database, "vm_boot_mode", "component")
	tx := database.Begin()
	defer tx.Rollback()
	component := &apimodel.Component{ComponentBase: apimodel.ComponentBase{ComponentID: "component"}, ComponentK8sAttributes: []apimodel.ComponentK8sAttribute{{Name: vmrestore.StateKey}}}
	if err := action.SyncComponentK8sAttributes(tx, &dbmodel.Application{TenantID: "tenant"}, []*apimodel.Component{component}); !errors.Is(err, vmrestore.ErrInternalAttribute) {
		t.Fatal("actual component replacement accepted an internal input")
	}
	var current dbmodel.ComponentK8sAttributes
	if err := tx.Where("ID = ?", row.ID).Take(&current).Error; err != nil || current.AttributeValue != row.AttributeValue {
		t.Fatal("denied replacement changed public attributes")
	}
}

func TestResourceImportK8sAttributesRejectsSourceIdentity(t *testing.T) {
	_, database := restoreAttributePathFixture(t)
	protected := insertRestoreAttribute(t, database, vmrestore.StateKey, "source-component")
	public := &dbmodel.ComponentK8sAttributes{Model: dbmodel.Model{ID: protected.ID}, TenantID: "source-tenant", ComponentID: "source-component", Name: "vm_boot_mode", AttributeValue: "bios"}
	rows := []*dbmodel.ComponentK8sAttributes{public, {Name: vmrestore.StateKey, AttributeValue: "PRIVATE_SENTINEL"}, {Name: vmrestore.ReceiptPrefix + "operation-a"}, {Name: vmrestore.LegacyBindingsKey}}
	result := (&clusterAction{}).createK8sAttributes(rows, "target-tenant", &dbmodel.TenantServices{ServiceID: "target-component"})
	if len(result) != 1 || result[0].Name != public.Name || result[0].TenantID != "target-tenant" || result[0].ComponentID != "target-component" {
		t.Fatal("actual import returned unfiltered source metadata")
	}
	if public.ID != protected.ID || public.TenantID != "source-tenant" || public.ComponentID != "source-component" {
		t.Fatal("actual import mutated the source object")
	}
	var current dbmodel.ComponentK8sAttributes
	if err := database.Where("ID = ?", protected.ID).Take(&current).Error; err != nil || current.Name != protected.Name || current.ComponentID != protected.ComponentID {
		t.Fatal("actual import overwrote the row selected by an incoming primary key")
	}
	var imported []dbmodel.ComponentK8sAttributes
	if err := database.Where("component_id = ?", "target-component").Find(&imported).Error; err != nil || len(imported) != 1 {
		t.Fatal("actual import persisted private state or lost public fields")
	}
	if imported[0].ID == protected.ID || imported[0].TenantID != "target-tenant" || imported[0].Name != public.Name || imported[0].AttributeValue != public.AttributeValue {
		t.Fatal("actual import did not derive public attribute identity")
	}
}

func TestPublicK8sAttributeCRUDPreservesOrdinaryComponentBehavior(t *testing.T) {
	action, database := restoreAttributePathFixture(t)
	private := insertRestoreAttribute(t, database, vmrestore.StateKey, "component")
	other := insertRestoreAttribute(t, database, "labels", "other-component")
	if err := action.CreateK8sAttribute("tenant", "component", &apimodel.ComponentK8sAttribute{Name: "labels", SaveType: "string", AttributeValue: "before"}); err != nil {
		t.Fatal("ordinary attribute create failed")
	}
	row, err := action.GetK8sAttribute("component", "labels")
	if err != nil || row == nil || row.ID == 0 || row.TenantID != "tenant" || row.ComponentID != "component" || row.AttributeValue != "before" {
		t.Fatal("ordinary attribute read lost server-derived fields")
	}
	if err := action.UpdateK8sAttribute("component", &apimodel.ComponentK8sAttribute{Name: "labels", AttributeValue: "after"}); err != nil {
		t.Fatal("ordinary attribute update failed")
	}
	row, err = action.GetK8sAttribute("component", "labels")
	if err != nil || row == nil || row.AttributeValue != "after" || row.SaveType != "string" {
		t.Fatal("ordinary update changed the omitted save type")
	}
	if err := action.DeleteK8sAttribute("component", "labels"); err != nil {
		t.Fatal("ordinary attribute delete failed")
	}
	if row, err := action.GetK8sAttribute("component", "labels"); err != nil || row != nil {
		t.Fatal("ordinary delete left the requested row")
	}
	if err := action.DeleteK8sAttribute("component", "missing-public"); err != nil {
		t.Fatal("missing public deletion stopped being idempotent")
	}
	if err := action.UpdateK8sAttribute("component", &apimodel.ComponentK8sAttribute{Name: "missing-public"}); !gorm.IsRecordNotFoundError(err) {
		t.Fatal("missing public update did not return not found")
	}
	for _, untouched := range []*dbmodel.ComponentK8sAttributes{private, other} {
		var current dbmodel.ComponentK8sAttributes
		if err := database.Where("ID = ?", untouched.ID).Take(&current).Error; err != nil || current.AttributeValue != untouched.AttributeValue {
			t.Fatal("ordinary CRUD changed an unrelated or private row")
		}
	}
}

func TestSyncComponentK8sAttributesKeepsEmptySetSemantics(t *testing.T) {
	action, database := restoreAttributePathFixture(t)
	row := insertRestoreAttribute(t, database, "labels", "component")
	for _, attributes := range [][]apimodel.ComponentK8sAttribute{nil, {}} {
		tx := database.Begin()
		component := &apimodel.Component{ComponentBase: apimodel.ComponentBase{ComponentID: "component"}, ComponentK8sAttributes: attributes}
		if err := action.SyncComponentK8sAttributes(tx, &dbmodel.Application{TenantID: "tenant"}, []*apimodel.Component{component}); err != nil {
			tx.Rollback()
			t.Fatal("empty public set was rejected")
		}
		if err := tx.Commit().Error; err != nil {
			t.Fatal("empty public set commit failed")
		}
		var current dbmodel.ComponentK8sAttributes
		if err := database.Where("ID = ?", row.ID).Take(&current).Error; err != nil || current.AttributeValue != row.AttributeValue {
			t.Fatal("empty public set deleted existing configuration")
		}
	}
}

func TestSyncComponentK8sAttributesRejectsInvalidTarget(t *testing.T) {
	action, database := restoreAttributePathFixture(t)
	for _, tc := range []struct {
		app        *dbmodel.Application
		components []*apimodel.Component
	}{
		{},
		{app: &dbmodel.Application{TenantID: "tenant"}, components: []*apimodel.Component{nil}},
		{app: &dbmodel.Application{TenantID: "tenant"}, components: []*apimodel.Component{{ComponentK8sAttributes: []apimodel.ComponentK8sAttribute{{Name: "labels"}}}}},
	} {
		tx := database.Begin()
		err := action.SyncComponentK8sAttributes(tx, tc.app, tc.components)
		tx.Rollback()
		if !errors.Is(err, vmrestore.ErrCorrupt) {
			t.Fatal("invalid public replacement target was accepted")
		}
	}
}

func TestResourceImportK8sAttributesHandlesEmptyAndFailedPersistence(t *testing.T) {
	previous := db.GetManager()
	dao := &restorePublicAttributeDAO{batchErr: errors.New("attribute_write_fixed_code")}
	db.SetTestManager(restorePublicAttributeManager{attributes: dao})
	t.Cleanup(func() { db.SetTestManager(previous) })
	action := &clusterAction{}
	component := &dbmodel.TenantServices{ServiceID: "component"}
	if action.createK8sAttributes([]*dbmodel.ComponentK8sAttributes{{Name: "labels"}}, "tenant", nil) != nil {
		t.Fatal("nil import component returned attributes")
	}
	if rows := action.createK8sAttributes(nil, "tenant", component); len(rows) != 0 || dao.writes != 0 {
		t.Fatal("empty import unexpectedly wrote attributes")
	}
	if rows := action.createK8sAttributes([]*dbmodel.ComponentK8sAttributes{nil, {Name: vmrestore.StateKey}}, "tenant", component); len(rows) != 0 || dao.writes != 0 {
		t.Fatal("private-only import unexpectedly wrote attributes")
	}
	if rows := action.createK8sAttributes([]*dbmodel.ComponentK8sAttributes{{Name: "labels"}}, "tenant", component); rows != nil || dao.writes != 1 {
		t.Fatal("failed attribute persistence returned runtime metadata")
	}
}

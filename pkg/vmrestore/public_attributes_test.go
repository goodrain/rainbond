package vmrestore

import (
	"errors"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// capability_id: rainbond.vm-restore.private-attributes
func TestPublicOverwritePreservesInternalRowsAndRejectsIDInjection(t *testing.T) {
	store, database, scope := fixtureStore(t)
	if _, err := store.Mutate(scope, expectation(), prepareState(scope, expectation())); err != nil {
		t.Fatal(err)
	}
	var internal dbmodel.ComponentK8sAttributes
	if err := database.Where("component_id = ? AND name = ?", scope.ServiceID, StateKey).Take(&internal).Error; err != nil {
		t.Fatal(err)
	}
	incoming := &dbmodel.ComponentK8sAttributes{Model: dbmodel.Model{ID: internal.ID}, TenantID: "foreign", ComponentID: "foreign", Name: "vm_boot_mode", SaveType: "string", AttributeValue: "bios"}
	tx := database.Begin()
	if err := ReplacePublicAttributesTx(tx, scope, []*dbmodel.ComponentK8sAttributes{incoming}); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatal(err)
	}
	current, err := store.Read(scope, expectation().OperationID)
	if err != nil || current.State.Fence == nil || current.State.ControlRevision != 1 {
		t.Fatal("public overwrite changed internal state")
	}
	var public dbmodel.ComponentK8sAttributes
	if err := database.Where("component_id = ? AND name = ?", scope.ServiceID, "vm_boot_mode").Take(&public).Error; err != nil {
		t.Fatal(err)
	}
	if public.ID == internal.ID || public.TenantID != scope.TenantID {
		t.Fatal("incoming database identity was reused")
	}
}

func TestExportAndImportCannotCarryBindingsOrReceipt(t *testing.T) {
	scope := Scope{TenantID: "target-tenant", ServiceID: "target-service", Namespace: "target-team"}
	rows := []*dbmodel.ComponentK8sAttributes{{Name: StateKey}, {Name: ReceiptPrefix + "operation-a"}, {Model: dbmodel.Model{ID: 44}, TenantID: "source", ComponentID: "source", Name: "vm_boot_mode", AttributeValue: "bios"}}
	copied := PublicAttributeCopies(rows, &scope)
	if len(copied) != 1 || copied[0].ID != 0 || copied[0].TenantID != scope.TenantID || copied[0].ComponentID != scope.ServiceID {
		t.Fatal("copy inherited protected state or source identity")
	}
	if rows[2].ID != 44 || rows[2].TenantID != "source" {
		t.Fatal("copy mutated source object")
	}
}

func TestPublicDeleteCannotRemoveInternalRow(t *testing.T) {
	store, database, scope := fixtureStore(t)
	if _, err := store.Mutate(scope, expectation(), prepareState(scope, expectation())); err != nil {
		t.Fatal(err)
	}
	var row dbmodel.ComponentK8sAttributes
	if err := database.Where("component_id = ? AND name = ?", scope.ServiceID, StateKey).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := DeletePublicAttribute(database, scope.ServiceID, row.ID); !errors.Is(err, ErrInternalAttribute) {
		t.Fatal("public ID deleted protected row")
	}
	if _, err := store.Read(scope, expectation().OperationID); err != nil {
		t.Fatal(err)
	}
}

func TestPublicDeleteRemovesOnlyTheObservedComponentsRow(t *testing.T) {
	store, database, scope := fixtureStore(t)
	if _, err := store.Mutate(scope, expectation(), prepareState(scope, expectation())); err != nil {
		t.Fatal(err)
	}
	rows := []dbmodel.ComponentK8sAttributes{
		{TenantID: scope.TenantID, ComponentID: scope.ServiceID, Name: "vm_boot_mode", AttributeValue: "bios"},
		{TenantID: "other-tenant", ComponentID: "other-component", Name: "vm_boot_mode", AttributeValue: "uefi"},
	}
	for i := range rows {
		if err := database.Create(&rows[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := DeletePublicAttribute(database, scope.ServiceID, rows[0].ID); err != nil {
		t.Fatal(err)
	}
	var removed dbmodel.ComponentK8sAttributes
	if err := database.Where("ID = ?", rows[0].ID).Take(&removed).Error; !gorm.IsRecordNotFoundError(err) {
		t.Fatal("successful deletion left the requested public row")
	}
	var untouched dbmodel.ComponentK8sAttributes
	if err := database.Where("ID = ?", rows[1].ID).Take(&untouched).Error; err != nil || untouched.AttributeValue != rows[1].AttributeValue {
		t.Fatal("public deletion changed another component's row")
	}
	current, err := store.Read(scope, expectation().OperationID)
	if err != nil || current.State.ControlRevision != 1 || current.State.Fence == nil {
		t.Fatal("public deletion changed the protected ledger")
	}
}

func TestPublicReplacementParticipatesInTheCallersTransaction(t *testing.T) {
	store, database, scope := fixtureStore(t)
	if _, err := store.Mutate(scope, expectation(), prepareState(scope, expectation())); err != nil {
		t.Fatal(err)
	}
	old := dbmodel.ComponentK8sAttributes{TenantID: scope.TenantID, ComponentID: scope.ServiceID, Name: "vm_boot_mode", AttributeValue: "bios"}
	other := dbmodel.ComponentK8sAttributes{TenantID: "other-tenant", ComponentID: "other-component", Name: old.Name, AttributeValue: "uefi"}
	if err := database.Create(&old).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	incoming := []*dbmodel.ComponentK8sAttributes{{Name: old.Name, AttributeValue: "uefi"}}
	tx := database.Begin()
	if err := ReplacePublicAttributesTx(tx, scope, incoming); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	var pending dbmodel.ComponentK8sAttributes
	if err := tx.Where("component_id = ? AND name = ?", scope.ServiceID, old.Name).Take(&pending).Error; err != nil || pending.AttributeValue != "uefi" {
		tx.Rollback()
		t.Fatal("replacement was not visible in its caller transaction")
	}
	if err := tx.Rollback().Error; err != nil {
		t.Fatal(err)
	}
	var restored dbmodel.ComponentK8sAttributes
	if err := database.Where("ID = ?", old.ID).Take(&restored).Error; err != nil || restored.AttributeValue != "bios" {
		t.Fatal("caller rollback did not restore the prior public row")
	}
	tx = database.Begin()
	if err := ReplacePublicAttributesTx(tx, scope, incoming); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatal(err)
	}
	var committed dbmodel.ComponentK8sAttributes
	if err := database.Where("component_id = ? AND name = ?", scope.ServiceID, old.Name).Take(&committed).Error; err != nil || committed.AttributeValue != "uefi" || committed.ID == old.ID {
		t.Fatal("caller commit did not replace the prior public row")
	}
	var untouched dbmodel.ComponentK8sAttributes
	if err := database.Where("ID = ?", other.ID).Take(&untouched).Error; err != nil || untouched.AttributeValue != other.AttributeValue {
		t.Fatal("public replacement changed another component's row")
	}
	current, err := store.Read(scope, expectation().OperationID)
	if err != nil || current.State.ControlRevision != 1 || current.State.Fence == nil {
		t.Fatal("public replacement changed the protected ledger")
	}
}

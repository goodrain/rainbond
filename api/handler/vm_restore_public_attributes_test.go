package handler

import (
	"errors"
	"testing"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
)

type restorePublicAttributeManager struct {
	db.Manager
	attributes dbdao.ComponentK8sAttributeDao
}

func (m restorePublicAttributeManager) ComponentK8sAttributeDao() dbdao.ComponentK8sAttributeDao {
	return m.attributes
}

type restorePublicAttributeDAO struct {
	dbdao.ComponentK8sAttributeDao
	row      *dbmodel.ComponentK8sAttributes
	writes   int
	batchErr error
}

func (d *restorePublicAttributeDAO) GetByComponentIDAndName(string, string) (*dbmodel.ComponentK8sAttributes, error) {
	return d.row, nil
}
func (d *restorePublicAttributeDAO) AddModel(dbmodel.Interface) error    { d.writes++; return nil }
func (d *restorePublicAttributeDAO) UpdateModel(dbmodel.Interface) error { d.writes++; return nil }
func (d *restorePublicAttributeDAO) DeleteByComponentIDAndName(string, string) error {
	d.writes++
	return nil
}
func (d *restorePublicAttributeDAO) CreateOrUpdateAttributesInBatch([]*dbmodel.ComponentK8sAttributes) error {
	d.writes++
	return d.batchErr
}

// capability_id: rainbond.vm-restore.public-attribute-service
func TestPublicAttributeCallsRejectPrivateNamesAndPersistedAlias(t *testing.T) {
	dao := &restorePublicAttributeDAO{row: &dbmodel.ComponentK8sAttributes{Name: vmrestore.StateKey}}
	action := &ServiceAction{dbmanager: restorePublicAttributeManager{attributes: dao}}
	for _, name := range []string{vmrestore.StateKey, "VM_RESTORE_STATE", vmrestore.ReceiptPrefix + "operation-a", " VM_RESTORE_RECEIPT_OPERATION-A ", vmrestore.LegacyBindingsKey + " "} {
		if _, err := action.GetK8sAttribute("component", name); !errors.Is(err, vmrestore.ErrInternalAttribute) {
			t.Fatal("private read was permitted")
		}
		if err := action.CreateK8sAttribute("tenant", "component", &apimodel.ComponentK8sAttribute{Name: name}); !errors.Is(err, vmrestore.ErrInternalAttribute) {
			t.Fatal("private create was permitted")
		}
		if err := action.UpdateK8sAttribute("component", &apimodel.ComponentK8sAttribute{Name: name}); !errors.Is(err, vmrestore.ErrInternalAttribute) {
			t.Fatal("private update was permitted")
		}
		if err := action.DeleteK8sAttribute("component", name); !errors.Is(err, vmrestore.ErrInternalAttribute) {
			t.Fatal("private delete was permitted")
		}
	}
	// MySQL collation may return the canonical row for a differently accented name.
	if _, err := action.GetK8sAttribute("component", "vḿ_restore_state"); !errors.Is(err, vmrestore.ErrInternalAttribute) {
		t.Fatal("persisted private row was exposed through a collation alias")
	}
	if err := action.UpdateK8sAttribute("component", &apimodel.ComponentK8sAttribute{Name: "vḿ_restore_state"}); !errors.Is(err, vmrestore.ErrInternalAttribute) {
		t.Fatal("persisted private row was overwritten through a collation alias")
	}
	if err := action.DeleteK8sAttribute("component", "vḿ_restore_state"); !errors.Is(err, vmrestore.ErrInternalAttribute) {
		t.Fatal("persisted private row was deleted through a collation alias")
	}
	if err := normalizeVMLoginCredentialAttributes([]apimodel.ComponentK8sAttribute{{Name: vmrestore.StateKey}}); !errors.Is(err, vmrestore.ErrInternalAttribute) {
		t.Fatal("component create accepted a private attribute")
	}
	if dao.writes != 0 {
		t.Fatal("private denial performed a database write")
	}
}

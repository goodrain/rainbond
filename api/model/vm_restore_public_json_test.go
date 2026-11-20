package model

import (
	"encoding/json"
	"strings"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
)

// capability_id: rainbond.vm-restore.public-export-boundary
func TestPublicComponentExportsExcludeRestoreStateAndSourceIdentity(t *testing.T) {
	internal := &dbmodel.ComponentK8sAttributes{Model: dbmodel.Model{ID: 99}, Name: vmrestore.StateKey, AttributeValue: "INTERNAL_STATE_SENTINEL"}
	public := &dbmodel.ComponentK8sAttributes{Model: dbmodel.Model{ID: 77}, Name: "vm_boot_mode", AttributeValue: "bios"}
	values := []interface{}{
		Component{ComponentK8sAttributes: []ComponentK8sAttribute{{Name: internal.Name, AttributeValue: internal.AttributeValue}, {Name: public.Name, AttributeValue: public.AttributeValue}}},
		ConvertResource{ComponentK8sAttributesManagement: []*dbmodel.ComponentK8sAttributes{internal, public}},
		ComponentAttributes{ComponentK8sAttributes: []*dbmodel.ComponentK8sAttributes{internal, public}},
	}
	for _, value := range values {
		payload, err := json.Marshal(value)
		if err != nil {
			t.Fatal("public projection failed")
		}
		if strings.Contains(string(payload), "INTERNAL_STATE_SENTINEL") || strings.Contains(string(payload), vmrestore.StateKey) {
			t.Fatal("public export included internal restore state")
		}
		if !strings.Contains(string(payload), "vm_boot_mode") {
			t.Fatal("public boot config was lost")
		}
		if strings.Contains(string(payload), `"ID":77`) {
			t.Fatal("public export retained source database row identity")
		}
	}
	if internal.ID != 99 || public.ID != 77 {
		t.Fatal("projection mutated source rows")
	}
}

func TestPublicComponentExportsExcludeAllPrivateAttributeAliases(t *testing.T) {
	rows := []*dbmodel.ComponentK8sAttributes{nil}
	attributes := []ComponentK8sAttribute{}
	for _, name := range []string{vmrestore.StateKey, " VM_RESTORE_STATE ", vmrestore.ReceiptPrefix + "operation-a", "VM_RESTORE_RECEIPT_OPERATION-A", vmrestore.LegacyBindingsKey, " VM_VOLUME_BINDINGS ", "vm_login_password_hash", " VM_LOGIN_PASSWORD_HASH "} {
		rows = append(rows, &dbmodel.ComponentK8sAttributes{Name: name, AttributeValue: "PRIVATE_ATTRIBUTE_SENTINEL"})
		attributes = append(attributes, ComponentK8sAttribute{Name: name, AttributeValue: "PRIVATE_ATTRIBUTE_SENTINEL"})
	}
	public := &dbmodel.ComponentK8sAttributes{Model: dbmodel.Model{ID: 77}, TenantID: "tenant", ComponentID: "component", Name: "labels", SaveType: "string", AttributeValue: "PUBLIC_ATTRIBUTE_SENTINEL"}
	rows = append(rows, public)
	attributes = append(attributes, ComponentK8sAttribute{Name: public.Name, SaveType: public.SaveType, AttributeValue: public.AttributeValue})
	for _, value := range []interface{}{
		&Component{ComponentK8sAttributes: attributes},
		&ConvertResource{ComponentsName: "ordinary-component", ComponentK8sAttributesManagement: rows},
		&ComponentAttributes{Image: "ordinary-image", ComponentK8sAttributes: rows},
	} {
		payload, err := json.Marshal(value)
		if err != nil {
			t.Fatal("public projection failed")
		}
		if strings.Contains(string(payload), "PRIVATE_ATTRIBUTE_SENTINEL") || strings.Contains(string(payload), `"ID":77`) {
			t.Fatal("public projection exposed private fields or source identity")
		}
		if !strings.Contains(string(payload), "PUBLIC_ATTRIBUTE_SENTINEL") || !strings.Contains(string(payload), "labels") {
			t.Fatal("public projection removed ordinary attributes")
		}
	}
	if public.ID != 77 || len(attributes) != 9 || len(rows) != 10 {
		t.Fatal("public projection changed source attributes")
	}
}

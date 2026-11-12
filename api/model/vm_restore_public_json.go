package model

import (
	"encoding/json"
	"strings"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
)

func publicVMAttributeName(name string) bool {
	return !vmrestore.IsInternalAttribute(name) && !strings.EqualFold(strings.TrimSpace(name), "vm_login_password_hash")
}
func publicVMAttributeModels(rows []*dbmodel.ComponentK8sAttributes) []*dbmodel.ComponentK8sAttributes {
	copies := vmrestore.PublicAttributeCopies(rows, nil)
	result := make([]*dbmodel.ComponentK8sAttributes, 0, len(copies))
	for _, row := range copies {
		if publicVMAttributeName(row.Name) {
			result = append(result, row)
		}
	}
	return result
}

// MarshalJSON excludes Region-owned state from generic component exports.
func (c Component) MarshalJSON() ([]byte, error) {
	type plain Component
	copy := plain(c)
	copy.ComponentK8sAttributes = nil
	for _, row := range c.ComponentK8sAttributes {
		if publicVMAttributeName(row.Name) {
			copy.ComponentK8sAttributes = append(copy.ComponentK8sAttributes, row)
		}
	}
	return json.Marshal(copy)
}

// MarshalJSON does not return restore records or source database primary IDs.
func (c ConvertResource) MarshalJSON() ([]byte, error) {
	type plain ConvertResource
	copy := plain(c)
	copy.ComponentK8sAttributesManagement = publicVMAttributeModels(c.ComponentK8sAttributesManagement)
	return json.Marshal(copy)
}

// MarshalJSON applies the same projection to imported component responses.
func (c ComponentAttributes) MarshalJSON() ([]byte, error) {
	type plain ComponentAttributes
	copy := plain(c)
	copy.ComponentK8sAttributes = publicVMAttributeModels(c.ComponentK8sAttributes)
	return json.Marshal(copy)
}

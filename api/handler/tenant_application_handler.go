package handler

import (
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
)

// TenantApplicationAction -
type TenantApplicationAction struct{}

// TenantApplicationHandler defines handler methods to TenantApplication.
type TenantApplicationHandler interface {
	CreateApplication(tenantID string) (*model.TenantApplication, error)
}

// NewTenantApplicationHandler creates a new Tenant Application Handler.
func NewTenantApplicationHandler() TenantApplicationHandler {
	return &TenantApplicationAction{}
}

// CreateApplication -
func (a *TenantApplicationAction) CreateApplication(tenantID string) (*model.TenantApplication, error) {
	createApp := &model.TenantApplication{
		TenantID: tenantID,
		AppID:    util.NewUUID(),
	}
	if err := db.GetManager().TenantApplicationDao().AddModel(createApp); err != nil {
		return nil, err
	}
	return createApp, nil
}

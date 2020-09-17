package handler

import (
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
)

// TenantApplicationAction -
type TenantApplicationAction struct{}

// TenantApplicationHandler defines handler methods to TenantApplication.
type TenantApplicationHandler interface {
	CreateApplication(createAppReq *model.TenantApplication) (*model.TenantApplication, error)
}

// NewTenantApplicationHandler creates a new Tenant Application Handler.
func NewTenantApplicationHandler() TenantApplicationHandler {
	return &TenantApplicationAction{}
}

// CreateApplication -
func (a *TenantApplicationAction) CreateApplication(createAppReq *model.TenantApplication) (*model.TenantApplication, error) {
	if err := db.GetManager().TenantApplicationDao().AddModel(createAppReq); err != nil {
		return nil, err
	}
	return createAppReq, nil
}

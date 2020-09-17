package handler

import (
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
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
	appReq := &dbmodel.TenantApplication{
		ApplicationName: createAppReq.ApplicationName,
		ApplicationID:   createAppReq.ApplicationID,
		TenantID:        createAppReq.TenantID,
	}

	if err := db.GetManager().TenantApplicationDao().AddModel(appReq); err != nil {
		return nil, err
	}
	return createAppReq, nil
}

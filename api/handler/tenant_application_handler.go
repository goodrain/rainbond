package handler

import (
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	"github.com/jinzhu/gorm"
)

// TenantApplicationAction -
type TenantApplicationAction struct{}

// TenantApplicationHandler defines handler methods to TenantApplication.
type TenantApplicationHandler interface {
	CreateApp(req *model.Application) (*model.Application, error)
	ListApps(tenantID string, page, pageSize int) ([]*dbmodel.Application, int64, error)
}

// NewTenantApplicationHandler creates a new Tenant Application Handler.
func NewTenantApplicationHandler() TenantApplicationHandler {
	return &TenantApplicationAction{}
}

// CreateApp -
func (a *TenantApplicationAction) CreateApp(req *model.Application) (*model.Application, error) {
	appReq := &dbmodel.Application{
		AppName:  req.AppName,
		AppID:    util.NewUUID(),
		TenantID: req.TenantID,
	}

	if err := db.GetManager().TenantApplicationDao().AddModel(appReq); err != nil {
		return nil, err
	}
	return req, nil
}

// ListApps -
func (a *TenantApplicationAction) ListApps(tenantID string, page, pageSize int) ([]*dbmodel.Application, int64, error) {
	apps, total, err := db.GetManager().TenantApplicationDao().ListApps(tenantID, page, pageSize)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, 0, err
	}
	return apps, total, nil
}

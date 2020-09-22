package handler

import (
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
)

// TenantApplicationAction -
type TenantApplicationAction struct{}

// TenantApplicationHandler defines handler methods to TenantApplication.
type TenantApplicationHandler interface {
	CreateApp(req *model.Application) (*model.Application, error)
	UpdateApp(srcApp *dbmodel.Application, req model.UpdateAppRequest) (*dbmodel.Application, error)
	ListApps(tenantID, appName string, page, pageSize int) (*model.ListAppResponse, error)
	GetAppByID(appID string) (*dbmodel.Application, error)
	DeleteApp(appID string) error
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
	req.AppID = appReq.AppID
	if err := db.GetManager().TenantApplicationDao().AddModel(appReq); err != nil {
		return nil, err
	}
	return req, nil
}

// UpdateApp -
func (a *TenantApplicationAction) UpdateApp(srcApp *dbmodel.Application, req model.UpdateAppRequest) (*dbmodel.Application, error) {
	srcApp.AppName = req.AppName
	if err := db.GetManager().TenantApplicationDao().UpdateModel(srcApp); err != nil {
		return nil, err
	}
	return srcApp, nil
}

// ListApps -
func (a *TenantApplicationAction) ListApps(tenantID, appName string, page, pageSize int) (*model.ListAppResponse, error) {
	var resp model.ListAppResponse
	apps, total, err := db.GetManager().TenantApplicationDao().ListApps(tenantID, appName, page, pageSize)
	if err != nil {
		return nil, err
	}
	if apps != nil {
		resp.Apps = apps
	} else {
		resp.Apps = make([]*dbmodel.Application, 0)
	}

	resp.Page = page
	resp.Total = total
	resp.PageSize = pageSize
	return &resp, nil
}

// GetAppByID -
func (a *TenantApplicationAction) GetAppByID(appID string) (*dbmodel.Application, error) {
	app, err := db.GetManager().TenantApplicationDao().GetAppByID(appID)
	if err != nil {
		return nil, err
	}
	return app, nil
}

// DeleteApp -
func (a *TenantApplicationAction) DeleteApp(appID string) error {
	// Get the number of services under the application
	total, err := db.GetManager().TenantServiceDao().CountServiceByAppID(appID)
	if err != nil {
		return err
	}
	if total != 0 {
		return bcode.ErrDeleteDueToBindService
	}
	return db.GetManager().TenantApplicationDao().DeleteApp(appID)
}

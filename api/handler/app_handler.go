package handler

import (
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
)

// AppHandler defines handler methods to app.
type AppHandler interface {
	CreateApp(tenantID string) (*model.App, error)
}

// NewAppHandler creates a new AppRestoreHandler.
func NewAppHandler() AppHandler {
	return &AppAction{}
}

// CreateApp -
func (a *AppAction) CreateApp(tenantID string) (*model.App, error) {
	createApp := &model.App{
		TenantID: tenantID,
		AppID:    util.NewUUID(),
	}
	if err := db.GetManager().TenantAppDao().AddModel(createApp); err != nil {
		return nil, err
	}
	return createApp, nil
}

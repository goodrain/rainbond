package handler

import (
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

// AppHandler defines handler methods to app.
type AppHandler interface {
	CreateApp(tenantID string) (*model.App, error)
	ListApps(tenantID string, page, pageSize int) ([]*model.App, int64, error)
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

// ListApps -
func (a *AppAction) ListApps(tenantID string, page, pageSize int) ([]*model.App, int64, error) {
	if tenantID == "" {
		err := errors.New("Failed to get tenantID")
		logrus.Error(err)
		return nil, 0, err
	}
	apps, total, err := db.GetManager().TenantAppDao().ListApps(tenantID, page, pageSize)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, 0, err
	}
	return apps, total, nil
}

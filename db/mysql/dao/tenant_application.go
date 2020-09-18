package dao

import (
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// TenantApplicationDaoImpl -
type TenantApplicationDaoImpl struct {
	DB *gorm.DB
}

//AddModel -
func (a *TenantApplicationDaoImpl) AddModel(mo model.Interface) error {
	appReq, _ := mo.(*model.Application)
	var oldApp model.Application
	if err := a.DB.Where("tenant_id = ? AND app_id = ?", appReq.TenantID, appReq.AppID).Find(&oldApp).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return a.DB.Create(appReq).Error
		}
		return err
	}

	return nil
}

//UpdateModel -
func (a *TenantApplicationDaoImpl) UpdateModel(mo model.Interface) error {
	updateReq := mo.(*model.Application)
	var oldApp model.Application
	if err := a.DB.Where("tenant_id = ? AND app_id = ?", updateReq.TenantID, updateReq.AppID).Find(&oldApp).Error; err != nil {
		return err
	}
	return a.DB.Model(&oldApp).Update("app_name", updateReq.AppName).Error
}

// ListApps -
func (a *TenantApplicationDaoImpl) ListApps(tenantID string, page, pageSize int) ([]*model.Application, int64, error) {
	var datas []*model.Application
	offset := (page - 1) * pageSize

	db := a.DB.Where("tenant_id=?", tenantID).Order("create_time desc")

	var total int64
	if err := db.Model(&model.Application{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Limit(pageSize).Offset(offset).Find(&datas).Error; err != nil {
		return nil, 0, err
	}
	return datas, total, nil
}

// GetAppByID -
func (a *TenantApplicationDaoImpl) GetAppByID(appID string) (*model.Application, error) {
	var app model.Application
	if err := a.DB.Where("app_id=?", appID).Find(&app).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

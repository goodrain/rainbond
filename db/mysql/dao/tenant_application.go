package dao

import (
	"github.com/goodrain/rainbond/api/util/bcode"
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
	return bcode.ErrApplicationExist
}

//UpdateModel -
func (a *TenantApplicationDaoImpl) UpdateModel(mo model.Interface) error {
	updateReq := mo.(*model.Application)
	return a.DB.Save(updateReq).Error
}

// ListApps -
func (a *TenantApplicationDaoImpl) ListApps(tenantID, appName string, page, pageSize int) ([]*model.Application, int64, error) {
	var datas []*model.Application
	offset := (page - 1) * pageSize

	db := a.DB.Where("tenant_id=?", tenantID).Order("create_time desc")
	if appName != "" {
		db = db.Where("app_name like ?", "%"+appName+"%")
	}
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
		if err == gorm.ErrRecordNotFound {
			return nil, bcode.ErrApplicationNotFound
		}
		return nil, err
	}
	return &app, nil
}

// DeleteApp Delete application By appID -
func (a *TenantApplicationDaoImpl) DeleteApp(appID string) error {
	var app model.Application
	if err := a.DB.Where("app_id=?", appID).Find(&app).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return bcode.ErrApplicationNotFound
		}
		return err
	}
	return a.DB.Delete(&app).Error
}

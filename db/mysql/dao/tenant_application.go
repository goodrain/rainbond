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
	if err := a.DB.Where("tenantID = ? AND appID = ?", appReq.TenantID, appReq.AppID).Find(&oldApp).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return a.DB.Create(appReq).Error
		}
		return err
	}

	return nil
}

//UpdateModel -
func (a *TenantApplicationDaoImpl) UpdateModel(mo model.Interface) error {

	return nil
}

// ListApps -
func (a *TenantApplicationDaoImpl) ListApps(tenantID string, page, pageSize int) ([]*model.Application, int64, error) {
	var datas []*model.Application
	offset := (page - 1) * pageSize

	db := a.DB.Where("tenantID=?", tenantID).Order("create_time desc")

	var total int64
	if err := db.Model(&model.Application{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Limit(pageSize).Offset(offset).Find(&datas).Error; err != nil {
		return nil, 0, err
	}
	return datas, total, nil
}

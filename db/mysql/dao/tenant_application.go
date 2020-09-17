package dao

import (
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

// TenantApplicationDaoImpl -
type TenantApplicationDaoImpl struct {
	DB *gorm.DB
}

//AddModel -
func (a *TenantApplicationDaoImpl) AddModel(mo model.Interface) error {
	appReq, ok := mo.(*model.TenantApplication)
	if !ok {
		return errors.New("Failed to convert interface to App")
	}

	var oldApp model.TenantApplication
	if err := a.DB.Where("tenantID = ? AND applicationID = ?", appReq.TenantID, appReq.ApplicationID).Find(&oldApp).Error; err != nil {
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

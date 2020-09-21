package dao

import (
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// ApplicationConfigDaoImpl -
type ApplicationConfigDaoImpl struct {
	DB *gorm.DB
}

//AddModel -
func (a *ApplicationConfigDaoImpl) AddModel(mo model.Interface) error {
	configReq, _ := mo.(*model.ApplicationConfigGroup)
	var oldApp model.ApplicationConfigGroup
	if err := a.DB.Where("config_group_name = ?", configReq.ConfigGroupName).Find(&oldApp).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return a.DB.Create(configReq).Error
		}
		return err
	}
	return bcode.ErrApplicationConfigGroupExist
}

//UpdateModel -
func (a *ApplicationConfigDaoImpl) UpdateModel(mo model.Interface) error {
	// updateReq := mo.(*model.Application)
	return nil
}

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
	if err := a.DB.Where("app_id = ? AND config_group_name = ?", configReq.AppID, configReq.ConfigGroupName).Find(&oldApp).Error; err != nil {
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

// GetConfigByID -
func (a *ApplicationConfigDaoImpl) GetConfigByID(appID, name string) (*model.ApplicationConfigGroup, error) {
	var oldApp model.ApplicationConfigGroup
	if err := a.DB.Where("app_id = ? AND config_group_name = ?", appID, name).Find(&oldApp).Error; err != nil {
		return nil, err
	}
	return &oldApp, nil
}

// ServiceConfigGroupDaoImpl -
type ServiceConfigGroupDaoImpl struct {
	DB *gorm.DB
}

//AddModel -
func (a *ServiceConfigGroupDaoImpl) AddModel(mo model.Interface) error {
	configReq, _ := mo.(*model.ServiceConfigGroup)
	var oldApp model.ServiceConfigGroup
	if err := a.DB.Where("app_id = ? AND config_group_name = ? AND service_id = ?", configReq.AppID, configReq.ConfigGroupName, configReq.ServiceID).Find(&oldApp).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return a.DB.Create(configReq).Error
		}
		return err
	}
	return bcode.ErrApplicationConfigGroupExist
}

//UpdateModel -
func (a *ServiceConfigGroupDaoImpl) UpdateModel(mo model.Interface) error {
	// updateReq := mo.(*model.Application)
	return nil
}

// ConfigItemDaoImpl -
type ConfigItemDaoImpl struct {
	DB *gorm.DB
}

//AddModel -
func (a *ConfigItemDaoImpl) AddModel(mo model.Interface) error {
	configReq, _ := mo.(*model.ConfigItem)
	var oldApp model.ConfigItem
	if err := a.DB.Where("app_id = ? AND config_group_name = ? AND item_key = ?", configReq.AppID, configReq.ConfigGroupName, configReq.ItemKey).Find(&oldApp).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return a.DB.Create(configReq).Error
		}
		return err
	}
	return bcode.ErrApplicationConfigGroupExist
}

//UpdateModel -
func (a *ConfigItemDaoImpl) UpdateModel(mo model.Interface) error {
	// updateReq := mo.(*model.Application)
	return nil
}

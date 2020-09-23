package dao

import (
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// AppConfigGroupDaoImpl -
type AppConfigGroupDaoImpl struct {
	DB *gorm.DB
}

//AddModel -
func (a *AppConfigGroupDaoImpl) AddModel(mo model.Interface) error {
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
func (a *AppConfigGroupDaoImpl) UpdateModel(mo model.Interface) error {
	return nil
}

// GetConfigGroupByID -
func (a *AppConfigGroupDaoImpl) GetConfigGroupByID(appID, configGroupName string) (*model.ApplicationConfigGroup, error) {
	var oldApp model.ApplicationConfigGroup
	if err := a.DB.Where("app_id = ? AND config_group_name = ?", appID, configGroupName).Find(&oldApp).Error; err != nil {
		return nil, err
	}
	return &oldApp, nil
}

// AppConfigGroupServiceDaoImpl -
type AppConfigGroupServiceDaoImpl struct {
	DB *gorm.DB
}

//AddModel -
func (a *AppConfigGroupServiceDaoImpl) AddModel(mo model.Interface) error {
	configReq, _ := mo.(*model.ServiceConfigGroup)
	var oldApp model.ServiceConfigGroup
	if err := a.DB.Where("app_id = ? AND config_group_name = ? AND service_id = ?", configReq.AppID, configReq.ConfigGroupName, configReq.ServiceID).Find(&oldApp).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return a.DB.Create(configReq).Error
		}
		return err
	}
	return bcode.ErrServiceConfigGroupExist
}

//UpdateModel -
func (a *AppConfigGroupServiceDaoImpl) UpdateModel(mo model.Interface) error {
	return nil
}

//DeleteServiceConfig -
func (a *AppConfigGroupServiceDaoImpl) DeleteServiceConfig(appID, configGroupName string) error {
	return a.DB.Where("app_id = ? AND config_group_name = ?", appID, configGroupName).Delete(model.ServiceConfigGroup{}).Error
}

// AppConfigGroupItemDaoImpl -
type AppConfigGroupItemDaoImpl struct {
	DB *gorm.DB
}

//AddModel -
func (a *AppConfigGroupItemDaoImpl) AddModel(mo model.Interface) error {
	configReq, _ := mo.(*model.ConfigItem)
	var oldApp model.ConfigItem
	if err := a.DB.Where("app_id = ? AND config_group_name = ? AND item_key = ?", configReq.AppID, configReq.ConfigGroupName, configReq.ItemKey).Find(&oldApp).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return a.DB.Create(configReq).Error
		}
		return err
	}
	return bcode.ErrConfigItemExist
}

//UpdateModel -
func (a *AppConfigGroupItemDaoImpl) UpdateModel(mo model.Interface) error {
	updateReq := mo.(*model.ConfigItem)
	return a.DB.Model(&model.ConfigItem{}).
		Where("app_id = ? AND config_group_name = ? AND item_key = ?", updateReq.AppID, updateReq.ConfigGroupName, updateReq.ItemKey).
		Update("item_value", updateReq.ItemValue).Error
}

package dao

import (
	gormbulkups "github.com/atcdot/gorm-bulk-upsert"
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	pkgerr "github.com/pkg/errors"
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
	updateReq := mo.(*model.ApplicationConfigGroup)
	return a.DB.Model(&model.ApplicationConfigGroup{}).Where("app_id = ? AND config_group_name = ?", updateReq.AppID, updateReq.ConfigGroupName).Update("enable", updateReq.Enable).Error
}

// GetConfigGroupByID -
func (a *AppConfigGroupDaoImpl) GetConfigGroupByID(appID, configGroupName string) (*model.ApplicationConfigGroup, error) {
	var oldApp model.ApplicationConfigGroup
	if err := a.DB.Where("app_id = ? AND config_group_name = ?", appID, configGroupName).Find(&oldApp).Error; err != nil {
		return nil, err
	}
	return &oldApp, nil
}

// ListByServiceID -
func (a *AppConfigGroupDaoImpl) ListByServiceID(sid string) ([]*model.ApplicationConfigGroup, error) {
	var groups []*model.ApplicationConfigGroup
	if err := a.DB.Model(model.ApplicationConfigGroup{}).Select("app_config_group.*").Joins("left join app_config_group_service on app_config_group.app_id = app_config_group_service.app_id and app_config_group.config_group_name = app_config_group_service.config_group_name").
		Where("app_config_group_service.service_id = ? and enable = true", sid).Scan(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// GetConfigGroupsByAppID -
func (a *AppConfigGroupDaoImpl) GetConfigGroupsByAppID(appID string, page, pageSize int) ([]*model.ApplicationConfigGroup, int64, error) {
	var oldApp []*model.ApplicationConfigGroup
	offset := (page - 1) * pageSize
	db := a.DB.Where("app_id = ?", appID).Order("create_time desc")

	var total int64
	if err := db.Model(&model.ApplicationConfigGroup{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Limit(pageSize).Offset(offset).Find(&oldApp).Error; err != nil {
		return nil, 0, err
	}
	return oldApp, total, nil
}

//DeleteConfigGroup -
func (a *AppConfigGroupDaoImpl) DeleteConfigGroup(appID, configGroupName string) error {
	return a.DB.Where("app_id = ? AND config_group_name = ?", appID, configGroupName).Delete(model.ApplicationConfigGroup{}).Error
}

//DeleteByAppID -
func (a *AppConfigGroupDaoImpl) DeleteByAppID(appID string) error {
	return a.DB.Where("app_id = ?", appID).Delete(model.ApplicationConfigGroup{}).Error
}

// CreateOrUpdateConfigGroupsInBatch -
func (a *AppConfigGroupDaoImpl) CreateOrUpdateConfigGroupsInBatch(cgroups []*model.ApplicationConfigGroup) error {
	var objects []interface{}
	for _, cg := range cgroups {
		objects = append(objects, *cg)
	}
	if err := gormbulkups.BulkUpsert(a.DB, objects, 2000); err != nil {
		return pkgerr.Wrap(err, "create or update config groups in batch")
	}
	return nil
}

// AppConfigGroupServiceDaoImpl -
type AppConfigGroupServiceDaoImpl struct {
	DB *gorm.DB
}

//AddModel -
func (a *AppConfigGroupServiceDaoImpl) AddModel(mo model.Interface) error {
	configReq, _ := mo.(*model.ConfigGroupService)
	var oldApp model.ConfigGroupService
	if err := a.DB.Where("app_id = ? AND config_group_name = ? AND service_id = ?", configReq.AppID, configReq.ConfigGroupName, configReq.ServiceID).Find(&oldApp).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return a.DB.Create(configReq).Error
		}
		return err
	}
	return bcode.ErrConfigGroupServiceExist
}

//UpdateModel -
func (a *AppConfigGroupServiceDaoImpl) UpdateModel(mo model.Interface) error {
	return nil
}

// GetConfigGroupServicesByID -
func (a *AppConfigGroupServiceDaoImpl) GetConfigGroupServicesByID(appID, configGroupName string) ([]*model.ConfigGroupService, error) {
	var oldApp []*model.ConfigGroupService
	if err := a.DB.Where("app_id = ? AND config_group_name = ?", appID, configGroupName).Find(&oldApp).Error; err != nil {
		return nil, err
	}
	return oldApp, nil
}

//DeleteConfigGroupService -
func (a *AppConfigGroupServiceDaoImpl) DeleteConfigGroupService(appID, configGroupName string) error {
	return a.DB.Where("app_id = ? AND config_group_name = ?", appID, configGroupName).Delete(model.ConfigGroupService{}).Error
}

//DeleteEffectiveServiceByServiceID -
func (a *AppConfigGroupServiceDaoImpl) DeleteEffectiveServiceByServiceID(serviceID string) error {
	return a.DB.Where("service_id = ?", serviceID).Delete(model.ConfigGroupService{}).Error
}

//DeleteByComponentIDs -
func (a *AppConfigGroupServiceDaoImpl) DeleteByComponentIDs(componentIDs []string) error {
	return a.DB.Where("service_id in (?)", componentIDs).Delete(model.ConfigGroupService{}).Error
}

// DeleteByAppID deletes ConfigGroupService based on the given appID.
func (a *AppConfigGroupServiceDaoImpl) DeleteByAppID(appID string) error {
	return a.DB.Where("app_id = ?", appID).Delete(model.ConfigGroupService{}).Error
}

// CreateOrUpdateConfigGroupServicesInBatch -
func (a *AppConfigGroupServiceDaoImpl) CreateOrUpdateConfigGroupServicesInBatch(cgservices []*model.ConfigGroupService) error {
	var objects []interface{}
	for _, cgs := range cgservices {
		objects = append(objects, *cgs)
	}
	if err := gormbulkups.BulkUpsert(a.DB, objects, 2000); err != nil {
		return pkgerr.Wrap(err, "create or update config group services in batch")
	}
	return nil
}

// AppConfigGroupItemDaoImpl -
type AppConfigGroupItemDaoImpl struct {
	DB *gorm.DB
}

//AddModel -
func (a *AppConfigGroupItemDaoImpl) AddModel(mo model.Interface) error {
	configReq, _ := mo.(*model.ConfigGroupItem)
	var oldApp model.ConfigGroupItem
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
	updateReq := mo.(*model.ConfigGroupItem)
	return a.DB.Model(&model.ConfigGroupItem{}).
		Where("app_id = ? AND config_group_name = ? AND item_key = ?", updateReq.AppID, updateReq.ConfigGroupName, updateReq.ItemKey).
		Update("item_value", updateReq.ItemValue).Error
}

// GetConfigGroupItemsByID -
func (a *AppConfigGroupItemDaoImpl) GetConfigGroupItemsByID(appID, configGroupName string) ([]*model.ConfigGroupItem, error) {
	var oldApp []*model.ConfigGroupItem
	if err := a.DB.Where("app_id = ? AND config_group_name = ?", appID, configGroupName).Find(&oldApp).Error; err != nil {
		return nil, err
	}
	return oldApp, nil
}

// ListByServiceID -
func (a *AppConfigGroupItemDaoImpl) ListByServiceID(sid string) ([]*model.ConfigGroupItem, error) {
	var items []*model.ConfigGroupItem
	if err := a.DB.Model(model.ConfigGroupItem{}).Select("app_config_group_item.*").Joins("left join app_config_group_service on app_config_group_item.app_id = app_config_group_service.app_id and app_config_group_item.config_group_name = app_config_group_service.config_group_name").
		Where("app_config_group_service.service_id = ?", sid).Scan(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

//DeleteConfigGroupItem -
func (a *AppConfigGroupItemDaoImpl) DeleteConfigGroupItem(appID, configGroupName string) error {
	return a.DB.Where("app_id = ? AND config_group_name = ?", appID, configGroupName).Delete(model.ConfigGroupItem{}).Error
}

//DeleteByAppID -
func (a *AppConfigGroupItemDaoImpl) DeleteByAppID(appID string) error {
	return a.DB.Where("app_id = ?", appID).Delete(model.ConfigGroupItem{}).Error
}

// CreateOrUpdateConfigGroupItemsInBatch -
func (a *AppConfigGroupItemDaoImpl) CreateOrUpdateConfigGroupItemsInBatch(cgitems []*model.ConfigGroupItem) error {
	var objects []interface{}
	for _, cgi := range cgitems {
		objects = append(objects, *cgi)
	}
	if err := gormbulkups.BulkUpsert(a.DB, objects, 2000); err != nil {
		return pkgerr.Wrap(err, "create or update config group items in batch")
	}
	return nil
}

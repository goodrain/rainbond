package handler

import (
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/sirupsen/logrus"
)

// AddConfigGroup -
func (a *ApplicationAction) AddConfigGroup(appID string, req *model.ApplicationConfigGroup) (*model.ApplicationConfigGroupResp, error) {
	var serviceResp []dbmodel.ServiceConfigGroup
	services, err := db.GetManager().TenantServiceDao().GetServicesByServiceIDs(req.ServiceIDs)
	if err != nil {
		return nil, err
	}

	tx := db.GetManager().Begin()
	// Create application configGroup-services
	for _, s := range services {
		serviceConfigGroup := dbmodel.ServiceConfigGroup{
			AppID:           appID,
			ConfigGroupName: req.ConfigGroupName,
			ServiceID:       s.ServiceID,
			ServiceAlias:    s.ServiceAlias,
		}
		serviceResp = append(serviceResp, serviceConfigGroup)
		if err := db.GetManager().AppConfigGroupServiceDao().AddModel(&serviceConfigGroup); err != nil {
			if err == bcode.ErrServiceConfigGroupExist {
				logrus.Warningf("config group \"%s\" under this service \"%s\" already exists.", serviceConfigGroup.ConfigGroupName, serviceConfigGroup.ServiceID)
				continue
			}
			tx.Rollback()
			return nil, err
		}
	}

	// Create application configGroup-configItem
	for _, it := range req.ConfigItems {
		configItem := &dbmodel.ConfigItem{
			AppID:           appID,
			ConfigGroupName: req.ConfigGroupName,
			ItemKey:         it.ItemKey,
			ItemValue:       it.ItemValue,
		}
		if err := db.GetManager().AppConfigGroupItemDaoTransactions(tx).AddModel(configItem); err != nil {
			if err == bcode.ErrConfigItemExist {
				logrus.Warningf("config item \"%s\" under this config group \"%s\" already exists.", configItem.ItemKey, configItem.ConfigGroupName)
				continue
			}
			tx.Rollback()
			return nil, err
		}
	}

	// Create application configGroup
	config := &dbmodel.ApplicationConfigGroup{
		AppID:           appID,
		ConfigGroupName: req.ConfigGroupName,
		DeployType:      req.DeployType,
	}
	if err := db.GetManager().AppConfigGroupDaoTransactions(tx).AddModel(config); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	appconfig, err := db.GetManager().AppConfigGroupDao().GetConfigGroupByID(appID, req.ConfigGroupName)
	if err != nil {
		return nil, err
	}
	var resp *model.ApplicationConfigGroupResp
	resp = &model.ApplicationConfigGroupResp{
		CreateTime:      appconfig.CreatedAt,
		AppID:           appID,
		ConfigGroupName: appconfig.ConfigGroupName,
		DeployType:      appconfig.DeployType,
		ConfigItems:     req.ConfigItems,
		Services:        serviceResp,
	}
	return resp, nil
}

// UpdateConfigGroup -
func (a *ApplicationAction) UpdateConfigGroup(appID, configGroupName string, req *model.UpdateAppConfigGroupReq) (*model.ApplicationConfigGroupResp, error) {
	var serviceResp []dbmodel.ServiceConfigGroup
	services, err := db.GetManager().TenantServiceDao().GetServicesByServiceIDs(req.ServiceIDs)
	if err != nil {
		return nil, err
	}

	tx := db.GetManager().Begin()
	// Update application configGroup-services
	if err := db.GetManager().AppConfigGroupServiceDaoTransactions(tx).DeleteServiceConfig(appID, configGroupName); err != nil {
		tx.Rollback()
		return nil, err
	}
	for _, s := range services {
		serviceConfigGroup := dbmodel.ServiceConfigGroup{
			AppID:           appID,
			ConfigGroupName: configGroupName,
			ServiceID:       s.ServiceID,
			ServiceAlias:    s.ServiceAlias,
		}
		serviceResp = append(serviceResp, serviceConfigGroup)
		if err := db.GetManager().AppConfigGroupServiceDaoTransactions(tx).AddModel(&serviceConfigGroup); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// Update application configGroup-configItem
	for _, it := range req.ConfigItems {
		configItem := &dbmodel.ConfigItem{
			AppID:           appID,
			ConfigGroupName: configGroupName,
			ItemKey:         it.ItemKey,
			ItemValue:       it.ItemValue,
		}
		if err := db.GetManager().AppConfigGroupItemDaoTransactions(tx).UpdateModel(configItem); err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	
	// Build return data
	appconfig, err := db.GetManager().AppConfigGroupDao().GetConfigGroupByID(appID, configGroupName)
	if err != nil {
		return nil, err
	}
	var resp *model.ApplicationConfigGroupResp
	resp = &model.ApplicationConfigGroupResp{
		CreateTime:      appconfig.CreatedAt,
		AppID:           appconfig.AppID,
		ConfigGroupName: appconfig.ConfigGroupName,
		DeployType:      appconfig.DeployType,
		ConfigItems:     req.ConfigItems,
		Services:        serviceResp,
	}
	return resp, nil
}

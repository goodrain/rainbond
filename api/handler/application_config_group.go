package handler

import (
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

// AddConfigGroup -
func (a *ApplicationAction) AddConfigGroup(appID string, req *model.ApplicationConfigGroup) (*model.ApplicationConfigGroupResp, error) {
	services, err := db.GetManager().TenantServiceDao().GetServicesByServiceIDs(req.ServiceIDs)
	if err != nil {
		return nil, err
	}

	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
	// Create application configGroup-services
	for _, s := range services {
		serviceConfigGroup := dbmodel.ConfigGroupService{
			AppID:           appID,
			ConfigGroupName: req.ConfigGroupName,
			ServiceID:       s.ServiceID,
			ServiceAlias:    s.ServiceAlias,
		}
		if err := db.GetManager().AppConfigGroupServiceDaoTransactions(tx).AddModel(&serviceConfigGroup); err != nil {
			if err == bcode.ErrConfigGroupServiceExist {
				logrus.Warningf("config group \"%s\" under this service \"%s\" already exists.", serviceConfigGroup.ConfigGroupName, serviceConfigGroup.ServiceID)
				continue
			}
			tx.Rollback()
			return nil, err
		}
	}

	// Create application configGroup-configItem
	for _, it := range req.ConfigItems {
		configItem := &dbmodel.ConfigGroupItem{
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
		Enable:          req.Enable,
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
	configGroupServices, err := db.GetManager().AppConfigGroupServiceDao().GetConfigGroupServicesByID(appID, req.ConfigGroupName)
	if err != nil {
		return nil, err
	}
	configGroupItems, err := db.GetManager().AppConfigGroupItemDao().GetConfigGroupItemsByID(appID, req.ConfigGroupName)
	if err != nil {
		return nil, err
	}
	var resp *model.ApplicationConfigGroupResp
	resp = &model.ApplicationConfigGroupResp{
		CreateTime:      appconfig.CreatedAt,
		AppID:           appID,
		ConfigGroupName: appconfig.ConfigGroupName,
		DeployType:      appconfig.DeployType,
		ConfigItems:     configGroupItems,
		Services:        configGroupServices,
		Enable:          appconfig.Enable,
	}
	return resp, nil
}

// UpdateConfigGroup -
func (a *ApplicationAction) UpdateConfigGroup(appID, configGroupName string, req *model.UpdateAppConfigGroupReq) (*model.ApplicationConfigGroupResp, error) {
	appconfig, err := db.GetManager().AppConfigGroupDao().GetConfigGroupByID(appID, configGroupName)
	if err != nil {
		return nil, err
	}
	services, err := db.GetManager().TenantServiceDao().GetServicesByServiceIDs(req.ServiceIDs)
	if err != nil {
		return nil, err
	}

	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
	// Update effective status
	appconfig.Enable = req.Enable
	if err := db.GetManager().AppConfigGroupDaoTransactions(tx).UpdateModel(appconfig); err != nil {
		tx.Rollback()
		return nil, err
	}
	// Update application configGroup-services
	if err := db.GetManager().AppConfigGroupServiceDaoTransactions(tx).DeleteConfigGroupService(appID, configGroupName); err != nil {
		tx.Rollback()
		return nil, err
	}
	for _, s := range services {
		serviceConfigGroup := dbmodel.ConfigGroupService{
			AppID:           appID,
			ConfigGroupName: configGroupName,
			ServiceID:       s.ServiceID,
			ServiceAlias:    s.ServiceAlias,
		}
		if err := db.GetManager().AppConfigGroupServiceDaoTransactions(tx).AddModel(&serviceConfigGroup); err != nil {
			if err == bcode.ErrConfigGroupServiceExist {
				logrus.Debugf("config group \"%s\" under this service \"%s\" already exists.", serviceConfigGroup.ConfigGroupName, serviceConfigGroup.ServiceID)
				continue
			}
			tx.Rollback()
			return nil, err
		}
	}

	// Update application configGroup-configItem
	if err := db.GetManager().AppConfigGroupItemDaoTransactions(tx).DeleteConfigGroupItem(appID, configGroupName); err != nil {
		tx.Rollback()
		return nil, err
	}
	for _, it := range req.ConfigItems {
		configItem := &dbmodel.ConfigGroupItem{
			AppID:           appID,
			ConfigGroupName: configGroupName,
			ItemKey:         it.ItemKey,
			ItemValue:       it.ItemValue,
		}
		if err := db.GetManager().AppConfigGroupItemDaoTransactions(tx).AddModel(configItem); err != nil {
			if err == bcode.ErrConfigItemExist {
				logrus.Debugf("config item \"%s\" under this config group \"%s\" already exists.", configItem.ItemKey, configItem.ConfigGroupName)
				continue
			}
			tx.Rollback()
			return nil, err
		}
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	configGroupServices, err := db.GetManager().AppConfigGroupServiceDao().GetConfigGroupServicesByID(appID, configGroupName)
	if err != nil {
		return nil, err
	}
	configGroupItems, err := db.GetManager().AppConfigGroupItemDao().GetConfigGroupItemsByID(appID, configGroupName)
	if err != nil {
		return nil, err
	}

	// Build return data
	var resp *model.ApplicationConfigGroupResp
	resp = &model.ApplicationConfigGroupResp{
		CreateTime:      appconfig.CreatedAt,
		AppID:           appconfig.AppID,
		ConfigGroupName: appconfig.ConfigGroupName,
		DeployType:      appconfig.DeployType,
		ConfigItems:     configGroupItems,
		Services:        configGroupServices,
		Enable:          appconfig.Enable,
	}
	return resp, nil
}

// DeleteConfigGroup -
func (a *ApplicationAction) DeleteConfigGroup(appID, configGroupName string) error {
	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
	// Delete application configGroup-services
	if err := db.GetManager().AppConfigGroupServiceDaoTransactions(tx).DeleteConfigGroupService(appID, configGroupName); err != nil {
		tx.Rollback()
		return err
	}
	// Delete application configGroup-configItem
	if err := db.GetManager().AppConfigGroupItemDaoTransactions(tx).DeleteConfigGroupItem(appID, configGroupName); err != nil {
		tx.Rollback()
		return err
	}
	// Delete application configGroup
	if err := db.GetManager().AppConfigGroupDaoTransactions(tx).DeleteConfigGroup(appID, configGroupName); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

// ListConfigGroups -
func (a *ApplicationAction) ListConfigGroups(appID string, page, pageSize int) (*model.ListApplicationConfigGroupResp, error) {
	var resp model.ListApplicationConfigGroupResp

	configGroups, total, err := db.GetManager().AppConfigGroupDao().GetConfigGroupsByAppID(appID, page, pageSize)
	if err != nil {
		return nil, err
	}
	for _, c := range configGroups {
		cgroup := model.ApplicationConfigGroupResp{
			CreateTime:      c.CreatedAt,
			AppID:           c.AppID,
			ConfigGroupName: c.ConfigGroupName,
			DeployType:      c.DeployType,
			Enable:          c.Enable,
		}

		configGroupServices, err := db.GetManager().AppConfigGroupServiceDao().GetConfigGroupServicesByID(c.AppID, c.ConfigGroupName)
		if err != nil {
			return nil, err
		}

		configGroupItems, err := db.GetManager().AppConfigGroupItemDao().GetConfigGroupItemsByID(c.AppID, c.ConfigGroupName)
		if err != nil {
			return nil, err
		}

		cgroup.Services = configGroupServices
		cgroup.ConfigItems = configGroupItems
		resp.ConfigGroup = append(resp.ConfigGroup, cgroup)
	}
	resp.Page = page
	resp.Total = total
	resp.PageSize = pageSize
	return &resp, nil
}

// SyncComponentConfigGroupRels -
func (a *ApplicationAction) SyncComponentConfigGroupRels(tx *gorm.DB, app *dbmodel.Application, components []*model.Component) error{
	var (
		componentIDs []string
		cgservices []*dbmodel.ConfigGroupService
	)
	for _, component := range components {
		if component.AppConfigGroupRels != nil {
			componentIDs = append(componentIDs, component.ComponentBase.ComponentID)
			for _, acgr := range component.AppConfigGroupRels {
				cgservices = append(cgservices, acgr.DbModel(app.AppID, component.ComponentBase.ComponentID, component.ComponentBase.ComponentAlias))
			}
		}
	}
	if err := db.GetManager().AppConfigGroupServiceDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	return db.GetManager().AppConfigGroupServiceDaoTransactions(tx).CreateOrUpdateConfigGroupServicesInBatch(cgservices)
}

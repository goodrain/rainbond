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
		if err := db.GetManager().AppConfigGroupItemDao().AddModel(configItem); err != nil {
			if err == bcode.ErrConfigItemExist {
				logrus.Warningf("config item \"%s\" under this config group \"%s\" already exists.", configItem.ItemKey, configItem.ConfigGroupName)
				continue
			}
			return nil, err
		}
	}

	// Create application configGroup
	config := &dbmodel.ApplicationConfigGroup{
		AppID:           appID,
		ConfigGroupName: req.ConfigGroupName,
		DeployType:      req.DeployType,
	}
	if err := db.GetManager().AppConfigGroupDao().AddModel(config); err != nil {
		return nil, err
	}

	appconfig, err := db.GetManager().AppConfigGroupDao().GetConfigByID(appID, req.ConfigGroupName)
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

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
	serviceConfigGroup := &dbmodel.ServiceConfigGroup{
		AppID:           appID,
		ConfigGroupName: req.ConfigGroupName,
	}
	configItem := &dbmodel.ConfigItem{
		AppID:           appID,
		ConfigGroupName: req.ConfigGroupName,
	}
	config := &dbmodel.ApplicationConfigGroup{
		AppID:           appID,
		ConfigGroupName: req.ConfigGroupName,
		DeployType:      req.DeployType,
	}

	// Create application ConfigGroup
	for _, sID := range req.ServiceIDs {
		serviceConfigGroup.ServiceID = sID
		if err := db.GetManager().ServiceConfigGroupDao().AddModel(serviceConfigGroup); err != nil {
			if err == bcode.ErrServiceConfigGroupExist {
				logrus.Warningf("config group \"%s\" under this service \"%s\" already exists.", serviceConfigGroup.ConfigGroupName, serviceConfigGroup.ServiceID)
				continue
			}
			return nil, err
		}
		serviceConfigGroup.ID++
	}
	for _, it := range req.ConfigItems {
		configItem.ItemKey = it.ItemKey
		configItem.ItemValue = it.ItemValue
		if err := db.GetManager().ConfigItemDao().AddModel(configItem); err != nil {
			if err == bcode.ErrConfigItemExist {
				logrus.Warningf("config item \"%s\" under this config group \"%s\" already exists.", configItem.ItemKey, configItem.ConfigGroupName)
				continue
			}
			return nil, err
		}
		configItem.ID++
	}
	if err := db.GetManager().ApplicationConfigDao().AddModel(config); err != nil {
		return nil, err
	}

	services := db.GetManager().TenantServiceDao().GetServicesIDAndNameByAppID(appID)
	appconfig, _ := db.GetManager().ApplicationConfigDao().GetConfigByID(appID, req.ConfigGroupName)
	var resp *model.ApplicationConfigGroupResp
	resp = &model.ApplicationConfigGroupResp{
		CreateTime:      appconfig.CreatedAt,
		AppID:           appID,
		ConfigGroupName: appconfig.ConfigGroupName,
		DeployType:      appconfig.DeployType,
		ConfigItems:     req.ConfigItems,
		Services:        services,
	}
	return resp, nil
}

package handler

import (
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
)

// AddConfigGroup -
func (a *ApplicationAction) AddConfigGroup(appID string, req *model.ApplicationConfigGroup) (*dbmodel.ApplicationConfigGroup, error) {
	var (
		serviceIDs  []string
		configItems []*dbmodel.ConfigItem
	)
	for _, sID := range req.ServiceIDs {
		serviceIDs = append(serviceIDs, sID)
	}
	for _, it := range req.ConfigItems {
		item := &dbmodel.ConfigItem{
			Key:   it.Key,
			Value: it.Value,
		}
		configItems = append(configItems, item)
	}

	config := &dbmodel.ApplicationConfigGroup{
		AppID:           appID,
		ConfigGroupName: req.ConfigGroupName,
		DeployType:      req.DeployType,
		ServiceIDs:      serviceIDs,
		ConfigItems:     configItems,
	}
	if err := db.GetManager().ApplicationConfigDao().AddModel(config); err != nil {
		return nil, err
	}
	return config, nil
}

package handler

import (
	"testing"

	"github.com/go-playground/assert/v2"
	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	daomock "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
)

func TestCreateAppConfigGroup(t *testing.T) {
	appID := "appID1"
	serviceID := "sid1"
	configName := "configname1"
	serviceConfigGroupReq := &dbmodel.ServiceConfigGroup{
		AppID:           appID,
		ConfigGroupName: configName,
		ServiceID:       serviceID,
	}
	configReq := &dbmodel.ApplicationConfigGroup{
		AppID:           appID,
		ConfigGroupName: configName,
		DeployType:      "env",
	}

	var configItems []model.ConfigItem
	it1 := model.ConfigItem{
		AppID:           appID,
		ConfigGroupName: configName,
		ItemKey:         "key1",
		ItemValue:       "value1",
	}
	configItems = append(configItems, it1)
	it2 := model.ConfigItem{
		AppID:           appID,
		ConfigGroupName: configName,
		ItemKey:         "key2",
		ItemValue:       "value2",
	}
	configItems = append(configItems, it2)
	
	testReq := &model.ApplicationConfigGroup{
		AppID:           appID,
		ConfigGroupName: configName,
		DeployType:      "env",
		ServiceIDs:      []string{"sid1"},
		ConfigItems:     configItems,
	}

	var serviceResult []dbmodel.ServiceIDAndNameResult
	sReslt := dbmodel.ServiceIDAndNameResult{
		ServiceID:   "sid1",
		ServiceName: "sid1_name",
	}
	serviceResult = append(serviceResult, sReslt)

	applicationConfigGroup := &dbmodel.ApplicationConfigGroup{
		AppID:           testReq.AppID,
		ConfigGroupName: testReq.ConfigGroupName,
		DeployType:      testReq.DeployType,
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	manager := db.NewMockManager(ctrl)
	db.SetTestManager(manager)

	serviceConfigGroupDao := daomock.NewMockServiceConfigGroupDao(ctrl)
	serviceConfigGroupDao.EXPECT().AddModel(serviceConfigGroupReq).Return(nil).AnyTimes()
	manager.EXPECT().ServiceConfigGroupDao().Return(serviceConfigGroupDao).AnyTimes()

	configItemDao := daomock.NewMockConfigItemDao(ctrl)
	configItemDao.EXPECT().AddModel(gomock.Any()).Return(nil).AnyTimes()
	manager.EXPECT().ConfigItemDao().Return(configItemDao).AnyTimes()

	applicationConfigDao := daomock.NewMockApplicationConfigDao(ctrl)
	applicationConfigDao.EXPECT().AddModel(configReq).Return(nil)
	applicationConfigDao.EXPECT().GetConfigByID(appID, configName).Return(applicationConfigGroup, nil)
	manager.EXPECT().ApplicationConfigDao().Return(applicationConfigDao).AnyTimes()

	tenantServiceDao := daomock.NewMockTenantServiceDao(ctrl)
	tenantServiceDao.EXPECT().GetServicesIDAndNameByAppID(appID).Return(serviceResult)
	manager.EXPECT().TenantServiceDao().Return(tenantServiceDao)

	appAction := NewApplicationHandler()
	resp, _ := appAction.AddConfigGroup(appID, testReq)
	assert.Equal(t, applicationConfigGroup.AppID, testReq.AppID)

	for _, serviceResp := range resp.Services {
		for _, expService := range serviceResult {
			assert.Equal(t, expService.ServiceName, serviceResp.ServiceName)
		}
	}
}

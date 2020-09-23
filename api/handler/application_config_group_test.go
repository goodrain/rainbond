package handler

import (
	"testing"

	"github.com/go-playground/assert/v2"
	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	daomock "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/pkg/errors"
)

func TestAddAppConfigGroup(t *testing.T) {
	tests := []struct {
		name     string
		appID    string
		request  *model.ApplicationConfigGroup
		mockFunc func(manager *db.MockManager, ctrl *gomock.Controller)
		wanterr  bool
	}{
		{
			name:  "add config group success",
			appID: "appID1",
			request: &model.ApplicationConfigGroup{
				ConfigGroupName: "configName1",
				DeployType:      "env",
				ServiceIDs:      []string{"sid1"},
				ConfigItems: []model.ConfigItem{
					{ItemKey: "key1", ItemValue: "value1"},
					{ItemKey: "key2", ItemValue: "value2"},
				},
			},
			mockFunc: func(manager *db.MockManager, ctrl *gomock.Controller) {
				serviceResult := []dbmodel.ServiceIDAndNameResult{
					{ServiceID: "sid1", ServiceName: "sid1_name"},
				}
				config := &dbmodel.ApplicationConfigGroup{
					AppID:           "appID1",
					ConfigGroupName: "configName1",
					DeployType:      "env",
				}
				serviceConfigGroupDao := daomock.NewMockServiceConfigGroupDao(ctrl)
				serviceConfigGroupDao.EXPECT().AddModel(gomock.Any()).Return(nil).AnyTimes()
				manager.EXPECT().ServiceConfigGroupDao().Return(serviceConfigGroupDao).AnyTimes()

				configItemDao := daomock.NewMockConfigItemDao(ctrl)
				configItemDao.EXPECT().AddModel(gomock.Any()).Return(nil).AnyTimes()
				manager.EXPECT().ConfigItemDao().Return(configItemDao).AnyTimes()

				applicationConfigDao := daomock.NewMockApplicationConfigDao(ctrl)
				applicationConfigDao.EXPECT().AddModel(gomock.Any()).Return(nil)
				applicationConfigDao.EXPECT().GetConfigByID(gomock.Any(), gomock.Any()).Return(config, nil)
				manager.EXPECT().ApplicationConfigDao().Return(applicationConfigDao).AnyTimes()

				tenantServiceDao := daomock.NewMockTenantServiceDao(ctrl)
				tenantServiceDao.EXPECT().GetServicesIDAndNameByAppID(gomock.Any()).Return(serviceResult)
				manager.EXPECT().TenantServiceDao().Return(tenantServiceDao)
			},
			wanterr: false,
		},
		{
			name:  "add config group service failed",
			appID: "appID1",
			request: &model.ApplicationConfigGroup{
				ConfigGroupName: "configName1",
				DeployType:      "env",
				ServiceIDs:      []string{"sid1"},
				ConfigItems: []model.ConfigItem{
					{ItemKey: "key1", ItemValue: "value1"},
					{ItemKey: "key2", ItemValue: "value2"},
				},
			},
			mockFunc: func(manager *db.MockManager, ctrl *gomock.Controller) {
				serviceConfigGroupDao := daomock.NewMockServiceConfigGroupDao(ctrl)
				serviceConfigGroupDao.EXPECT().AddModel(gomock.Any()).Return(errors.New("add service config failed")).AnyTimes()
				manager.EXPECT().ServiceConfigGroupDao().Return(serviceConfigGroupDao).AnyTimes()
			},
			wanterr: true,
		},
		{
			name:  "add config item failed",
			appID: "appID1",
			request: &model.ApplicationConfigGroup{
				ConfigGroupName: "configName1",
				DeployType:      "env",
				ServiceIDs:      []string{"sid1"},
				ConfigItems: []model.ConfigItem{
					{ItemKey: "key1", ItemValue: "value1"},
					{ItemKey: "key2", ItemValue: "value2"},
				},
			},
			mockFunc: func(manager *db.MockManager, ctrl *gomock.Controller) {
				serviceConfigGroupDao := daomock.NewMockServiceConfigGroupDao(ctrl)
				serviceConfigGroupDao.EXPECT().AddModel(gomock.Any()).Return(nil).AnyTimes()
				manager.EXPECT().ServiceConfigGroupDao().Return(serviceConfigGroupDao).AnyTimes()

				configItemDao := daomock.NewMockConfigItemDao(ctrl)
				configItemDao.EXPECT().AddModel(gomock.Any()).Return(errors.New("add config item failed")).AnyTimes()
				manager.EXPECT().ConfigItemDao().Return(configItemDao).AnyTimes()
			},
			wanterr: true,
		},
		{
			name:  "add application config group failed",
			appID: "appID1",
			request: &model.ApplicationConfigGroup{
				ConfigGroupName: "configName1",
				DeployType:      "env",
				ServiceIDs:      []string{"sid1"},
				ConfigItems: []model.ConfigItem{
					{ItemKey: "key1", ItemValue: "value1"},
					{ItemKey: "key2", ItemValue: "value2"},
				},
			},
			mockFunc: func(manager *db.MockManager, ctrl *gomock.Controller) {
				serviceConfigGroupDao := daomock.NewMockServiceConfigGroupDao(ctrl)
				serviceConfigGroupDao.EXPECT().AddModel(gomock.Any()).Return(nil).AnyTimes()
				manager.EXPECT().ServiceConfigGroupDao().Return(serviceConfigGroupDao).AnyTimes()

				configItemDao := daomock.NewMockConfigItemDao(ctrl)
				configItemDao.EXPECT().AddModel(gomock.Any()).Return(nil).AnyTimes()
				manager.EXPECT().ConfigItemDao().Return(configItemDao).AnyTimes()

				applicationConfigDao := daomock.NewMockApplicationConfigDao(ctrl)
				applicationConfigDao.EXPECT().AddModel(gomock.Any()).Return(errors.New("add application config group failed"))
				manager.EXPECT().ApplicationConfigDao().Return(applicationConfigDao).AnyTimes()
			},
			wanterr: true,
		},
	}
	for i := range tests {
		tc := tests[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			manager := db.NewMockManager(ctrl)
			db.SetTestManager(manager)
			tc.mockFunc(manager, ctrl)

			appAction := NewApplicationHandler()
			resp, err := appAction.AddConfigGroup(tc.appID, tc.request)
			if (err != nil) != tc.wanterr {
				t.Errorf("Unexpected error = %v, wantErr %v", err, tc.wanterr)
				return
			}
			if resp != nil {
				assert.Equal(t, resp.AppID, tc.appID)
				assert.Equal(t, resp.DeployType, tc.request.DeployType)
			}
		})
	}

}

package handler

//
//import (
//	"testing"
//
//	"github.com/go-playground/assert/v2"
//	"github.com/golang/mock/gomock"
//	"github.com/goodrain/rainbond/api/model"
//	"github.com/goodrain/rainbond/db"
//	daomock "github.com/goodrain/rainbond/db/dao"
//	dbmodel "github.com/goodrain/rainbond/db/model"
//	"github.com/pkg/errors"
//)
//
//func TestAddAppConfigGroup(t *testing.T) {
//	tests := []struct {
//		name     string
//		appID    string
//		request  *model.ApplicationConfigGroup
//		mockFunc func(manager *db.MockManager, ctrl *gomock.Controller)
//		wanterr  bool
//	}{
//		{
//			name:  "add config group success",
//			appID: "appID1",
//			request: &model.ApplicationConfigGroup{
//				ConfigGroupName: "configName1",
//				DeployType:      "env",
//				ServiceIDs:      []string{"sid1"},
//				ConfigItems: []model.ConfigItem{
//					{ItemKey: "key1", ItemValue: "value1"},
//					{ItemKey: "key2", ItemValue: "value2"},
//				},
//			},
//			mockFunc: func(manager *db.MockManager, ctrl *gomock.Controller) {
//				serviceResult := []*dbmodel.TenantServices{
//					{ServiceID: "sid1", ServiceAlias: "sid1_name"},
//				}
//				config := &dbmodel.ApplicationConfigGroup{
//					AppID:           "appID1",
//					ConfigGroupName: "configName1",
//					DeployType:      "env",
//				}
//				tenantServiceDao := daomock.NewMockTenantServiceDao(ctrl)
//				tenantServiceDao.EXPECT().GetServicesByServiceIDs(gomock.Any()).Return(serviceResult, nil)
//				manager.EXPECT().TenantServiceDao().Return(tenantServiceDao)
//
//				serviceConfigGroupDao := daomock.NewMockAppConfigGroupServiceDao(ctrl)
//				serviceConfigGroupDao.EXPECT().AddModel(gomock.Any()).Return(nil).AnyTimes()
//				manager.EXPECT().AppConfigGroupServiceDao().Return(serviceConfigGroupDao).AnyTimes()
//
//				configItemDao := daomock.NewMockAppConfigGroupItemDao(ctrl)
//				configItemDao.EXPECT().AddModel(gomock.Any()).Return(nil).AnyTimes()
//				manager.EXPECT().AppConfigGroupItemDao().Return(configItemDao).AnyTimes()
//
//				applicationConfigDao := daomock.NewMockAppConfigGroupDao(ctrl)
//				applicationConfigDao.EXPECT().AddModel(gomock.Any()).Return(nil)
//				applicationConfigDao.EXPECT().GetConfigGroupByID(gomock.Any(), gomock.Any()).Return(config, nil)
//				manager.EXPECT().AppConfigGroupDao().Return(applicationConfigDao).AnyTimes()
//			},
//			wanterr: false,
//		},
//		{
//			name:  "add config group service failed",
//			appID: "appID1",
//			request: &model.ApplicationConfigGroup{
//				ConfigGroupName: "configName1",
//				DeployType:      "env",
//				ServiceIDs:      []string{"sid1"},
//				ConfigItems: []model.ConfigItem{
//					{ItemKey: "key1", ItemValue: "value1"},
//					{ItemKey: "key2", ItemValue: "value2"},
//				},
//			},
//			mockFunc: func(manager *db.MockManager, ctrl *gomock.Controller) {
//				serviceResult := []*dbmodel.TenantServices{
//					{ServiceID: "sid1", ServiceAlias: "sid1_name"},
//				}
//				tenantServiceDao := daomock.NewMockTenantServiceDao(ctrl)
//				tenantServiceDao.EXPECT().GetServicesByServiceIDs(gomock.Any()).Return(serviceResult, nil)
//				manager.EXPECT().TenantServiceDao().Return(tenantServiceDao)
//
//				serviceConfigGroupDao := daomock.NewMockAppConfigGroupServiceDao(ctrl)
//				serviceConfigGroupDao.EXPECT().AddModel(gomock.Any()).Return(errors.New("add service config failed")).AnyTimes()
//				manager.EXPECT().AppConfigGroupServiceDao().Return(serviceConfigGroupDao).AnyTimes()
//			},
//			wanterr: true,
//		},
//		{
//			name:  "add config item failed",
//			appID: "appID1",
//			request: &model.ApplicationConfigGroup{
//				ConfigGroupName: "configName1",
//				DeployType:      "env",
//				ServiceIDs:      []string{"sid1"},
//				ConfigItems: []model.ConfigItem{
//					{ItemKey: "key1", ItemValue: "value1"},
//					{ItemKey: "key2", ItemValue: "value2"},
//				},
//			},
//			mockFunc: func(manager *db.MockManager, ctrl *gomock.Controller) {
//				serviceResult := []*dbmodel.TenantServices{
//					{ServiceID: "sid1", ServiceAlias: "sid1_name"},
//				}
//				tenantServiceDao := daomock.NewMockTenantServiceDao(ctrl)
//				tenantServiceDao.EXPECT().GetServicesByServiceIDs(gomock.Any()).Return(serviceResult, nil)
//				manager.EXPECT().TenantServiceDao().Return(tenantServiceDao)
//
//				serviceConfigGroupDao := daomock.NewMockAppConfigGroupServiceDao(ctrl)
//				serviceConfigGroupDao.EXPECT().AddModel(gomock.Any()).Return(nil).AnyTimes()
//				manager.EXPECT().AppConfigGroupServiceDao().Return(serviceConfigGroupDao).AnyTimes()
//
//				configItemDao := daomock.NewMockAppConfigGroupItemDao(ctrl)
//				configItemDao.EXPECT().AddModel(gomock.Any()).Return(errors.New("add config item failed")).AnyTimes()
//				manager.EXPECT().AppConfigGroupItemDao().Return(configItemDao).AnyTimes()
//			},
//			wanterr: true,
//		},
//		{
//			name:  "add application config group failed",
//			appID: "appID1",
//			request: &model.ApplicationConfigGroup{
//				ConfigGroupName: "configName1",
//				DeployType:      "env",
//				ServiceIDs:      []string{"sid1"},
//				ConfigItems: []model.ConfigItem{
//					{ItemKey: "key1", ItemValue: "value1"},
//					{ItemKey: "key2", ItemValue: "value2"},
//				},
//			},
//			mockFunc: func(manager *db.MockManager, ctrl *gomock.Controller) {
//				serviceResult := []*dbmodel.TenantServices{
//					{ServiceID: "sid1", ServiceAlias: "sid1_name"},
//				}
//				tenantServiceDao := daomock.NewMockTenantServiceDao(ctrl)
//				tenantServiceDao.EXPECT().GetServicesByServiceIDs(gomock.Any()).Return(serviceResult, nil)
//				manager.EXPECT().TenantServiceDao().Return(tenantServiceDao)
//
//				serviceConfigGroupDao := daomock.NewMockAppConfigGroupServiceDao(ctrl)
//				serviceConfigGroupDao.EXPECT().AddModel(gomock.Any()).Return(nil).AnyTimes()
//				manager.EXPECT().AppConfigGroupServiceDao().Return(serviceConfigGroupDao).AnyTimes()
//
//				configItemDao := daomock.NewMockAppConfigGroupItemDao(ctrl)
//				configItemDao.EXPECT().AddModel(gomock.Any()).Return(nil).AnyTimes()
//				manager.EXPECT().AppConfigGroupItemDao().Return(configItemDao).AnyTimes()
//
//				applicationConfigDao := daomock.NewMockAppConfigGroupDao(ctrl)
//				applicationConfigDao.EXPECT().AddModel(gomock.Any()).Return(errors.New("add application config group failed"))
//				manager.EXPECT().AppConfigGroupDao().Return(applicationConfigDao).AnyTimes()
//			},
//			wanterr: true,
//		},
//	}
//	for i := range tests {
//		tc := tests[i]
//		t.Run(tc.name, func(t *testing.T) {
//			ctrl := gomock.NewController(t)
//			defer ctrl.Finish()
//
//			manager := db.NewMockManager(ctrl)
//			db.SetTestManager(manager)
//			tc.mockFunc(manager, ctrl)
//
//			appAction := ApplicationAction{}
//			resp, err := appAction.AddConfigGroup(tc.appID, tc.request)
//			if (err != nil) != tc.wanterr {
//				t.Errorf("Unexpected error = %v, wantErr %v", err, tc.wanterr)
//				return
//			}
//			if resp != nil {
//				assert.Equal(t, resp.AppID, tc.appID)
//				assert.Equal(t, resp.DeployType, tc.request.DeployType)
//			}
//		})
//	}
//
//}
//
//func TestListConfigGroups(t *testing.T) {
//	tests := []struct {
//		name     string
//		appID    string
//		request  *model.ApplicationConfigGroup
//		mockFunc func(manager *db.MockManager, ctrl *gomock.Controller)
//		wanterr  bool
//	}{
//		{
//			name:  "list config group success",
//			appID: "appID1",
//			request: &model.ApplicationConfigGroup{
//				ConfigGroupName: "configName1",
//				DeployType:      "env",
//				ServiceIDs:      []string{"sid1"},
//				ConfigItems: []model.ConfigItem{
//					{ItemKey: "key1", ItemValue: "value1"},
//					{ItemKey: "key2", ItemValue: "value2"},
//				},
//			},
//			mockFunc: func(manager *db.MockManager, ctrl *gomock.Controller) {
//				configGroupsServiceResult := []*dbmodel.ConfigGroupService{
//					{ServiceID: "sid1", ServiceAlias: "sid1_name"},
//				}
//				configGroupsResult := []*dbmodel.ApplicationConfigGroup{
//					{AppID: "appID1", ConfigGroupName: "configName1", DeployType: "env"},
//				}
//				configGroupItemResult := []*dbmodel.ConfigGroupItem{
//					{ItemKey: "itemKey1", ItemValue: "itemValue1"},
//				}
//				applicationConfigDao := daomock.NewMockAppConfigGroupDao(ctrl)
//				applicationConfigDao.EXPECT().GetConfigGroupsByAppID(gomock.Any(), gomock.Any(), gomock.Any()).Return(configGroupsResult, int64(1), nil)
//				manager.EXPECT().AppConfigGroupDao().Return(applicationConfigDao).AnyTimes()
//
//				serviceConfigGroupDao := daomock.NewMockAppConfigGroupServiceDao(ctrl)
//				serviceConfigGroupDao.EXPECT().GetConfigGroupServicesByID(gomock.Any(), gomock.Any()).Return(configGroupsServiceResult, nil).AnyTimes()
//				manager.EXPECT().AppConfigGroupServiceDao().Return(serviceConfigGroupDao).AnyTimes()
//
//				configItemDao := daomock.NewMockAppConfigGroupItemDao(ctrl)
//				configItemDao.EXPECT().GetConfigGroupItemsByID(gomock.Any(), gomock.Any()).Return(configGroupItemResult, nil).AnyTimes()
//				manager.EXPECT().AppConfigGroupItemDao().Return(configItemDao).AnyTimes()
//			},
//			wanterr: false,
//		},
//		{
//			name:  "list config group failed because get config group error",
//			appID: "appID1",
//			mockFunc: func(manager *db.MockManager, ctrl *gomock.Controller) {
//				applicationConfigDao := daomock.NewMockAppConfigGroupDao(ctrl)
//				applicationConfigDao.EXPECT().GetConfigGroupsByAppID(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), errors.New("get config group error"))
//				manager.EXPECT().AppConfigGroupDao().Return(applicationConfigDao).AnyTimes()
//			},
//			wanterr: true,
//		},
//		{
//			name:  "list config group failed because get config group service error",
//			appID: "appID1",
//			mockFunc: func(manager *db.MockManager, ctrl *gomock.Controller) {
//				configGroupsResult := []*dbmodel.ApplicationConfigGroup{
//					{AppID: "appID1", ConfigGroupName: "configName1", DeployType: "env"},
//				}
//				applicationConfigDao := daomock.NewMockAppConfigGroupDao(ctrl)
//				applicationConfigDao.EXPECT().GetConfigGroupsByAppID(gomock.Any(), gomock.Any(), gomock.Any()).Return(configGroupsResult, int64(1), nil)
//				manager.EXPECT().AppConfigGroupDao().Return(applicationConfigDao).AnyTimes()
//
//				serviceConfigGroupDao := daomock.NewMockAppConfigGroupServiceDao(ctrl)
//				serviceConfigGroupDao.EXPECT().GetConfigGroupServicesByID(gomock.Any(), gomock.Any()).Return(nil, errors.New("get config group service error")).AnyTimes()
//				manager.EXPECT().AppConfigGroupServiceDao().Return(serviceConfigGroupDao).AnyTimes()
//			},
//			wanterr: true,
//		},
//		{
//			name:  "list config group failed because get config group item error",
//			appID: "appID1",
//			mockFunc: func(manager *db.MockManager, ctrl *gomock.Controller) {
//				configGroupsServiceResult := []*dbmodel.ConfigGroupService{
//					{ServiceID: "sid1", ServiceAlias: "sid1_name"},
//				}
//				configGroupsResult := []*dbmodel.ApplicationConfigGroup{
//					{AppID: "appID1", ConfigGroupName: "configName1", DeployType: "env"},
//				}
//				applicationConfigDao := daomock.NewMockAppConfigGroupDao(ctrl)
//				applicationConfigDao.EXPECT().GetConfigGroupsByAppID(gomock.Any(), gomock.Any(), gomock.Any()).Return(configGroupsResult, int64(1), nil)
//				manager.EXPECT().AppConfigGroupDao().Return(applicationConfigDao).AnyTimes()
//
//				serviceConfigGroupDao := daomock.NewMockAppConfigGroupServiceDao(ctrl)
//				serviceConfigGroupDao.EXPECT().GetConfigGroupServicesByID(gomock.Any(), gomock.Any()).Return(configGroupsServiceResult, nil).AnyTimes()
//				manager.EXPECT().AppConfigGroupServiceDao().Return(serviceConfigGroupDao).AnyTimes()
//
//				configItemDao := daomock.NewMockAppConfigGroupItemDao(ctrl)
//				configItemDao.EXPECT().GetConfigGroupItemsByID(gomock.Any(), gomock.Any()).Return(nil, errors.New("get config group item error")).AnyTimes()
//				manager.EXPECT().AppConfigGroupItemDao().Return(configItemDao).AnyTimes()
//			},
//			wanterr: true,
//		},
//	}
//	for i := range tests {
//		tc := tests[i]
//		t.Run(tc.name, func(t *testing.T) {
//			ctrl := gomock.NewController(t)
//			defer ctrl.Finish()
//
//			manager := db.NewMockManager(ctrl)
//			db.SetTestManager(manager)
//			tc.mockFunc(manager, ctrl)
//
//			appAction := NewApplicationHandler(nil, nil, nil, nil)
//			resp, err := appAction.ListConfigGroups(tc.appID, 1, 10)
//			if (err != nil) != tc.wanterr {
//				t.Errorf("Unexpected error = %v, wantErr %v", err, tc.wanterr)
//				return
//			}
//			if resp != nil {
//				for _, r := range resp.ConfigGroup {
//					assert.Equal(t, r.AppID, tc.appID)
//					assert.Equal(t, r.ConfigGroupName, tc.request.ConfigGroupName)
//					assert.Equal(t, r.DeployType, tc.request.DeployType)
//					got := false
//					for _, s := range r.Services {
//						if s.ServiceID == tc.request.ServiceIDs[0] {
//							got = true
//						}
//					}
//					if got != true {
//						t.Errorf("get config group service error,serviceID not exists")
//					}
//				}
//			}
//		})
//	}
//
//}

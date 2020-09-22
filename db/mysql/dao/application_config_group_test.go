package dao

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

func TestAppConfigDaoAddModel(t *testing.T) {
	req := &model.ApplicationConfigGroup{
		AppID:           "appID",
		ConfigGroupName: "configname",
	}
	tests := []struct {
		name     string
		request  *model.ApplicationConfigGroup
		mockFunc func(mock sqlmock.Sqlmock)
		wanterr  bool
	}{
		{
			name:    "config group exists,return err",
			request: req,
			mockFunc: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.
					NewRows([]string{"app_id", "config_group_name"}).
					AddRow("ID1", "Name1")
				mock.ExpectQuery("SELECT").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnRows(rows).WillReturnError(nil)
			},
			wanterr: true,
		},
		{
			name:    "config group not found,create success",
			request: req,
			mockFunc: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `application_config_group` WHERE (app_id = ? AND config_group_name = ?)")).WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnError(gorm.ErrRecordNotFound)
				mock.ExpectBegin()
				mock.ExpectExec("INSERT").WillReturnResult(sqlmock.NewResult(1, 1)).WillReturnError(nil)
				mock.ExpectCommit()
			},
			wanterr: false,
		},
		{
			name:    "database error",
			request: req,
			mockFunc: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnError(gorm.ErrRecordNotFound)
				mock.ExpectBegin()
				mock.ExpectExec("INSERT").WillReturnError(errors.New("database error"))
				mock.ExpectRollback()
			},
			wanterr: true,
		},
	}

	for i := range tests {
		tc := tests[i]
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer db.Close()

			gdb, _ := gorm.Open("mysql", db)
			appConfigDaoImpl := &ApplicationConfigDaoImpl{
				DB: gdb,
			}
			tc.mockFunc(mock)

			req := &model.ApplicationConfigGroup{
				AppID:           "appID",
				ConfigGroupName: "configname",
			}
			err = appConfigDaoImpl.AddModel(req)
			if (err != nil) != tc.wanterr {
				t.Errorf("Unexpected error = %v, wantErr %v", err, tc.wanterr)
				return
			}
		})
	}
}

func TestAppGetConfigByID(t *testing.T) {
	tests := []struct {
		name     string
		appID    string
		mockFunc func(mock sqlmock.Sqlmock)
		wanterr  bool
	}{
		{
			name:  "get config success",
			appID: "ID1",
			mockFunc: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.
					NewRows([]string{"app_id", "config_group_name"}).
					AddRow("ID1", "Name1")
				mock.ExpectQuery("SELECT").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnRows(rows).WillReturnError(nil)
			},
			wanterr: false,
		},
		{
			name: "get config failed",
			mockFunc: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnError(errors.New("query failed"))
			},
			wanterr: true,
		},
	}

	for i := range tests {
		tc := tests[i]
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer db.Close()

			gdb, _ := gorm.Open("mysql", db)
			appConfigDaoImpl := &ApplicationConfigDaoImpl{
				DB: gdb,
			}
			tc.mockFunc(mock)

			resp, err := appConfigDaoImpl.GetConfigByID("appID", "name")
			if (err != nil) != tc.wanterr {
				t.Errorf("Unexpected error = %v, wantErr %v", err, tc.wanterr)
				return
			}
			if resp != nil && resp.AppID != tc.appID {
				t.Errorf("reponse app_id should equal %v, but got %v", resp.AppID, tc.appID)
				return
			}
		})
	}
}

func TestServiceConfigGroupDaoAddModel(t *testing.T) {
	req := &model.ServiceConfigGroup{
		AppID:           "appID",
		ConfigGroupName: "configname",
	}
	tests := []struct {
		name     string
		request  *model.ServiceConfigGroup
		mockFunc func(mock sqlmock.Sqlmock)
		wanterr  bool
	}{
		{
			name:    "service config group exists,return err",
			request: req,
			mockFunc: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.
					NewRows([]string{"app_id", "config_group_name", "service_id"}).
					AddRow("ID1", "Name1", "serviceID1")
				mock.ExpectQuery("SELECT").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnRows(rows).WillReturnError(nil)
			},
			wanterr: true,
		},
		{
			name:    "service config group not found,create success",
			request: req,
			mockFunc: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnError(gorm.ErrRecordNotFound)
				mock.ExpectBegin()
				mock.ExpectExec("INSERT").WillReturnResult(sqlmock.NewResult(1, 1)).WillReturnError(nil)
				mock.ExpectCommit()
			},
			wanterr: false,
		},
		{
			name:    "database error",
			request: req,
			mockFunc: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnError(gorm.ErrRecordNotFound)
				mock.ExpectExec("INSERT").WillReturnError(errors.New("database error"))
				mock.ExpectRollback()
			},
			wanterr: true,
		},
	}

	for i := range tests {
		tc := tests[i]
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer db.Close()

			gdb, _ := gorm.Open("mysql", db)
			serviceConfigGroupDao := &ServiceConfigGroupDaoImpl{
				DB: gdb,
			}
			tc.mockFunc(mock)

			req := &model.ServiceConfigGroup{
				AppID:           "appID",
				ConfigGroupName: "configname",
				ServiceID:       "serviceID",
			}
			err = serviceConfigGroupDao.AddModel(req)
			if (err != nil) != tc.wanterr {
				t.Errorf("Unexpected error = %v, wantErr %v", err, tc.wanterr)
				return
			}
		})
	}
}

func TestConfigItemDaoAddModel(t *testing.T) {
	req := &model.ConfigItem{
		AppID:           "appID",
		ConfigGroupName: "configname",
		ItemKey:         "key1",
	}
	tests := []struct {
		name     string
		request  *model.ConfigItem
		mockFunc func(mock sqlmock.Sqlmock)
		wanterr  bool
	}{
		{
			name:    "config item exists,return err",
			request: req,
			mockFunc: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.
					NewRows([]string{"app_id", "config_group_name", "item_key"}).
					AddRow("ID1", "Name1", "key1")
				mock.ExpectQuery("SELECT").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnRows(rows).WillReturnError(nil)
			},
			wanterr: true,
		},
		{
			name:    "config item not found,create success",
			request: req,
			mockFunc: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnError(gorm.ErrRecordNotFound)
				mock.ExpectBegin()
				mock.ExpectExec("INSERT").WillReturnResult(sqlmock.NewResult(1, 1)).WillReturnError(nil)
				mock.ExpectCommit()
			},
			wanterr: false,
		},
		{
			name:    "database error",
			request: req,
			mockFunc: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnError(errors.New("database error"))
				mock.ExpectExec("INSERT").WillReturnError(errors.New("database error"))
				mock.ExpectRollback()
			},
			wanterr: true,
		},
	}

	for i := range tests {
		tc := tests[i]
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer db.Close()

			gdb, _ := gorm.Open("mysql", db)
			configItemDaoImpl := &ConfigItemDaoImpl{
				DB: gdb,
			}
			tc.mockFunc(mock)

			req := &model.ConfigItem{
				AppID:           "appID",
				ConfigGroupName: "configname",
				ItemKey:         "key1",
			}
			err = configItemDaoImpl.AddModel(req)
			if (err != nil) != tc.wanterr {
				t.Errorf("Unexpected error = %v, wantErr %v", err, tc.wanterr)
				return
			}
		})
	}
}

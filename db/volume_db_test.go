// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package db

import (
	"github.com/jinzhu/gorm"
	dbconfig "github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	"testing"
)

func TestManager_TenantServiceConfigFileDaoImpl_UpdateModel(t *testing.T) {
	if err := CreateManager(dbconfig.Config{
		DBType: "sqlite3",
	}); err != nil {
		t.Fatal(err)
	}
	tx := GetManager().Begin()
	tx.Delete(model.TenantServiceConfigFile{})
	tx.Commit()

	vname := util.NewUUID()
	cf := &model.TenantServiceConfigFile{
		UUID:        util.NewUUID(),
		VolumeName:  vname,
		FileContent: "dummy file content",
	}
	if err := GetManager().TenantServiceConfigFileDao().AddModel(cf); err != nil {
		t.Fatal(err)
	}
	cf, err := GetManager().TenantServiceConfigFileDao().GetByVolumeName(vname)
	if err != nil {
		t.Fatal(err)
	}
	if cf == nil {
		t.Errorf("Expected one config file, but returned %v", cf)
	}

	if err := GetManager().TenantServiceConfigFileDao().DelByVolumeID(vname); err != nil {
		t.Fatal(err)
	}
	cf, err = GetManager().TenantServiceConfigFileDao().GetByVolumeName(vname)
	if err != nil && err != gorm.ErrRecordNotFound {
		t.Fatal(err)
	}
	if cf != nil {
		t.Errorf("Expected nothing for cfs, but returned %v", cf)
	}
}

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

	vid := util.NewUUID()
	cf := &model.TenantServiceConfigFile{
		UUID:        util.NewUUID(),
		VolumeName:  vid,
		Filename:    "dummy_filename",
		FileContent: "dummy file content",
	}
	if err := GetManager().TenantServiceConfigFileDao().AddModel(cf); err != nil {
		t.Fatal(err)
	}
	cfs, err := GetManager().TenantServiceConfigFileDao().ListByVolumeID(vid)
	if err != nil {
		t.Fatal(err)
	}
	if cfs == nil || len(cfs) != 1 {
		t.Errorf("Expected one config file, but returned %v", cfs)
	}

	if err := GetManager().TenantServiceConfigFileDao().DelByVolumeID(vid); err != nil {
		t.Fatal(err)
	}
	cfs, err = GetManager().TenantServiceConfigFileDao().ListByVolumeID(vid)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfs) != 0 {
		t.Errorf("Expected nothing for cfs, but returned %v", cfs)
	}
}

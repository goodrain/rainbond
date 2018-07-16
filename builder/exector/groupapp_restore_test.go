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

package exector

import (
	"fmt"
	"testing"

	"github.com/pquerna/ffjson/ffjson"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"

	"github.com/goodrain/rainbond/util"
)

func TestDownloadFromLocal(t *testing.T) {
	var b = BackupAPPRestore{
		BackupID: "test",
		EventID:  "test",
		Logger:   event.GetTestLogger(),
	}
	cacheDir := fmt.Sprintf("/tmp/%s/%s", b.BackupID, b.EventID)
	if err := util.CheckAndCreateDir(cacheDir); err != nil {
		t.Fatal("create cache dir error", err.Error())
	}
	b.cacheDir = cacheDir
	if err := b.downloadFromFTP(&dbmodel.AppBackup{
		EventID:  "test",
		BackupID: "ccc",
	}); err != nil {
		t.Fatal("downloadFromLocal error", err.Error())
	}
	t.Log(b.cacheDir)
}

func TestModify(t *testing.T) {
	var b = BackupAPPRestore{
		BackupID:      "test",
		EventID:       "test",
		serviceChange: make(map[string]*Info, 0),
		Logger:        event.GetTestLogger(),
	}
	var appSnapshots = []*RegionServiceSnapshot{
		&RegionServiceSnapshot{
			ServiceID: "1234",
			Service: &dbmodel.TenantServices{
				ServiceID: "1234",
			},
			ServiceMntRelation: []*dbmodel.TenantServiceMountRelation{
				&dbmodel.TenantServiceMountRelation{
					ServiceID:       "1234",
					DependServiceID: "123456",
				},
			},
		},
		&RegionServiceSnapshot{
			ServiceID: "123456",
			Service: &dbmodel.TenantServices{
				ServiceID: "1234",
			},
			ServiceEnv: []*dbmodel.TenantServiceEnvVar{
				&dbmodel.TenantServiceEnvVar{
					ServiceID: "123456",
					Name:      "testenv",
				},
				&dbmodel.TenantServiceEnvVar{
					ServiceID: "123456",
					Name:      "testenv2",
				},
			},
		},
	}
	b.modify(appSnapshots)
	re, _ := ffjson.Marshal(appSnapshots)
	t.Log(string(re))
}

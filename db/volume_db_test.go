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
	"context"
	"fmt"
	dbconfig "github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	"github.com/jinzhu/gorm"
	"github.com/testcontainers/testcontainers-go"
	"testing"
	"time"
)

func TestManager_TenantServiceConfigFileDaoImpl_UpdateModel(t *testing.T) {
	dbname := "region"
	rootpw := "rainbond"

	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "mariadb",
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": rootpw,
			"MYSQL_DATABASE":      dbname,
		},
		Cmd: []string{"character-set-server=utf8mb4", "collation-server=utf8mb4_unicode_ci"},
	}
	mariadb, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer mariadb.Terminate(ctx)

	host, err := mariadb.Host(ctx)
	if err != nil {
		t.Error(err)
	}
	port, err := mariadb.MappedPort(ctx, "3306")
	if err != nil {
		t.Error(err)
	}

	connInfo := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", "root",
		rootpw, host, port.Int(), dbname)
	tryTimes := 3
	for {
		if err := CreateManager(dbconfig.Config{
			DBType:              "mysql",
			MysqlConnectionInfo: connInfo,
		}); err != nil {
			if tryTimes == 0 {
				t.Fatalf("Connect info: %s; error creating db manager: %v", connInfo, err)
			} else {
				tryTimes = tryTimes - 1
				time.Sleep(10 * time.Second)
				continue
			}
		}
		break
	}

	cf := &model.TenantServiceConfigFile{
		ServiceID:   util.NewUUID(),
		VolumeName:  util.NewUUID(),
		FileContent: "dummy file content",
	}
	if err := GetManager().TenantServiceConfigFileDao().AddModel(cf); err != nil {
		t.Fatal(err)
	}
	cf, err = GetManager().TenantServiceConfigFileDao().GetByVolumeName(cf.ServiceID, cf.VolumeName)
	if err != nil {
		t.Fatal(err)
	}
	if cf == nil {
		t.Errorf("Expected one config file, but returned %v", cf)
	}

	if err := GetManager().TenantServiceConfigFileDao().DelByVolumeID(cf.ServiceID, cf.VolumeName); err != nil {
		t.Fatal(err)
	}
	cf, err = GetManager().TenantServiceConfigFileDao().GetByVolumeName(cf.ServiceID, cf.VolumeName)
	if err != nil && err != gorm.ErrRecordNotFound {
		t.Fatal(err)
	}
	if cf != nil {
		t.Errorf("Expected nothing for cfs, but returned %v", cf)
	}
}

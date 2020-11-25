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
	"github.com/testcontainers/testcontainers-go"
	"testing"
	"time"
)

func TestManager_PluginBuildVersionDaoImpl_ListSuccessfulOnesByPluginIDs(t *testing.T) {
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

	// prepare test data
	oridata := []struct {
		pluginID, status string
	}{
		{pluginID: "ff6aad8a70324384a7578285799e50d9", status: "complete"},
		{pluginID: "ff6aad8a70324384a7578285799e50d9", status: "failure"},
		{pluginID: "ff6aad8a70324384a7578285799e50d9", status: "failure"},
		{pluginID: "4998ca78d41f45149e71c1f03ad0aa22", status: "complete"},
	}
	for _, od := range oridata {
		buildVersion := &model.TenantPluginBuildVersion{
			PluginID:      od.pluginID,
			Status:        od.status,
			DeployVersion: time.Now().Format("20060102150405.000000"),
		}
		if err := GetManager().TenantPluginBuildVersionDao().AddModel(buildVersion); err != nil {
			t.Fatalf("failed to create plugin build version: %v", err)
		}
	}

	pluginIDs := []string{"ff6aad8a70324384a7578285799e50d9", "4998ca78d41f45149e71c1f03ad0aa22"}
	verions, err := GetManager().TenantPluginBuildVersionDao().ListSuccessfulOnesByPluginIDs(pluginIDs)
	if err != nil {
		t.Errorf("received unexpected error: %v", err)
	}
	for _, p := range verions {
		t.Logf("version id: %d; deploy version: %s; status: %s", p.ID, p.DeployVersion, p.Status)
	}
}

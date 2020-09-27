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
	"testing"
	"time"

	dbconfig "github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	"github.com/testcontainers/testcontainers-go"
)

func TestTenantServicesDao_GetOpenedPort(t *testing.T) {
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

	sid := util.NewUUID()
	trueVal := true
	falseVal := true
	err = GetManager().TenantServicesPortDao().AddModel(&model.TenantServicesPort{
		ServiceID:      sid,
		ContainerPort:  1111,
		MappingPort:    1111,
		IsInnerService: &falseVal,
		IsOuterService: &trueVal,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = GetManager().TenantServicesPortDao().AddModel(&model.TenantServicesPort{
		ServiceID:      sid,
		ContainerPort:  2222,
		MappingPort:    2222,
		IsInnerService: &trueVal,
		IsOuterService: &falseVal,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = GetManager().TenantServicesPortDao().AddModel(&model.TenantServicesPort{
		ServiceID:      sid,
		ContainerPort:  3333,
		MappingPort:    3333,
		IsInnerService: &falseVal,
		IsOuterService: &falseVal,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = GetManager().TenantServicesPortDao().AddModel(&model.TenantServicesPort{
		ServiceID:      sid,
		ContainerPort:  5555,
		MappingPort:    5555,
		IsInnerService: &trueVal,
		IsOuterService: &trueVal,
	})
	if err != nil {
		t.Fatal(err)
	}
	ports, err := GetManager().TenantServicesPortDao().GetOpenedPorts(sid)
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 3 {
		t.Errorf("Expected 3 for the length of ports, but return %d", len(ports))
	}
}

func TestListInnerPorts(t *testing.T) {
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

	sid := util.NewUUID()
	trueVal := true
	falseVal := false
	err = GetManager().TenantServicesPortDao().AddModel(&model.TenantServicesPort{
		ServiceID:      sid,
		ContainerPort:  1111,
		MappingPort:    1111,
		IsInnerService: &trueVal,
		IsOuterService: &trueVal,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = GetManager().TenantServicesPortDao().AddModel(&model.TenantServicesPort{
		ServiceID:      sid,
		ContainerPort:  2222,
		MappingPort:    2222,
		IsInnerService: &trueVal,
		IsOuterService: &falseVal,
	})
	if err != nil {
		t.Fatal(err)
	}

	ports, err := GetManager().TenantServicesPortDao().ListInnerPortsByServiceIDs([]string{sid})
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 2 {
		t.Errorf("Expocted %d for ports, but got %d", 2, len(ports))
	}
}

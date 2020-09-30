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
	"github.com/testcontainers/testcontainers-go"
	"testing"
	"time"
)

func TestTenantServicesDao_ListThirdPartyServices(t *testing.T) {
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

	svcs, err := GetManager().TenantServiceDao().ListThirdPartyServices()
	if err != nil {
		t.Fatalf("error listing third-party service: %v", err)
	}
	if len(svcs) != 0 {
		t.Errorf("Expected 0 for the length of third-party services, but returned %d", len(svcs))
	}

	for i := 0; i < 3; i++ {
		item1 := &model.TenantServices{
			TenantID:  util.NewUUID(),
			ServiceID: util.NewUUID(),
			Kind:      model.ServiceKindThirdParty.String(),
		}
		if err = GetManager().TenantServiceDao().AddModel(item1); err != nil {
			t.Fatalf("error create third-party service: %v", err)
		}
	}
	svcs, err = GetManager().TenantServiceDao().ListThirdPartyServices()
	if err != nil {
		t.Fatalf("error listing third-party service: %v", err)
	}
	if len(svcs) != 3 {
		t.Errorf("Expected 3 for the length of third-party services, but returned %d", len(svcs))
	}
}

func TestTenantServicesPortDao_HasOpenPort(t *testing.T) {
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

	t.Run("service doesn't exist", func(t *testing.T) {
		hasOpenPort := GetManager().TenantServicesPortDao().HasOpenPort("foobar")
		if hasOpenPort {
			t.Error("Expected false for hasOpenPort, but returned true")
		}
	})
	trueVal := true
	falseVal := true
	t.Run("outer service", func(t *testing.T) {
		port := &model.TenantServicesPort{
			ServiceID:      util.NewUUID(),
			IsOuterService: &trueVal,
		}
		if err := GetManager().TenantServicesPortDao().AddModel(port); err != nil {
			t.Fatalf("error creating TenantServicesPort: %v", err)
		}
		hasOpenPort := GetManager().TenantServicesPortDao().HasOpenPort(port.ServiceID)
		if !hasOpenPort {
			t.Errorf("Expected true for hasOpenPort, but returned %v", hasOpenPort)
		}
	})
	t.Run("inner service", func(t *testing.T) {
		port := &model.TenantServicesPort{
			ServiceID:      util.NewUUID(),
			IsInnerService: &trueVal,
		}
		if err := GetManager().TenantServicesPortDao().AddModel(port); err != nil {
			t.Fatalf("error creating TenantServicesPort: %v", err)
		}
		hasOpenPort := GetManager().TenantServicesPortDao().HasOpenPort(port.ServiceID)
		if !hasOpenPort {
			t.Errorf("Expected true for hasOpenPort, but returned %v", hasOpenPort)
		}
	})
	t.Run("not inner or outer service", func(t *testing.T) {
		port := &model.TenantServicesPort{
			ServiceID:      util.NewUUID(),
			IsInnerService: &falseVal,
			IsOuterService: &falseVal,
		}
		if err := GetManager().TenantServicesPortDao().AddModel(port); err != nil {
			t.Fatalf("error creating TenantServicesPort: %v", err)
		}
		hasOpenPort := GetManager().TenantServicesPortDao().HasOpenPort(port.ServiceID)
		if hasOpenPort {
			t.Errorf("Expected false for hasOpenPort, but returned %v", hasOpenPort)
		}
	})
}

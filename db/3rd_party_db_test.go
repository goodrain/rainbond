// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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

func TestEndpointDaoImpl_UpdateModel(t *testing.T) {
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

	trueVal := true
	falseVal := false
	ep := &model.Endpoint{
		UUID:      util.NewUUID(),
		ServiceID: util.NewUUID(),
		IP:        "10.10.10.10",
		IsOnline:  &trueVal,
	}
	err = GetManager().EndpointsDao().AddModel(ep)
	if err != nil {
		t.Fatalf("error adding endpoint: %v", err)
	}
	ep.IsOnline = &falseVal
	err = GetManager().EndpointsDao().UpdateModel(ep)
	if err != nil {
		t.Fatalf("error updating endpoint: %v", err)
	}
	e, err := GetManager().EndpointsDao().GetByUUID(ep.UUID)
	if err != nil {
		t.Fatalf("error getting endpoint: %v", err)
	}
	if *e.IsOnline != false {
		t.Errorf("Expected %v for e.IsOnline, but returned %v", false, e.IsOnline)
	}
}

func TestEndpointDaoImpl_AddModel(t *testing.T) {
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
		t.Fatal(err)
	}
	port, err := mariadb.MappedPort(ctx, "3306")
	if err != nil {
		t.Fatal(err)
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

	falseVal := false
	ep := &model.Endpoint{
		UUID:      util.NewUUID(),
		ServiceID: util.NewUUID(),
		IP:        "10.10.10.10",
		IsOnline:  &falseVal,
	}
	err = GetManager().EndpointsDao().AddModel(ep)
	if err != nil {
		t.Fatalf("error adding endpoint: %v", err)
	}
	e, err := GetManager().EndpointsDao().GetByUUID(ep.UUID)
	if err != nil {
		t.Fatalf("error getting endpoint: %v", err)
	}
	if *e.IsOnline != false {
		t.Errorf("Expected %v for e.IsOnline, but returned %v", false, e.IsOnline)
	}
}

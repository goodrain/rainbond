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
	"fmt"
	"testing"
	"time"

	dbconfig "github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
)

func TestTenantDao(t *testing.T) {
	if err := CreateManager(dbconfig.Config{
		MysqlConnectionInfo: "root:admin@tcp(127.0.0.1:3306)/region",
		DBType:              "mysql",
	}); err != nil {
		t.Fatal(err)
	}
	err := GetManager().TenantDao().AddModel(&model.Tenants{
		Name: "barnett4",
		UUID: util.NewUUID(),
	})
	if err != nil {
		t.Fatal(err)
	}
	tenant, err := GetManager().TenantDao().GetTenantByUUID("27bbdd119b24444696dc51aa2f41eef8")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(tenant)
}

func TestTenantServiceDao(t *testing.T) {
	if err := CreateManager(dbconfig.Config{
		MysqlConnectionInfo: "root:admin@tcp(127.0.0.1:3306)/region",
		DBType:              "mysql",
	}); err != nil {
		t.Fatal(err)
	}
	service, err := GetManager().TenantServiceDao().GetServiceByTenantIDAndServiceAlias("27bbdd119b24444696dc51aa2f41eef8", "grb58f90")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(service)
	service, err = GetManager().TenantServiceDao().GetServiceByID("2f29882148c19f5f84e3a7cedf6097c7")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(service)
	services, err := GetManager().TenantServiceDao().GetServiceAliasByIDs([]string{"2f29882148c19f5f84e3a7cedf6097c7"})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range services {
		t.Log(s)
	}
}

func TestGetServiceEnvs(t *testing.T) {
	if err := CreateManager(dbconfig.Config{
		MysqlConnectionInfo: "root:admin@tcp(127.0.0.1:3306)/region",
		DBType:              "mysql",
	}); err != nil {
		t.Fatal(err)
	}
	envs, err := GetManager().TenantServiceEnvVarDao().GetServiceEnvs("2f29882148c19f5f84e3a7cedf6097c7", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range envs {
		t.Log(e)
	}
}

func TestSetServiceLabel(t *testing.T) {
	if err := CreateManager(dbconfig.Config{
		MysqlConnectionInfo: "root:admin@tcp(127.0.0.1:3306)/region",
		DBType:              "mysql",
	}); err != nil {
		t.Fatal(err)
	}
	label := model.TenantServiceLable{
		LabelKey:   model.LabelKeyServiceType,
		LabelValue: util.StatefulServiceType,
		ServiceID:  "889bb1f028f655bebd545f24aa184a0b",
	}
	label.CreatedAt = time.Now()
	label.ID = 1
	err := GetManager().TenantServiceLabelDao().UpdateModel(&label)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateTenantServiceLBMappingPort(t *testing.T) {
	if err := CreateManager(dbconfig.Config{
		MysqlConnectionInfo: "root:admin@tcp(127.0.0.1:3306)/region",
		DBType:              "mysql",
	}); err != nil {
		t.Fatal(err)
	}
	mapPort, err := GetManager().TenantServiceLBMappingPortDao().CreateTenantServiceLBMappingPort("889bb1f028f655bebd545f24aa184a0b", 8080)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(mapPort)
}

func TestCreateTenantServiceLBMappingPortTran(t *testing.T) {
	if err := CreateManager(dbconfig.Config{
		MysqlConnectionInfo: "root:admin@tcp(127.0.0.1:3306)/region",
		DBType:              "mysql",
	}); err != nil {
		t.Fatal(err)
	}
	tx := GetManager().Begin()
	mapPort, err := GetManager().TenantServiceLBMappingPortDaoTransactions(tx).CreateTenantServiceLBMappingPort("889bb1f028f655bebd545f24aa184a0b", 8082)
	if err != nil {
		tx.Rollback()
		t.Fatal(err)
		return
	}
	tx.Commit()
	t.Log(mapPort)
}

func TestGetMem(t *testing.T) {
	if err := CreateManager(dbconfig.Config{
		MysqlConnectionInfo: "admin:admin@tcp(127.0.0.1:3306)/region",
		DBType:              "mysql",
	}); err != nil {
		t.Fatal(err)
	}
	// err := GetManager().TenantDao().AddModel(&model.Tenants{
	// 	Name: "barnett3",
	// 	UUID: util.NewUUID(),
	// })
	// if err != nil {
	// 	t.Fatal(err)
	// }
}

func TestCockroachDBCreateTable(t *testing.T) {
	if err := CreateManager(dbconfig.Config{
		MysqlConnectionInfo: "postgresql://root@localhost:5432/region?sslmode=disable",
		DBType:              "cockroachdb",
	}); err != nil {
		t.Fatal(err)
	}
}
func TestCockroachDBCreateService(t *testing.T) {
	if err := CreateManager(dbconfig.Config{
		MysqlConnectionInfo: "postgresql://root@localhost:5432/region?sslmode=disable",
		DBType:              "cockroachdb",
	}); err != nil {
		t.Fatal(err)
	}
	err := GetManager().TenantServiceDao().AddModel(&model.TenantServices{
		TenantID:     "asdasd",
		ServiceID:    "asdasdasdasd",
		ServiceAlias: "grasdasdasdads",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCockroachDBDeleteService(t *testing.T) {
	if err := CreateManager(dbconfig.Config{
		MysqlConnectionInfo: "postgresql://root@localhost:5432/region?sslmode=disable",
		DBType:              "cockroachdb",
	}); err != nil {
		t.Fatal(err)
	}
	err := GetManager().TenantServiceDao().DeleteServiceByServiceID("asdasdasdasd")
	if err != nil {
		t.Fatal(err)
	}
}

//func TestCockroachDBSaveDeployInfo(t *testing.T) {
//	if err := CreateManager(dbconfig.Config{
//		MysqlConnectionInfo: "postgresql://root@localhost:5432/region?sslmode=disable",
//		DBType:              "cockroachdb",
//	}); err != nil {
//		t.Fatal(err)
//	}
//	err := GetManager().K8sDeployReplicationDao().AddModel(&model.K8sDeployReplication{
//		TenantID:        "asdasd",
//		ServiceID:       "asdasdasdasd",
//		ReplicationID:   "asdasdadsasdasdasd",
//		ReplicationType: model.TypeReplicationController,
//	})
//	if err != nil {
//		t.Fatal(err)
//	}
//}

//func TestCockroachDBDeleteDeployInfo(t *testing.T) {
//	if err := CreateManager(dbconfig.Config{
//		MysqlConnectionInfo: "postgresql://root@localhost:5432/region?sslmode=disable",
//		DBType:              "cockroachdb",
//	}); err != nil {
//		t.Fatal(err)
//	}
	//err := GetManager().K8sDeployReplicationDao().DeleteK8sDeployReplication("asdasdadsasdasdasd")
	//if err != nil {
	//	t.Fatal(err)
	//}
//}

func TestGetHttpRuleByServiceIDAndContainerPort(t *testing.T) {
	if err := CreateManager(dbconfig.Config{
		MysqlConnectionInfo: "admin:admin@tcp(127.0.0.1:3306)/region",
		DBType:              "mysql",
	}); err != nil {
		t.Fatal(err)
	}

	_, err := GetManager().HttpRuleDao().GetHttpRuleByServiceIDAndContainerPort(
		"43eaae441859eda35b02075d37d83581", 10001)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetTcpRuleByServiceIDAndContainerPort(t *testing.T) {
	if err := CreateManager(dbconfig.Config{
		MysqlConnectionInfo: "admin:admin@tcp(127.0.0.1:3306)/region",
		DBType:              "mysql",
	}); err != nil {
		t.Fatal(err)
	}

	_, err := GetManager().TcpRuleDao().GetTcpRuleByServiceIDAndContainerPort(
		"43eaae441859eda35b02075d37d83581", 10001)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetCertificateByID(t *testing.T) {
	if err := CreateManager(dbconfig.Config{
		MysqlConnectionInfo: "admin:admin@tcp(127.0.0.1:3306)/region",
		DBType:              "mysql",
	}); err != nil {
		t.Fatal(err)
	}

	cert, err := GetManager().TenantServiceLabelDao().GetTenantNodeAffinityLabel("105bb7d4b94774f922edb3051bdf8ce1")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(cert)
}

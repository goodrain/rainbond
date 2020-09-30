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

func TestGwRuleConfig(t *testing.T) {
	dbm, err := CreateTestManager()
	if err != nil {
		t.Fatalf("error creating test db manager: %v", err)
	}
	rid := util.NewUUID()
	cfg := &model.GwRuleConfig{
		RuleID: rid,
		Key:    "set-header-Host",
		Value:  "$http_host",
	}
	if err := dbm.GwRuleConfigDao().AddModel(cfg); err != nil {
		t.Fatalf("error create rule config: %v", err)
	}
	cfg2 := &model.GwRuleConfig{
		RuleID: rid,
		Key:    "set-header-foo",
		Value:  "bar",
	}
	if err := dbm.GwRuleConfigDao().AddModel(cfg2); err != nil {
		t.Fatalf("error create rule config: %v", err)
	}
	list, err := dbm.GwRuleConfigDao().ListByRuleID(rid)
	if err != nil {
		t.Fatalf("error listing configs: %v", err)
	}
	if list == nil && len(list) != 2 {
		t.Errorf("Expected 2 for the length fo list, but returned %d", len(list))
	}

	if err := dbm.GwRuleConfigDao().DeleteByRuleID(rid); err != nil {
		t.Fatalf("error deleting rule config: %v", err)
	}
	list, err = dbm.GwRuleConfigDao().ListByRuleID(rid)
	if err != nil {
		t.Fatalf("error listing configs: %v", err)
	}
	if list != nil && len(list) > 0 {
		t.Errorf("Expected empty for list, but returned %+v", list)
	}
}

func TestCertificateDaoImpl_AddOrUpdate(t *testing.T) {
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

	cert := &model.Certificate{
		UUID:            util.NewUUID(),
		CertificateName: "dummy-name",
		Certificate:     "dummy-certificate",
		PrivateKey:      "dummy-privateKey",
	}
	err = GetManager().CertificateDao().AddOrUpdate(cert)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := GetManager().CertificateDao().GetCertificateByID(cert.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if resp.UUID != cert.UUID {
		t.Errorf("Expected %s for resp.UUID, but returned %s", cert.UUID, resp.UUID)
	}
	if resp.CertificateName != cert.CertificateName {
		t.Errorf("Expected %s for resp.CertificateName, but returned %s", cert.CertificateName, resp.CertificateName)
	}
	if resp.Certificate != cert.Certificate {
		t.Errorf("Expected %s for resp.Certificate, but returned %s", cert.Certificate, resp.Certificate)
	}
	if resp.PrivateKey != cert.PrivateKey {
		t.Errorf("Expected %s for resp.UUID, but returned %s", cert.PrivateKey, resp.PrivateKey)
	}

	cert.Certificate = "update-certificate"
	cert.PrivateKey = "update-privateKey"
	err = GetManager().CertificateDao().AddOrUpdate(cert)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = GetManager().CertificateDao().GetCertificateByID(cert.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if resp.UUID != cert.UUID {
		t.Errorf("Expected %s for resp.UUID, but returned %s", cert.UUID, resp.UUID)
	}
	if resp.CertificateName != cert.CertificateName {
		t.Errorf("Expected %s for resp.CertificateName, but returned %s", cert.CertificateName, resp.CertificateName)
	}
	if resp.Certificate != cert.Certificate {
		t.Errorf("Expected %s for resp.Certificate, but returned %s", cert.Certificate, resp.Certificate)
	}
	if resp.PrivateKey != cert.PrivateKey {
		t.Errorf("Expected %s for resp.UUID, but returned %s", cert.PrivateKey, resp.PrivateKey)
	}
}

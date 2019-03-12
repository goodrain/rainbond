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
	"testing"

	dbconfig "github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
)

func TestIPPortImpl_UpdateModel(t *testing.T) {
	if err := CreateManager(dbconfig.Config{
		DBType: "sqlite3",
	}); err != nil {
		t.Fatal(err)
	}
	tx := GetManager().Begin()
	tx.Delete(model.IPPort{})
	tx.Commit()

	ipport := &model.IPPort{
		UUID: util.NewUUID(),
		IP:   "127.0.0.1",
		Port: 8888,
	}
	if err := GetManager().IPPortDao().AddModel(ipport); err != nil {
		t.Fatal(err)
	}
	ipport.Port = 9999
	err := GetManager().IPPortDao().UpdateModel(ipport)
	if err != nil {
		t.Fatal(err)
	}
}

func TestIPPortImpl_DeleteIPPortByIPAndPort(t *testing.T) {
	if err := CreateManager(dbconfig.Config{
		DBType: "sqlite3",
	}); err != nil {
		t.Fatal(err)
	}
	tx := GetManager().Begin()
	tx.Delete(model.IPPort{})
	tx.Commit()

	ipport := &model.IPPort{
		UUID: util.NewUUID(),
		IP:   "127.0.0.1",
		Port: 8888,
	}
	if err := GetManager().IPPortDao().AddModel(ipport); err != nil {
		t.Fatal(err)
	}

	if err := GetManager().IPPortDao().DeleteByIPAndPort(ipport.IP, ipport.Port); err != nil {
		t.Fatal(err)
	}

	ipPort, err := GetManager().IPPortDao().GetIPPortByIPAndPort(ipport.IP, ipport.Port)
	if err != nil {
		t.Fatal(err)
	}
	if ipPort != nil {
		t.Errorf("Expected nil for ipPort, but return %v", ipPort)
	}
}

func TestIPPortImpl_GetIPByPort(t *testing.T) {
	if err := CreateManager(dbconfig.Config{
		DBType: "sqlite3",
	}); err != nil {
		t.Fatal(err)
	}
	tx := GetManager().Begin()
	tx.Delete(model.IPPort{})
	tx.Commit()

	ipport := &model.IPPort{
		UUID: util.NewUUID(),
		IP:   "127.0.0.1",
		Port: 8888,
	}
	if err := GetManager().IPPortDao().AddModel(ipport); err != nil {
		t.Fatal(err)
	}

	ports, err := GetManager().IPPortDao().GetIPByPort(8888)
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 1 {
		t.Fatalf("Expected 1 for length of ports, but returned %d)", len(ports))
	}
}

func TestIPPortImpl_GetIPPortByIPAndPort(t *testing.T) {
	if err := CreateManager(dbconfig.Config{
		DBType: "sqlite3",
	}); err != nil {
		t.Fatal(err)
	}
	tx := GetManager().Begin()
	tx.Delete(model.IPPort{})
	tx.Commit()

	ipport := &model.IPPort{
		UUID: util.NewUUID(),
		IP:   "127.0.0.1",
		Port: 8888,
	}
	if err := GetManager().IPPortDao().AddModel(ipport); err != nil {
		t.Fatal(err)
	}

	ipPort, err := GetManager().IPPortDao().GetIPPortByIPAndPort(ipport.IP, ipport.Port)
	if err != nil {
		t.Fatal(err)
	}
	if ipPort.IP != ipport.IP {
		t.Errorf("Expected %s for ip, but retruned %s", ipport.IP, ipPort.IP)
	}
	if ipPort.Port != ipport.Port {
		t.Errorf("Expected %d for port, but retruned %d", ipport.Port, ipPort.Port)
	}
}

func TestIPPoolImpl_AddModel(t *testing.T) {
	if err := CreateManager(dbconfig.Config{
		DBType: "sqlite3",
	}); err != nil {
		t.Fatal(err)
	}
	tx := GetManager().Begin()
	tx.Delete(model.IPPool{})
	tx.Commit()

	ippool := &model.IPPool{
		EID:  util.NewUUID(),
		CIDR: "192.168.11.11/24",
	}

	if err := GetManager().IPPoolDao().AddModel(ippool); err != nil {
		t.Fatal(err)
	}
	pool, err := GetManager().IPPoolDao().GetIPPoolByEID(ippool.EID)
	if err != nil {
		t.Fatal(err)
	}
	if pool == nil {
		t.Errorf("Expected %v for pool, but returned nil", pool)
	}
	if pool.CIDR != ippool.CIDR {
		t.Errorf("Expected %v for CIDR, but returned %s", ippool.CIDR, pool.CIDR)
	}
}

func TestIPPoolImpl_UpdateModel(t *testing.T) {
	if err := CreateManager(dbconfig.Config{
		DBType: "sqlite3",
	}); err != nil {
		t.Fatal(err)
	}
	tx := GetManager().Begin()
	tx.Delete(model.IPPool{})
	tx.Commit()

	ippool := &model.IPPool{
		EID:  util.NewUUID(),
		CIDR: "192.168.11.11/24",
	}

	if err := GetManager().IPPoolDao().AddModel(ippool); err != nil {
		t.Fatal(err)
	}
	ippool.CIDR = "192.168.22.22/24"
	if err := GetManager().IPPoolDao().UpdateModel(ippool); err != nil {
		t.Fatal(err)
	}
	if ippool.CIDR != "192.168.22.22/24" {
		t.Errorf("Expected %s for CIDR, but returned %s", "192.168.22.22/24", ippool.CIDR)
	}
}

func TestHTTPRuleImpl_ListByServiceID(t *testing.T) {
	if err := CreateManager(dbconfig.Config{
		DBType: "sqlite3",
	}); err != nil {
		t.Fatal(err)
	}
	tx := GetManager().Begin()
	tx.Delete(model.HTTPRule{})
	tx.Commit()

	rules, err := GetManager().HTTPRuleDao().ListByServiceID("")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 0 {
		t.Errorf("Expected 0 for len(rules), but returned %v", len(rules))
	}

	serviceID := util.NewUUID()
	rules = []*model.HTTPRule{
		{
			UUID:      util.NewUUID(),
			ServiceID: serviceID,
		},
		{
			UUID:      util.NewUUID(),
			ServiceID: serviceID,
		},
		{
			UUID:      util.NewUUID(),
			ServiceID: serviceID,
		},
	}
	for _, rule := range rules {
		err := GetManager().HTTPRuleDao().AddModel(rule)
		if err != nil {
			t.Fatal(err)
		}
	}
	rules, err = GetManager().HTTPRuleDao().ListByServiceID(serviceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 3 {
		t.Errorf("Expected 3 for len(rules), but returned %v", len(rules))
	}
}

func TestGwRuleConfig(t *testing.T) {
	dbm, err := CreateTestManager()
	if err != nil {
		t.Fatalf("error creating test db manager: %v", err)
	}
	rid := util.NewUUID()
	cid := util.NewUUID()
	cfg := &model.GwRuleConfig{
		ConfigID: cid,
		RuleID:   rid,
		Key:      "set-header-Host",
		Value:    "$http_host",
	}
	if err := dbm.GwRuleConfigDao().AddModel(cfg); err != nil {
		t.Fatalf("error create rule config: %v", err)
	}

	cfg.Key = "proxy_read_timeout"
	cfg.Value = "75"
	if err := dbm.GwRuleConfigDao().UpdateModel(cfg); err != nil {
		t.Fatalf("error updating rule config: %v", err)
	}
	config, err := dbm.GwRuleConfigDao().GetByConfigID(cfg.ConfigID)
	if err != nil {
		t.Fatalf("error getting rule config: %v", err)
	}
	if config.Key != "proxy_read_timeout" {
		t.Errorf("Expected proxy_read_timeout for key, but returned %s", config.Key)
	}
	if config.Value != "75" {
		t.Errorf("Expected $http_host for key, but returned %s", config.Value)
	}

	for i := 0; i < 2; i++ {
		cfg = &model.GwRuleConfig{
			ConfigID: util.NewUUID(),
			RuleID:   rid,
			Key:      "set-header-Host",
			Value:    "$http_host",
		}
		if err := dbm.GwRuleConfigDao().AddModel(cfg); err != nil {
			t.Fatalf("error create rule config: %v", err)
		}
	}
	cfgs, err := dbm.GwRuleConfigDao().ListByRuleID(rid)
	if err != nil {
		t.Fatalf("error create rule config: %v", err)
	}
	if cfgs == nil || len(cfgs) != 3 {
		t.Errorf("Expected 3 for the length of cfgs, but returned %d", len(cfgs))
	}

	if err := dbm.GwRuleConfigDao().DeleteByConfigID(cid); err != nil {
		t.Fatalf("error deleting rule config: %v", err)
	}
	cfgs, err = dbm.GwRuleConfigDao().ListByRuleID(rid)
	if err != nil {
		t.Fatalf("error create rule config: %v", err)
	}
	if cfgs == nil || len(cfgs) != 2 {
		t.Errorf("Expected 2 for the length of cfgs, but returned %d", len(cfgs))
	}

	if err := dbm.GwRuleConfigDao().DeleteByRuleID(rid); err != nil {
		t.Fatalf("error deleting rule config: %v", err)
	}
	cfgs, err = dbm.GwRuleConfigDao().ListByRuleID(rid)
	if err != nil {
		t.Fatalf("error create rule config: %v", err)
	}
	if !(cfgs == nil || len(cfgs) == 0) {
		t.Errorf("Expected 0 for the length of cfgs, but returned %d", len(cfgs))
	}
}

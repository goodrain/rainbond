package dao

import (
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func newTCPRuleTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "tcp-rule.db"))
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	db.LogMode(false)

	if err := db.AutoMigrate(&model.TCPRule{}).Error; err != nil {
		db.Close()
		t.Fatalf("auto migrate tcp rule: %v", err)
	}

	return db
}

// capability_id: rainbond.gateway.reject-duplicate-tcp-nodeport
func TestTCPRuleDaoAddModelRejectsPortOwnedByAnotherRule(t *testing.T) {
	db := newTCPRuleTestDB(t)
	defer db.Close()

	dao := &TCPRuleDaoTmpl{DB: db}
	original := &model.TCPRule{
		UUID:          "rule-a",
		ServiceID:     "service-a",
		ContainerPort: 8080,
		IP:            "0.0.0.0",
		Port:          30000,
	}
	if err := dao.AddModel(original); err != nil {
		t.Fatalf("add original TCP rule: %v", err)
	}

	err := dao.AddModel(&model.TCPRule{
		UUID:          "rule-b",
		ServiceID:     "service-b",
		ContainerPort: 9090,
		IP:            "0.0.0.0",
		Port:          30000,
	})
	if err == nil {
		t.Fatal("expected duplicate TCP NodePort to be rejected")
	}

	stored, err := dao.GetTCPRuleByPort(30000)
	if err != nil {
		t.Fatalf("get original TCP rule: %v", err)
	}
	if stored == nil || stored.UUID != original.UUID || stored.ServiceID != original.ServiceID {
		t.Fatalf("expected original TCP rule to remain, got %#v", stored)
	}
}

func TestTCPRuleDaoAddModelUpdatesExistingRuleByUUID(t *testing.T) {
	db := newTCPRuleTestDB(t)
	defer db.Close()

	dao := &TCPRuleDaoTmpl{DB: db}
	if err := dao.AddModel(&model.TCPRule{
		UUID:          "rule-a",
		ServiceID:     "service-a",
		ContainerPort: 8080,
		IP:            "0.0.0.0",
		Port:          30000,
	}); err != nil {
		t.Fatalf("add original TCP rule: %v", err)
	}

	if err := dao.AddModel(&model.TCPRule{
		UUID:          "rule-a",
		ServiceID:     "service-a",
		ContainerPort: 9090,
		IP:            "0.0.0.0",
		Port:          30001,
	}); err != nil {
		t.Fatalf("update TCP rule: %v", err)
	}

	updated, err := dao.GetTCPRuleByPort(30001)
	if err != nil {
		t.Fatalf("get updated TCP rule: %v", err)
	}
	if updated == nil || updated.UUID != "rule-a" || updated.ContainerPort != 9090 {
		t.Fatalf("expected TCP rule to be updated, got %#v", updated)
	}
	var count int
	if err := db.Model(&model.TCPRule{}).Count(&count).Error; err != nil {
		t.Fatalf("count TCP rules: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one TCP rule after update, got %d", count)
	}
}

func TestTCPRuleDaoAddModelKeepsAnonymousRulesOnDifferentPorts(t *testing.T) {
	db := newTCPRuleTestDB(t)
	defer db.Close()

	dao := &TCPRuleDaoTmpl{DB: db}
	for _, port := range []int{30000, 30001} {
		if err := dao.AddModel(&model.TCPRule{
			IP:            "0.0.0.0",
			Port:          port,
			ContainerPort: 8080,
		}); err != nil {
			t.Fatalf("add anonymous TCP rule for port %d: %v", port, err)
		}
	}

	var count int
	if err := db.Model(&model.TCPRule{}).Count(&count).Error; err != nil {
		t.Fatalf("count TCP rules: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected two anonymous TCP rules, got %d", count)
	}
}

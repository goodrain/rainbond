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

func TestTCPRuleDaoReplaceByIPAndPortCollapsesDuplicates(t *testing.T) {
	db := newTCPRuleTestDB(t)
	defer db.Close()

	dao := &TCPRuleDaoTmpl{DB: db}
	seedRules := []*model.TCPRule{
		{
			UUID:          "rule-a",
			ServiceID:     "service-a",
			ContainerPort: 8080,
			IP:            "0.0.0.0",
			Port:          30000,
		},
		{
			UUID:          "rule-b",
			ServiceID:     "service-b",
			ContainerPort: 9090,
			IP:            "0.0.0.0",
			Port:          30001,
		},
		{
			ContainerPort: 9090,
			IP:            "0.0.0.0",
			Port:          30001,
		},
	}
	for _, rule := range seedRules {
		if err := db.Create(rule).Error; err != nil {
			t.Fatalf("seed TCP rule: %v", err)
		}
	}

	canonical := &model.TCPRule{
		UUID:          "rule-a",
		ServiceID:     "service-a",
		ContainerPort: 8080,
		IP:            "0.0.0.0",
		Port:          30001,
	}
	if err := dao.ReplaceByIPAndPort(canonical); err != nil {
		t.Fatalf("replace TCP rules: %v", err)
	}

	rules, err := dao.ListByIPAndPort("0.0.0.0", 30001)
	if err != nil {
		t.Fatalf("list replacement TCP rules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected one canonical TCP rule, got %d", len(rules))
	}
	if rules[0].UUID != canonical.UUID ||
		rules[0].ServiceID != canonical.ServiceID ||
		rules[0].ContainerPort != canonical.ContainerPort {
		t.Fatalf("unexpected canonical TCP rule: %#v", rules[0])
	}

	var oldUUIDCount int
	if err := db.Model(&model.TCPRule{}).
		Where("uuid = ? and port = ?", canonical.UUID, 30000).
		Count(&oldUUIDCount).Error; err != nil {
		t.Fatalf("count old UUID rule: %v", err)
	}
	if oldUUIDCount != 0 {
		t.Fatalf("expected old UUID rule to be removed, got %d", oldUUIDCount)
	}
}

func TestTCPRuleDaoDeleteByRecordIDsDeletesOnlyCapturedRules(t *testing.T) {
	db := newTCPRuleTestDB(t)
	defer db.Close()

	dao := &TCPRuleDaoTmpl{DB: db}
	for _, rule := range []*model.TCPRule{
		{
			UUID:          "rule-a",
			ServiceID:     "service-a",
			ContainerPort: 8080,
			IP:            "0.0.0.0",
			Port:          30001,
		},
		{
			ContainerPort: 8080,
			IP:            "0.0.0.0",
			Port:          30001,
		},
	} {
		if err := db.Create(rule).Error; err != nil {
			t.Fatalf("seed captured TCP rule: %v", err)
		}
	}

	captured, err := dao.ListByIPAndPort("0.0.0.0", 30001)
	if err != nil {
		t.Fatalf("capture TCP rule IDs: %v", err)
	}
	capturedIDs := make([]uint, 0, len(captured))
	for _, rule := range captured {
		capturedIDs = append(capturedIDs, rule.ID)
	}

	newOwner := &model.TCPRule{
		UUID:          "rule-b",
		ServiceID:     "service-b",
		ContainerPort: 9090,
		IP:            "0.0.0.0",
		Port:          30001,
	}
	if err := db.Create(newOwner).Error; err != nil {
		t.Fatalf("seed new TCP rule owner: %v", err)
	}

	if err := dao.DeleteByRecordIDs(capturedIDs); err != nil {
		t.Fatalf("delete captured TCP rules: %v", err)
	}
	if err := dao.DeleteByRecordIDs(nil); err != nil {
		t.Fatalf("delete empty TCP rule ID list: %v", err)
	}

	rules, err := dao.ListByIPAndPort("0.0.0.0", 30001)
	if err != nil {
		t.Fatalf("list remaining TCP rules: %v", err)
	}
	if len(rules) != 1 || rules[0].ID != newOwner.ID {
		t.Fatalf("expected only the new TCP rule owner to remain, got %#v", rules)
	}
}

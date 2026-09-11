package mysql

import (
	"path/filepath"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/goodrain/rainbond/db/model"
	mysqldao "github.com/goodrain/rainbond/db/mysql/dao"
	"github.com/jinzhu/gorm"
)

func TestTCPRuleMySQLMigrationQuotesProtocolDefault(t *testing.T) {
	database, mock := newMySQLDialectPatchTestDBWithMock(t)
	mock.MatchExpectationsInOrder(false)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT DATABASE()")).WillReturnRows(sqlmock.NewRows([]string{"database"}).AddRow("region"))
	mock.ExpectQuery(regexp.QuoteMeta("SHOW TABLES FROM `region` WHERE `Tables_in_region` = ?")).
		WithArgs("gateway_tcp_rule").WillReturnRows(sqlmock.NewRows([]string{"table"}).AddRow("gateway_tcp_rule"))
	for _, field := range database.NewScope(&model.TCPRule{}).GetStructFields() {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT DATABASE()")).WillReturnRows(sqlmock.NewRows([]string{"database"}).AddRow("region"))
		rows := sqlmock.NewRows([]string{"Field"})
		if field.DBName != "protocol" {
			rows.AddRow(field.DBName)
		}
		mock.ExpectQuery(regexp.QuoteMeta("SHOW COLUMNS FROM `gateway_tcp_rule` FROM `region` WHERE Field = ?")).
			WithArgs(field.DBName).WillReturnRows(rows)
	}
	mock.ExpectExec(regexp.QuoteMeta("ALTER TABLE `gateway_tcp_rule` ADD `protocol` varchar(16) DEFAULT 'tcp';")).
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := database.AutoMigrate(&model.TCPRule{}).Error; err != nil {
		t.Fatalf("migrate legacy MySQL rule table: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTCPRuleMigrationPreservesLegacyRulesAndAllowsUDP(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "legacy-rules.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.Exec(`CREATE TABLE gateway_tcp_rule (
        ID integer PRIMARY KEY, create_time datetime, uuid varchar(255), service_id varchar(255),
        container_port integer, ip varchar(255), port integer
    )`).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Exec("INSERT INTO gateway_tcp_rule (uuid, service_id, container_port, ip, port) VALUES (?, ?, ?, ?, ?)",
		"existing-rule", "component", 53, "0.0.0.0", 30010).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := database.AutoMigrate(&model.TCPRule{}).Error; err != nil {
			t.Fatal(err)
		}
	}
	dao := &mysqldao.TCPRuleDaoTmpl{DB: database}
	old, err := dao.GetTCPRuleByPort(30010)
	if err != nil || old == nil || old.UUID != "existing-rule" || old.Protocol != "tcp" {
		t.Fatalf("legacy rule was not preserved with TCP default: %#v, %v", old, err)
	}
	for _, protocol := range []string{"udp", "tcp+udp"} {
		if err := dao.ReplaceByIPAndPort(&model.TCPRule{
			UUID: "new-rule", ServiceID: "component", ContainerPort: 53,
			IP: "0.0.0.0", Port: 30020, Protocol: protocol,
		}); err != nil {
			t.Fatalf("persist %s rule after migration: %v", protocol, err)
		}
		saved, err := dao.GetTCPRuleByPort(30020)
		if err != nil || saved == nil || saved.Protocol != protocol {
			t.Fatalf("incorrect protocol after migration: %#v, %v", saved, err)
		}
	}
}

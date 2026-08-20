package gormbulkups

import (
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

// capability_id: rainbond.database.dameng-bulk-upsert
func TestUsesDamengRowByRowUpsert(t *testing.T) {
	tests := []struct {
		name    string
		dialect string
		want    bool
	}{
		{name: "Dameng", dialect: "dm", want: true},
		{name: "SQLite", dialect: "sqlite3", want: false},
		{name: "MySQL", dialect: "mysql", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := usesDamengRowByRowUpsert(test.dialect); got != test.want {
				t.Fatalf("usesDamengRowByRowUpsert(%q) = %t, want %t", test.dialect, got, test.want)
			}
		})
	}
}

type damengUpsertRecord struct {
	ID    uint   `gorm:"column:id;primary_key"`
	Value string `gorm:"column:value"`
}

type mysqlRegressionRecord struct {
	ID    string `gorm:"column:id;primary_key;size:32"`
	Value string `gorm:"column:value;size:32"`
}

func (mysqlRegressionRecord) TableName() string {
	return "bulk_upsert_records"
}

func TestDamengUpsertObjectSetCreatesThenUpdatesByPrimaryKey(t *testing.T) {
	db, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "dameng-upsert.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	db.LogMode(false)
	if err := db.AutoMigrate(&damengUpsertRecord{}).Error; err != nil {
		t.Fatalf("migrate test table: %v", err)
	}

	if err := damengUpsertObjectSet(db, []interface{}{damengUpsertRecord{ID: 7, Value: "first"}}, 2000); err != nil {
		t.Fatalf("create record: %v", err)
	}
	if err := damengUpsertObjectSet(db, []interface{}{damengUpsertRecord{ID: 7, Value: "second"}}, 2000); err != nil {
		t.Fatalf("update record: %v", err)
	}

	var records []damengUpsertRecord
	if err := db.Find(&records).Error; err != nil {
		t.Fatalf("list records: %v", err)
	}
	if len(records) != 1 || records[0].Value != "second" {
		t.Fatalf("records = %#v, want one updated record", records)
	}
}

func TestDamengUpsertObjectSetReusesCallerTransaction(t *testing.T) {
	db, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "dameng-upsert-transaction.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	db.LogMode(false)
	if err := db.AutoMigrate(&damengUpsertRecord{}).Error; err != nil {
		t.Fatalf("migrate test table: %v", err)
	}

	tx := db.Begin()
	if err := tx.Error; err != nil {
		t.Fatalf("begin caller transaction: %v", err)
	}
	if err := damengUpsertObjectSet(tx, []interface{}{damengUpsertRecord{ID: 8, Value: "transactional"}}, 2000); err != nil {
		t.Fatalf("upsert in caller transaction: %v", err)
	}
	var count int
	if err := tx.Model(&damengUpsertRecord{}).Where("id = ?", 8).Count(&count).Error; err != nil {
		t.Fatalf("count records in caller transaction: %v", err)
	}
	if count != 1 {
		t.Fatalf("records in caller transaction = %d, want 1", count)
	}
	if err := tx.Rollback().Error; err != nil {
		t.Fatalf("rollback caller transaction: %v", err)
	}

	count = 0
	if err := db.Model(&damengUpsertRecord{}).Where("id = ?", 8).Count(&count).Error; err != nil {
		t.Fatalf("count records after rollback: %v", err)
	}
	if count != 0 {
		t.Fatalf("record was committed by the upsert instead of caller transaction, count = %d", count)
	}
}

func TestDamengUpsertObjectSetRollsBackOwnedTransactionOnInvalidObject(t *testing.T) {
	db, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "dameng-upsert-invalid.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	db.LogMode(false)

	err = damengUpsertObjectSet(db, []interface{}{"not-a-struct"}, 2000)
	if err == nil {
		t.Fatal("expected an invalid object error")
	}
}

func TestBulkUpsertHelpers(t *testing.T) {
	objects := []interface{}{1, 2, 3, 4, 5}
	chunks := splitObjects(objects, 2)
	if len(chunks) != 3 || len(chunks[2]) != 1 {
		t.Fatalf("chunks = %#v, want three chunks with one final item", chunks)
	}

	keys := sortedKeys(map[string]interface{}{"b": 1, "a": 2})
	if !sort.StringsAreSorted(keys) || keys[0] != "a" || keys[1] != "b" {
		t.Fatalf("keys = %#v, want sorted keys", keys)
	}
	if !containString([]string{"a", "b"}, "a") || containString([]string{"a", "b"}, "c") {
		t.Fatal("containString returned an unexpected result")
	}

	if _, err := extractMapValue("not-a-struct", nil); err == nil {
		t.Fatal("expected extractMapValue to reject a non-struct")
	}
}

func TestBulkUpsertKeepsMySQLStatement(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer sqlDB.Close()

	db, err := gorm.Open("mysql", sqlDB)
	if err != nil {
		t.Fatalf("open GORM database: %v", err)
	}
	defer db.Close()
	db.LogMode(false)

	expectedSQL := "INSERT INTO `bulk_upsert_records` (`id`, `value`) VALUES (?, ?) ON DUPLICATE KEY UPDATE `value`=VALUES(`value`)"
	mock.ExpectExec(regexp.QuoteMeta(expectedSQL)).
		WithArgs("record-1", "first").
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := BulkUpsert(db, []interface{}{mysqlRegressionRecord{ID: "record-1", Value: "first"}}, 2000); err != nil {
		t.Fatalf("bulk upsert: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("verify MySQL statement: %v", err)
	}
}

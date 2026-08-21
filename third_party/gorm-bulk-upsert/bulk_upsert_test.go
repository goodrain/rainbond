package gormbulkups

import (
	"regexp"
	"sort"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

type mysqlRegressionRecord struct {
	ID    string `gorm:"column:id;primary_key;size:32"`
	Value string `gorm:"column:value;size:32"`
}

func (mysqlRegressionRecord) TableName() string {
	return "bulk_upsert_records"
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

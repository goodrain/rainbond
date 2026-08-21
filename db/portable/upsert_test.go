package portable

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

type portableUpsertFixture struct {
	ID        int64     `gorm:"column:id;primary_key;AUTO_INCREMENT"`
	CreatedAt time.Time `gorm:"column:create_time"`
	Key       string    `gorm:"column:row_key;size:64;unique_index"`
	Value     string    `gorm:"column:row_value;type:text"`
	Enabled   bool      `gorm:"column:enabled"`
	Replicas  int       `gorm:"column:replicas"`
}

func (portableUpsertFixture) TableName() string {
	return "portable_upsert_fixture"
}

// capability_id: region.database.portable-bulk-upsert
func TestBulkUpsertUsesCallerTransactionAndDeclaredConflictColumns(t *testing.T) {
	db, err := gorm.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.AutoMigrate(&portableUpsertFixture{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&portableUpsertFixture{Key: "same-key", Value: "old"}).Error; err != nil {
		t.Fatal(err)
	}

	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	rows := []interface{}{
		portableUpsertFixture{Key: "same-key", Value: "new"},
		&portableUpsertFixture{Key: "new-key", Value: "created"},
	}
	if err := BulkUpsert(tx, rows, 100, "row_key"); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatal(err)
	}

	var stored []portableUpsertFixture
	if err := db.Order("row_key").Find(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if len(stored) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(stored))
	}
	if stored[0].Key != "new-key" || stored[0].Value != "created" {
		t.Fatalf("unexpected inserted row: %#v", stored[0])
	}
	if stored[1].Key != "same-key" || stored[1].Value != "new" {
		t.Fatalf("unexpected updated row: %#v", stored[1])
	}
}

func TestBulkUpsertRejectsMissingConflictColumns(t *testing.T) {
	db, err := gorm.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = BulkUpsert(db, []interface{}{portableUpsertFixture{Key: "key"}}, 100)
	if err == nil {
		t.Fatal("expected missing conflict columns to fail")
	}
}

func TestBulkUpsertKeepsExistingMySQLFastPath(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	db, err := gorm.Open("mysql", sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `portable_upsert_fixture`")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	rows := []interface{}{portableUpsertFixture{Key: "key", Value: "value"}}
	if err := BulkUpsert(db, rows, 100, "row_key"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBulkUpsertTreatsUnchangedRowsAsExisting(t *testing.T) {
	db := openPortableUpsertTestDB(t)
	defer db.Close()

	createdAt := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	fixture := &portableUpsertFixture{CreatedAt: createdAt, Key: "same-key", Value: "unchanged"}
	if err := db.Create(fixture).Error; err != nil {
		t.Fatal(err)
	}

	if err := BulkUpsert(db, []interface{}{portableUpsertFixture{
		Key: "same-key", Value: "unchanged",
	}}, 100, "row_key"); err != nil {
		t.Fatal(err)
	}

	var rows []portableUpsertFixture
	if err := db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected unchanged upsert to keep one row, got %d", len(rows))
	}
	if !rows[0].CreatedAt.Equal(createdAt) {
		t.Fatalf("expected create_time to remain %s, got %s", createdAt, rows[0].CreatedAt)
	}
}

func TestBulkUpsertUpdatesZeroValues(t *testing.T) {
	db := openPortableUpsertTestDB(t)
	defer db.Close()

	if err := db.Create(&portableUpsertFixture{
		Key: "same-key", Value: "old", Enabled: true, Replicas: 3,
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := BulkUpsert(db, []interface{}{portableUpsertFixture{
		Key: "same-key", Value: "", Enabled: false, Replicas: 0,
	}}, 100, "row_key"); err != nil {
		t.Fatal(err)
	}

	var stored portableUpsertFixture
	if err := db.Where("row_key = ?", "same-key").First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Value != "" || stored.Enabled || stored.Replicas != 0 {
		t.Fatalf("expected zero values to be persisted, got %#v", stored)
	}
}

func TestBulkUpsertUsesCallerRollback(t *testing.T) {
	db := openPortableUpsertTestDB(t)
	defer db.Close()

	if err := db.Create(&portableUpsertFixture{Key: "existing", Value: "old"}).Error; err != nil {
		t.Fatal(err)
	}

	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	if err := BulkUpsert(tx, []interface{}{
		portableUpsertFixture{Key: "existing", Value: "new"},
		portableUpsertFixture{Key: "inserted", Value: "new"},
	}, 100, "row_key"); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Rollback().Error; err != nil {
		t.Fatal(err)
	}

	var existing portableUpsertFixture
	if err := db.Where("row_key = ?", "existing").First(&existing).Error; err != nil {
		t.Fatal(err)
	}
	if existing.Value != "old" {
		t.Fatalf("expected rollback to preserve old value, got %q", existing.Value)
	}
	var count int
	if err := db.Model(&portableUpsertFixture{}).Where("row_key = ?", "inserted").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected rollback to discard inserted row, got %d", count)
	}
}

func openPortableUpsertTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&portableUpsertFixture{}).Error; err != nil {
		db.Close()
		t.Fatal(err)
	}
	return db
}

package portable

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

// capability_id: region.database.portable-transaction
func TestWithinTransactionUsesDatabaseTransaction(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	db, err := gorm.Open("mysql", sqlDB)
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectBegin()
	mock.ExpectCommit()
	if err := WithinTransaction(db, func(tx *gorm.DB) error {
		if _, ok := tx.CommonDB().(*sql.Tx); !ok {
			t.Fatal("expected a transaction handle")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// capability_id: region.database.portable-transaction
func TestWithinTransactionUsesSQLiteCompatibilityPath(t *testing.T) {
	db, err := gorm.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := WithinTransaction(db, func(tx *gorm.DB) error {
		if _, ok := tx.CommonDB().(*sql.Tx); ok {
			t.Fatal("SQLite compatibility path must reuse the database handle")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// capability_id: region.database.portable-transaction
func TestWithinTransactionPropagatesCallbackError(t *testing.T) {
	db, err := gorm.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	expected := errors.New("callback failed")
	if err := WithinTransaction(db, func(*gorm.DB) error { return expected }); !errors.Is(err, expected) {
		t.Fatalf("expected callback error, got %v", err)
	}
}
